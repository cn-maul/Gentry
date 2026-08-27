package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/cn-maul/Gentry/fetcher"
	"github.com/cn-maul/Gentry/llm"
)

// 发送给 LLM 的 HTML 预算（字节）。过大既拖慢响应也容易超出上下文窗口；
// 绝大多数页面的主体结构远小于此值。
const maxAIHTMLBytes = 60000

// captureMaxAttempts 是捕获管线的最大提案次数：首次 + 一次错误反馈重试。
const captureMaxAttempts = 2

// AIExtractionResult 是一次 AI 提取的产物：建议的规则配置 + 本地验证样本。
type AIExtractionResult struct {
	Config   ScanMonitorConfig `json:"config"`
	Samples  []ExtractResult   `json:"samples"`
	Verified bool              `json:"verified"`
	Message  string            `json:"message"`
}

// CaptureRequest 是统一捕获管线的输入：目标页面与定位关键词。
type CaptureRequest struct {
	URL      string
	Keywords []string
}

// CaptureDiagnostics 记录捕获过程的观测信息，供前端展示与问题排查。
type CaptureDiagnostics struct {
	Attempts    int      `json:"attempts"`
	Failures    []string `json:"failures,omitempty"`
	KeywordHits int      `json:"keyword_hits"`
	ItemCount   int      `json:"item_count"`
}

// CaptureResult 是捕获管线的产物：一份候选规则草稿 + 样本 + 诊断信息。
// 草稿不会自动入库；用户确认后经 quickCreate 保存为扫描规则。
type CaptureResult struct {
	Config      ScanMonitorConfig   `json:"config"`
	Samples     []ExtractResult     `json:"samples"`
	Verified    bool                `json:"verified"`
	Message     string              `json:"message"`
	Diagnostics *CaptureDiagnostics `json:"diagnostics,omitempty"`
}

// ProposalInput 是提案器一次调用所需的全部上下文；
// Feedback 非空表示上一轮校验失败的原因，提案器应据此修正输出。
type ProposalInput struct {
	PageURL  string
	Keywords []string
	Cleaned  string // 清洗并截断后的页面 HTML
	Feedback string
}

// Proposer 从页面内容产出候选规则配置。实现必须无副作用：
// 一切验证都由管线的 Validator 完成，提案器只负责"猜"。
type Proposer interface {
	Propose(ctx context.Context, input ProposalInput) (ScanMonitorConfig, error)
}

// aiSelectorProposal 是期望 LLM 输出的 JSON 结构。
type aiSelectorProposal struct {
	Container string            `json:"container"`
	Item      string            `json:"item"`
	Fields    []ScanFieldConfig `json:"fields"`
}

const aiSelectorSystemPrompt = `你是网页内容提取专家。用户会提供网页 HTML（可能被截断）和一组关键词，关键词是目标列表条目应包含的文字，用于定位正确的内容区域。请分析 HTML 结构，为该内容列表编写 CSS 选择器提取规则。

输出要求：
1. 只输出一个 JSON 对象，不要 markdown 代码块，不要任何解释文字。
2. JSON 格式：
{"container":"<容器选择器>","item":"<条目选择器>","fields":[{"name":"title","selector":"<选择器>","type":"text","attr":"","transform":""}]}
3. container 是包含整个目标列表的元素，要足够具体以避开导航栏、页脚、侧边栏等干扰区域。
4. item 是 container 内重复出现的列表条目。
5. fields 必须包含 name 为 "title" 的字段；若条目含链接，添加 name 为 "url"、type 为 "attr"、attr 为 "href" 的字段；有日期时可添加 name 为 "date" 的字段。
6. type 只能是 "text" 或 "attr"。选择器必须是合法 CSS 选择器（不要 XPath）。字段 selector 相对条目元素；留空表示取条目自身文本。`

// RunCapture 抓取页面并运行捕获管线：LLM 提案 → 本地验证 → 错误反馈重试。
// 返回的草稿仅供人工确认，不会写入规则库或监控。
func RunCapture(ctx context.Context, req CaptureRequest) (*CaptureResult, error) {
	cfg, err := llm.LoadConfig()
	if err != nil {
		return nil, err
	}
	client := fetcher.New()
	html, err := client.FetchContext(ctx, req.URL)
	if err != nil {
		return nil, fmt.Errorf("抓取页面失败: %w", err)
	}
	return runCapture(ctx, html, req, llmProposer{cfg: cfg})
}

