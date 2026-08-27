package monitor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cn-maul/Gentry/llm"
)

// stubProposer 按预设脚本依次返回草稿或错误，并记录收到的反馈。
type stubProposer struct {
	configs []ScanMonitorConfig
	errs    []error
	inputs  []ProposalInput
}

func (s *stubProposer) Propose(_ context.Context, input ProposalInput) (ScanMonitorConfig, error) {
	s.inputs = append(s.inputs, input)
	i := len(s.inputs) - 1
	if i < len(s.errs) && s.errs[i] != nil {
		return ScanMonitorConfig{}, s.errs[i]
	}
	if i < len(s.configs) {
		return s.configs[i], nil
	}
	return ScanMonitorConfig{}, errors.New("stub exhausted")
}

var captureTestHTML = `<html><head><style>.x{}</style></head><body>
<nav><a href="/home">首页</a></nav>
<ul class="news-list">
<li><a href="/1.html">2026 面试公告发布</a><span>2026-07-07</span></li>
<li><a href="/2.html">录用公示名单</a><span>2026-07-06</span></li>
</ul>
<footer><a href="/c">联系</a></footer>
</body></html>`

func goodDraft() ScanMonitorConfig {
	return ScanMonitorConfig{
		Container: "ul.news-list",
		Item:      "li",
		Fields: []ScanFieldConfig{
			{Name: "title", Selector: "a", Type: "text"},
			{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
		},
	}
}

func TestRunCaptureVerifiedHappyPath(t *testing.T) {
	p := &stubProposer{configs: []ScanMonitorConfig{goodDraft()}}
	result, err := runCapture(context.Background(), captureTestHTML, CaptureRequest{URL: "https://x.example/list", Keywords: []string{"公告"}}, p)
	if err != nil {
		t.Fatalf("runCapture: %v", err)
	}
	if !result.Verified || result.Diagnostics.KeywordHits != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Samples) != 2 {
		t.Fatalf("expected 2 samples: %+v", result.Samples)
	}
	if result.Config.Container != "ul.news-list" {
		t.Fatalf("unexpected config: %+v", result.Config)
	}
	if len(p.inputs) != 1 || p.inputs[0].Feedback != "" {
		t.Fatalf("single attempt without feedback expected: %+v", p.inputs)
	}
}

func TestRunCaptureFeedbackRetry(t *testing.T) {
	p := &stubProposer{
		errs: []error{nil}, // 第一轮由校验失败驱动
		configs: []ScanMonitorConfig{
			{Container: "ul<<<", Item: "li", Fields: []ScanFieldConfig{{Name: "title", Type: "text"}}},
			goodDraft(),
		},
	}
	result, err := runCapture(context.Background(), captureTestHTML, CaptureRequest{URL: "https://x.example/list", Keywords: []string{"公告"}}, p)
	if err != nil {
		t.Fatalf("runCapture: %v", err)
	}
	if !result.Verified {
		t.Fatalf("expected verified after retry: %+v", result)
	}
	if len(p.inputs) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(p.inputs))
	}
	if p.inputs[1].Feedback == "" || !strings.Contains(p.inputs[1].Feedback, "容器选择器无效") {
		t.Fatalf("feedback should carry validation error: %q", p.inputs[1].Feedback)
	}
	if len(result.Diagnostics.Failures) != 1 {
		t.Fatalf("diagnostics should record first failure: %+v", result.Diagnostics)
	}
}

func TestRunCaptureKeywordMissStopsWithoutRetry(t *testing.T) {
	draft := ScanMonitorConfig{
		Container: "nav",
		Item:      "a",
		Fields:    []ScanFieldConfig{{Name: "title", Type: "text"}},
	}
	p := &stubProposer{configs: []ScanMonitorConfig{draft}}
	result, err := runCapture(context.Background(), captureTestHTML, CaptureRequest{URL: "https://x.example/", Keywords: []string{"公告"}}, p)
	if err != nil {
		t.Fatalf("runCapture: %v", err)
	}
	if result.Verified {
		t.Fatal("expected unverified on keyword miss")
	}
	if len(p.inputs) != 1 {
		t.Fatalf("keyword miss must not consume retry, attempts=%d", len(p.inputs))
	}
}

func TestRunCaptureFatalErrorAbortsImmediately(t *testing.T) {
	p := &stubProposer{errs: []error{&llm.ProviderError{Kind: llm.KindAuth, Status: 401}}}
	_, err := runCapture(context.Background(), captureTestHTML, CaptureRequest{URL: "https://x.example/", Keywords: []string{"公告"}}, p)
	if err == nil {
		t.Fatal("expected fatal error to propagate")
	}
	if !llm.IsFatal(err) {
		t.Fatalf("expected fatal auth error: %v", err)
	}
	if len(p.inputs) != 1 {
		t.Fatalf("fatal error must abort without second attempt, got %d", len(p.inputs))
	}
}

func TestRunCaptureExhaustedAttemptsReturnsFeedbackError(t *testing.T) {
	bad := ScanMonitorConfig{Container: "ul<<<", Item: "li", Fields: []ScanFieldConfig{{Name: "title", Type: "text"}}}
	p := &stubProposer{configs: []ScanMonitorConfig{bad, bad}}
	_, err := runCapture(context.Background(), captureTestHTML, CaptureRequest{URL: "https://x.example/"}, p)
	if err == nil {
		t.Fatal("expected error after exhausted attempts")
	}
	if !strings.Contains(err.Error(), "未能生成可用的选择器") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===== prepareHTMLForAI：rune 安全截断与关键词窗口 =====

func TestPrepareHTMLTruncatesOnRuneBoundary(t *testing.T) {
	// 构造一个截断点落在多字节字符中间的页面
	repeat := strings.Repeat("公", 20000) // 每字 3 字节，共 60KB+
	html := "<html><body>" + repeat + "</body></html>"
	out := prepareHTMLForAI(html, nil)
	if len(out) > maxAIHTMLBytes+64 { // 允许省略注释的开销
		t.Fatalf("output exceeds budget: %d bytes", len(out))
	}
	// 输出必须是合法 UTF-8：末尾不能出现被切断的字符
	if strings.HasSuffix(out, string([]byte{0xE5})) || strings.HasSuffix(out, string([]byte{0xE5, 0x85})) {
		t.Fatal("truncation cut a multi-byte rune")
	}
}

func TestPrepareHTMLKeepsKeywordWindow(t *testing.T) {
	filler := strings.Repeat("<div class=\"pad\">填充内容</div>", 3000) // 远超 60KB
	list := `<ul class="news"><li>招聘公告第一条</li><li>招聘公告第二条</li></ul>`
	html := "<html><body>" + filler + list + "</body></html>"
	out := prepareHTMLForAI(html, []string{"公告"})
	if strings.Contains(out[:min(len(out), maxAIHTMLBytes)], "news") == false && !strings.Contains(out, "news") {
		t.Fatal("keyword window should keep the target list in view")
	}
	if !strings.Contains(out, "公告") {
		t.Fatal("window should contain keyword context")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ===== parseAISelectorJSON：平衡括号解析 =====

func TestParseAISelectorJSONToleratesBracesInProse(t *testing.T) {
	answer := `分析如下 {注意} ：{"container":"ul.news","item":"li","fields":[{"name":"title","type":"text"}]} 以上`
	proposal, err := parseAISelectorJSON(answer)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if proposal.Container != "ul.news" || proposal.Item != "li" {
		t.Fatalf("unexpected proposal: %+v", proposal)
	}
}
