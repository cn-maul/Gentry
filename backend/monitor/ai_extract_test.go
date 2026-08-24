package monitor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cn-maul/Gentry/llm"
)

const aiTestPageHTML = `<html><head><title>公告</title><style>.x{}</style><script>var a=1;</script></head>
<body>
<nav><a href="/home">首页</a><a href="/about">关于</a></nav>
<ul class="news-list">
<li><a href="/1.html">2026 面试公告发布</a><span>2026-07-07</span></li>
<li><a href="/2.html">录用公示名单</a><span>2026-07-06</span></li>
</ul>
<footer><a href="/contact">联系我们</a></footer>
</body></html>`

func startFakeLLM(t *testing.T, replies []string) *httptest.Server {
	t.Helper()
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if index >= len(replies) {
			t.Errorf("unexpected extra LLM call #%d", index)
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
			return
		}
		reply := replies[index]
		index++
		_, _ = w.Write([]byte(fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":%q}}]}`, reply)))
	}))
	t.Cleanup(server.Close)
	return server
}

func configureFakeLLM(t *testing.T, server *httptest.Server, model string) {
	t.Helper()
	if err := llm.SaveConfig(llm.Config{BaseURL: server.URL, APIKey: "", Model: model}); err != nil {
		t.Fatalf("save llm config: %v", err)
	}
}

func TestParseAISelectorJSONToleratesFences(t *testing.T) {
	proposal, err := parseAISelectorJSON("```json\n{\"container\":\"ul.list\",\"item\":\"li\",\"fields\":[{\"name\":\"title\",\"type\":\"text\"}]}\n```\n补充说明文字")
	if err != nil {
		t.Fatalf("parse fenced json: %v", err)
	}
	if proposal.Container != "ul.list" || proposal.Item != "li" || len(proposal.Fields) != 1 {
		t.Fatalf("unexpected proposal: %+v", proposal)
	}

	if _, err := parseAISelectorJSON("模型拒绝分析该页面"); err == nil {
		t.Fatal("expected error for non-json answer")
	}
	if _, err := parseAISelectorJSON(`{"container":"","item":"li"}`); err == nil {
		t.Fatal("expected error for missing container")
	}
}

func TestAIExtractRuleVerifiedProposal(t *testing.T) {
	setupMonitorTestDB(t)
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(aiTestPageHTML))
	}))
	defer page.Close()

	reply := `{"container":"ul.news-list","item":"li","fields":[
		{"name":"title","selector":"a","type":"text"},
		{"name":"url","selector":"a","type":"attr","attr":"href"},
		{"name":"date","selector":"span","type":"text"}]}`
	fake := startFakeLLM(t, []string{reply})
	configureFakeLLM(t, fake, "fake-model")

	result, err := AIExtractRule(context.Background(), page.URL, []string{"公告"})
	if err != nil {
		t.Fatalf("AIExtractRule: %v", err)
	}
	if !result.Verified {
		t.Fatalf("expected verified result, message: %s", result.Message)
	}
	if result.Config.Container != "ul.news-list" || result.Config.Item != "li" {
		t.Fatalf("unexpected config: %+v", result.Config)
	}
	if len(result.Samples) != 2 {
		t.Fatalf("expected 2 samples, got %+v", result.Samples)
	}
	if title, _ := result.Samples[0]["title"].(string); !strings.Contains(title, "面试公告") {
		t.Fatalf("unexpected first sample: %+v", result.Samples[0])
	}
	if url, _ := result.Samples[0]["url"].(string); !strings.HasPrefix(url, page.URL) {
		t.Fatalf("sample url not resolved to absolute: %+v", result.Samples[0])
	}
	if date, _ := result.Samples[0]["date"].(string); date != "2026-07-07" {
		t.Fatalf("unexpected sample date: %v", result.Samples[0]["date"])
	}
}

func TestAIExtractRuleRetriesOnInvalidSelector(t *testing.T) {
	setupMonitorTestDB(t)
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(aiTestPageHTML))
	}))
	defer page.Close()

	// 第一次返回无法编译的选择器，验证失败后带错误反馈重试
	bad := `{"container":"ul<<<","item":"li","fields":[{"name":"title","type":"text"}]}`
	good := `{"container":"ul.news-list","item":"li","fields":[{"name":"title","selector":"a","type":"text"}]}`
	fake := startFakeLLM(t, []string{bad, good})
	configureFakeLLM(t, fake, "fake-model")

	result, err := AIExtractRule(context.Background(), page.URL, []string{"公告"})
	if err != nil {
		t.Fatalf("AIExtractRule with retry: %v", err)
	}
	if !result.Verified || result.Config.Container != "ul.news-list" {
		t.Fatalf("expected retry to succeed, got: %+v", result)
	}
}

func TestAIExtractRuleKeywordMissReturnsUnverified(t *testing.T) {
	setupMonitorTestDB(t)
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(aiTestPageHTML))
	}))
	defer page.Close()

	// 选择器落在导航栏上：可提取但没有条目命中关键词
	reply := `{"container":"nav","item":"a","fields":[{"name":"title","type":"text"}]}`
	fake := startFakeLLM(t, []string{reply, reply})
	configureFakeLLM(t, fake, "fake-model")

	result, err := AIExtractRule(context.Background(), page.URL, []string{"公告"})
	if err != nil {
		t.Fatalf("AIExtractRule keyword miss: %v", err)
	}
	if result.Verified {
		t.Fatalf("expected unverified result, got: %+v", result)
	}
	if len(result.Samples) == 0 || !strings.Contains(result.Message, "关键词") {
		t.Fatalf("expected samples with warning, got: %+v", result)
	}
}