// runCapture 是管线核心，页面 HTML 与提案器均可注入，便于测试。
func runCapture(ctx context.Context, html string, req CaptureRequest, proposer Proposer) (*CaptureResult, error) {
	cleaned := prepareHTMLForAI(html, req.Keywords)

	diag := &CaptureDiagnostics{}
	feedback := ""
	for attempt := 1; attempt <= captureMaxAttempts; attempt++ {
		diag.Attempts = attempt
		config, perr := proposer.Propose(ctx, ProposalInput{
			PageURL:  req.URL,
			Keywords: req.Keywords,
			Cleaned:  cleaned,
			Feedback: feedback,
		})
		if perr != nil {
			// 鉴权等致命错误重试无意义，立即中止
			if llm.IsFatal(perr) {
				return nil, perr
			}
			feedback = perr.Error()
			diag.Failures = append(diag.Failures, feedback)
			continue
		}

		samples, keywordHits, verr := ValidateDraftOnPage(html, req.URL, req.Keywords, config)
		if verr != nil {
			feedback = verr.Error()
			diag.Failures = append(diag.Failures, feedback)
			continue
		}
		diag.KeywordHits = keywordHits
		diag.ItemCount = len(samples)

		if keywordHits > 0 || len(req.Keywords) == 0 {
			return &CaptureResult{
				Config:      config,
				Samples:     samples,
				Verified:    true,
				Message:     fmt.Sprintf("已验证：提取到 %d 条内容，其中 %d 条命中关键词", len(samples), keywordHits),
				Diagnostics: diag,
			}, nil
		}
		// 草稿可用但关键词未命中：返回建议让用户对照样本判断，不再消耗重试
		return &CaptureResult{
			Config:      config,
			Samples:     samples,
			Verified:    false,
			Message:     fmt.Sprintf("提取到 %d 条内容，但没有条目命中关键词，请对照样本确认区域是否正确", len(samples)),
			Diagnostics: diag,
		}, nil
	}
	return nil, fmt.Errorf("未能生成可用的选择器: %s", feedback)
}

// llmProposer 基于 LLM 的提案器：把清洗后的页面与反馈交给模型，解析出规则草稿。
type llmProposer struct {
	cfg llm.Config
}

func (p llmProposer) Propose(ctx context.Context, input ProposalInput) (ScanMonitorConfig, error) {
	keywordLine := "（未提供关键词，请自行判断页面中最可能被监控的主内容列表）"
	if len(input.Keywords) > 0 {
		keywordLine = strings.Join(input.Keywords, "、")
	}
	userPrompt := "目标关键词：" + keywordLine + "\n\n网页 HTML：\n" + input.Cleaned
	if input.Feedback != "" {
		userPrompt = "上一次的选择器未通过验证，错误信息：" + input.Feedback +
			"\n请修正后重新输出完整 JSON。\n\n目标关键词：" + keywordLine + "\n\n网页 HTML：\n" + input.Cleaned
	}
	answer, err := llm.Chat(ctx, p.cfg, aiSelectorSystemPrompt, userPrompt)
	if err != nil {
		return ScanMonitorConfig{}, err
	}
	proposal, err := parseAISelectorJSON(answer)
	if err != nil {
		return ScanMonitorConfig{}, err
	}
	return proposalToConfig(proposal), nil
}

// prepareHTMLForAI 去掉脚本、样式等对结构分析无意义的部分，
// 并在字节预算内截断：优先保留关键词首次密集出现位置附近的内容，
// 截断点回退到 UTF-8 字符边界，避免切断多字节汉字。
func prepareHTMLForAI(html string, keywords []string) string {
	cleaned := html
	if doc, err := goquery.NewDocumentFromReader(strings.NewReader(html)); err == nil {
		doc.Find("script, style, noscript, template, svg, iframe, link, meta").Remove()
		if out, err := doc.Html(); err == nil {
			cleaned = out
		}
	}
	if len(cleaned) <= maxAIHTMLBytes {
		return cleaned
	}

	start := keywordWindowStart(cleaned, keywords)
	end := start + maxAIHTMLBytes
	if end > len(cleaned) {
		end = len(cleaned)
	} else {
		// 回退到字符边界，避免把多字节字符切成非法 UTF-8
		for end > start && !isRuneBoundary(cleaned, end) {
			end--
		}
	}
	window := cleaned[start:end]
	if start > 0 {
		window = "<!-- 页面前半部分已省略 -->" + window
	}
	return window
}

// keywordWindowStart 决定截断窗口起点：若关键词首次命中位置过深
// （超过预算的 2/5），从命中位置前留出 1/4 预算处开始截取，
// 保证目标列表进入模型视野；否则从头开始。
func keywordWindowStart(content string, keywords []string) int {
	const (
		deepThreshold = maxAIHTMLBytes * 2 / 5
		headMargin    = maxAIHTMLBytes / 4
	)
	lower := strings.ToLower(content)
	for _, kw := range keywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw == "" {
			continue
		}
		if idx := strings.Index(lower, kw); idx >= deepThreshold {
			start := idx - headMargin
			if start < 0 {
				start = 0
			}
			for start > 0 && !isRuneBoundary(content, start) {
				start--
			}
			return start
		}
	}
	return 0
}

