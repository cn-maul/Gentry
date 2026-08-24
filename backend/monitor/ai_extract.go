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

// 发送给 LLM 的 HTML 上限。过大既拖慢响应也容易超出上下文窗口；
// 绝大多数页面的主体结构远小于此值。
const maxAIHTMLBytes = 60000

// AIExtractionResult 是一次 AI 提取的产物：建议的规则配置 + 本地验证样本。
type AIExtractionResult struct {
	Config   ScanMonitorConfig `json:"config"`
	Samples  []ExtractResult   `json:"samples"`
	Verified bool              `json:"verified"`
	Message  string            `json:"message"`
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

// AIExtractRule 抓取页面，让 LLM 生成选择器，并在本地验证：
// 选择器可编译、能提取条目、条目内容命中关键词。验证失败会携带错误反馈
// 自动重试一次；仍无法完全验证时返回可用的建议供用户人工确认。
func AIExtractRule(ctx context.Context, pageURL string, keywords []string) (*AIExtractionResult, error) {
	cfg, err := llm.LoadConfig()
	if err != nil {
		return nil, err
	}
	client := fetcher.New()
	html, err := client.FetchContext(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("抓取页面失败: %w", err)
	}
	pageHTML := prepareHTMLForAI(html)

	keywordLine := "（未提供关键词，请自行判断页面中最可能被监控的主内容列表）"
	if len(keywords) > 0 {
		keywordLine = strings.Join(keywords, "、")
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		userPrompt := "目标关键词：" + keywordLine + "\n\n网页 HTML：\n" + pageHTML
		if lastErr != nil {
			userPrompt = "上一次的选择器未通过验证，错误信息：" + lastErr.Error() +
				"\n请修正后重新输出完整 JSON。\n\n目标关键词：" + keywordLine + "\n\n网页 HTML：\n" + pageHTML
		}
		answer, err := llm.Chat(ctx, cfg, aiSelectorSystemPrompt, userPrompt)
		if err != nil {
			return nil, err
		}
		proposal, err := parseAISelectorJSON(answer)
		if err != nil {
			lastErr = fmt.Errorf("AI 输出不是有效的选择器 JSON")
			continue
		}
		config := proposalToConfig(proposal)
		samples, keywordHits, err := verifyAIProposal(html, pageURL, keywords, config)
		if err != nil {
			lastErr = err
			continue
		}
		if keywordHits > 0 || len(keywords) == 0 {
			return &AIExtractionResult{
				Config:   config,
				Samples:  samples,
				Verified: true,
				Message:  fmt.Sprintf("已验证：提取到 %d 条内容，其中 %d 条命中关键词", len(samples), keywordHits),
			}, nil
		}
		// 选择器可用但关键词未命中：返回建议让用户对照样本判断
		return &AIExtractionResult{
			Config:   config,
			Samples:  samples,
			Verified: false,
			Message:  fmt.Sprintf("提取到 %d 条内容，但没有条目命中关键词，请对照样本确认区域是否正确", len(samples)),
		}, nil
	}
	return nil, fmt.Errorf("AI 未能生成可用的选择器: %v", lastErr)
}

// prepareHTMLForAI 去掉脚本、样式等对结构分析无意义的部分并截断。
func prepareHTMLForAI(html string) string {
	cleaned := html
	if doc, err := goquery.NewDocumentFromReader(strings.NewReader(html)); err == nil {
		doc.Find("script, style, noscript, template, svg, iframe, link, meta").Remove()
		if out, err := doc.Html(); err == nil {
			cleaned = out
		}
	}
	if len(cleaned) > maxAIHTMLBytes {
		cleaned = cleaned[:maxAIHTMLBytes]
	}
	return cleaned
}

// parseAISelectorJSON 从模型输出中提取选择器 JSON，容忍 markdown 代码块和前后杂文。
func parseAISelectorJSON(answer string) (aiSelectorProposal, error) {
	content := strings.TrimSpace(answer)
	content = strings.ReplaceAll(content, "```json", "```")
	if idx := strings.Index(content, "```"); idx >= 0 {
		if end := strings.Index(content[idx+3:], "```"); end >= 0 {
			content = content[idx+3 : idx+3+end]
		}
	}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return aiSelectorProposal{}, fmt.Errorf("输出中未找到 JSON 对象")
	}
	var proposal aiSelectorProposal
	if err := json.Unmarshal([]byte(content[start:end+1]), &proposal); err != nil {
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

// verifyAIProposal 在真实 HTML 上运行建议的选择器：编译、提取、解析链接并统计关键词命中。
func verifyAIProposal(html, pageURL string, keywords []string, config ScanMonitorConfig) ([]ExtractResult, int, error) {
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