func isRuneBoundary(s string, i int) bool {
	if i <= 0 || i >= len(s) {
		return true
	}
	return s[i]&0xC0 != 0x80
}

// parseAISelectorJSON 从模型输出中提取选择器 JSON：
// 平衡括号扫描容忍代码围栏、前后杂文和杂文中孤立的花括号。
func parseAISelectorJSON(answer string) (aiSelectorProposal, error) {
	raw, err := llm.ExtractJSONObject(answer)
	if err != nil {
		return aiSelectorProposal{}, err
	}
	var proposal aiSelectorProposal
	if err := json.Unmarshal([]byte(raw), &proposal); err != nil {
		return aiSelectorProposal{}, fmt.Errorf("解析 JSON 失败: %w", err)
	}
	if strings.TrimSpace(proposal.Container) == "" || strings.TrimSpace(proposal.Item) == "" {
		return aiSelectorProposal{}, fmt.Errorf("缺少容器或条目选择器")
	}
	return proposal, nil
}

func proposalToConfig(proposal aiSelectorProposal) ScanMonitorConfig {
	fields := make([]ScanFieldConfig, 0, len(proposal.Fields))
	for _, field := range proposal.Fields {
		fieldType := field.Type
		if fieldType != "text" && fieldType != "attr" {
			fieldType = "text"
		}
		fields = append(fields, ScanFieldConfig{
			Name:      strings.TrimSpace(field.Name),
			Selector:  field.Selector,
			Type:      fieldType,
			Attr:      field.Attr,
			Transform: field.Transform,
		})
	}
	return ScanMonitorConfig{
		Container: strings.TrimSpace(proposal.Container),
		Item:      strings.TrimSpace(proposal.Item),
		Fields:    fields,
	}
}

// ValidateDraftOnPage 在真实 HTML 上验证候选规则草稿：选择器可编译、必含
// title、能提取条目、链接可解析，并统计关键词命中的条目数。
// 捕获管线、AI 提取与草稿直测共用这一实现。
func ValidateDraftOnPage(html, pageURL string, keywords []string, config ScanMonitorConfig) ([]ExtractResult, int, error) {
	if _, err := cascadia.Compile(config.Container); err != nil {
		return nil, 0, fmt.Errorf("容器选择器无效: %w", err)
	}
	if _, err := cascadia.Compile(config.Item); err != nil {
		return nil, 0, fmt.Errorf("条目选择器无效: %w", err)
	}
	for _, field := range config.Fields {
		if strings.TrimSpace(field.Selector) != "" {
			if _, err := cascadia.Compile(field.Selector); err != nil {
				return nil, 0, fmt.Errorf("字段 %s 的选择器无效: %w", field.Name, err)
			}
		}
	}
	hasTitle := false
	for _, field := range config.Fields {
		if field.Name == "title" {
			hasTitle = true
			break
		}
	}
	if !hasTitle {
		return nil, 0, fmt.Errorf("字段中缺少 title")
	}

	items, err := NewExtractor(ScanConfigToSelectors(config)).Extract(html)
	if err != nil {
		return nil, 0, fmt.Errorf("提取失败: %w", err)
	}
	if len(items) == 0 {
		return nil, 0, fmt.Errorf("选择器未提取到任何条目")
	}
	if err := ResolveExtractedURLs(pageURL, items); err != nil {
		return nil, 0, fmt.Errorf("解析条目链接失败: %w", err)
	}

	keywordHits := 0
	if len(keywords) > 0 {
		for _, item := range items {
			if matchKeywords(item, keywords) {
				keywordHits++
			}
		}
	}
	if len(items) > 10 {
		items = items[:10]
	}
	return items, keywordHits, nil
}

// verifyAIProposal 是 ValidateDraftOnPage 的历史别名，保持既有调用兼容。
func verifyAIProposal(html, pageURL string, keywords []string, config ScanMonitorConfig) ([]ExtractResult, int, error) {
	return ValidateDraftOnPage(html, pageURL, keywords, config)
}

// AIExtractRule 抓取页面，让 LLM 生成选择器，并在本地验证：
// 选择器可编译、能提取条目、条目内容命中关键词。验证失败会携带错误反馈
// 自动重试一次；仍无法完全验证时返回可用的建议供用户人工确认。
// 它是统一捕获管线在 LLM 提案器上的便捷入口。
func AIExtractRule(ctx context.Context, pageURL string, keywords []string) (*AIExtractionResult, error) {
	result, err := RunCapture(ctx, CaptureRequest{URL: pageURL, Keywords: keywords})
	if err != nil {
		return nil, err
	}
	return &AIExtractionResult{
		Config:   result.Config,
		Samples:  result.Samples,
		Verified: result.Verified,
		Message:  result.Message,
	}, nil
}
