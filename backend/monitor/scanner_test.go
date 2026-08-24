package monitor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cn-maul/Gentry/database"
)

func setupScanRuleDB(t *testing.T) {
	t.Helper()
	originalDB := database.DB
	if err := database.Init(filepath.Join(t.TempDir(), "scan-rules.db")); err != nil {
		t.Fatalf("init scan rule database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		database.DB = originalDB
	})
}

func TestExtractorExtractsGenericListItems(t *testing.T) {
	html := `<ul class="news-list"><li><a href="/a.html">第一条公告</a><time>2026-07-07</time></li><li><a href="/b.html">第二条公告</a><time>2026-07-06</time></li></ul>`
	ex := NewExtractor(SiteSelectors{
		Container: ".news-list",
		Item:      "li",
		Fields: []FieldConfig{
			{Name: "title", Selector: "a", Type: "text"},
			{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
			{Name: "date", Selector: "time", Type: "text"},
		},
	})
	items, err := ex.Extract(html)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0]["title"] != "第一条公告" {
		t.Fatalf("unexpected title: %v", items[0]["title"])
	}
	if items[0]["url"] != "/a.html" {
		t.Fatalf("unexpected url: %v", items[0]["url"])
	}
	if items[0]["date"] != "2026-07-07" {
		t.Fatalf("unexpected date: %v", items[0]["date"])
	}
}

func TestExtractorNarrowsLegacyBareListSelectorAwayFromNavigation(t *testing.T) {
	html := `<body>
		<div class="sideMenu"><ul class="submenu">
			<li><a href="/zfxxgk/fdzdgknr/zfgb/">政府公报</a></li>
			<li><a href="/zfxxgk/fdzdgknr/cfqz/">处罚强制</a></li>
			<li><a href="/jczwgk/gkly/cszhzf/xzqzqzqd/">行政许可和对外管理服务</a></li>
		</ul></div>
		<div class="zfxxgk_zdgkc"><ul>
			<li><a href="/2026/07-23/3653721.html">2026年安阳市殷都区委组织部所属事业单位公开选调工作人员公告</a><span>2026-07-23</span></li>
			<li><a href="/2026/07-17/3652898.html">2026年事业单位公开招聘面试资格确认递补公告</a><span>2026-07-17</span></li>
			<li><a href="/2026/07-01/3650588.html">2026年事业单位公开招聘面试资格确认公告</a><span>2026-07-01</span></li>
		</ul></div>
	</body>`
	ex := NewExtractor(SiteSelectors{
		Container: "ul",
		Item:      "li",
		Fields: []FieldConfig{
			{Name: "title", Selector: "a", Type: "text"},
			{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
		},
	})
	items, err := ex.Extract(html)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected only 3 announcement items, got %d: %+v", len(items), items)
	}
	if items[0]["title"] != "2026年安阳市殷都区委组织部所属事业单位公开选调工作人员公告" {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	for _, item := range items {
		if item["title"] == "政府公报" || item["title"] == "处罚强制" || item["title"] == "行政许可和对外管理服务" {
			t.Fatalf("navigation item must not be extracted: %+v", item)
		}
	}
}

func TestExtractorRecoversLegacyMetadataItemSelector(t *testing.T) {
	html := `<body><ul class="page-list">
		<a class="list_item_pubinfo_a" href="/D49Y/10166257.jhtml"><span>第一条公开招聘公告</span></a>
		<ul class="pubinfo-card"><li>索引号 001</li><li>2026-07-21</li></ul>
		<a class="list_item_pubinfo_a" href="/D49Y/10135265.jhtml"><span>第二条公开招聘公告</span></a>
		<ul class="pubinfo-card"><li>索引号 002</li><li>2026-07-02</li></ul>
	</ul></body>`
	ex := NewExtractor(SiteSelectors{
		Container: "ul.page-list",
		Item:      "ul.pubinfo-card",
		Fields:    []FieldConfig{{Name: "title", Type: "text"}},
	})
	items, err := ex.Extract(html)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 recovered articles, got %d: %+v", len(items), items)
	}
	if items[0]["title"] != "第一条公开招聘公告" || items[0]["url"] != "/D49Y/10166257.jhtml" {
		t.Fatalf("unexpected recovered article: %+v", items[0])
	}
}

func TestExtractorTitleFallsBackToItemText(t *testing.T) {
	html := `<ul class="news-list"><li>纯文本公告标题</li></ul>`
	ex := NewExtractor(SiteSelectors{
		Container: ".news-list",
		Item:      "li",
		Fields:    []FieldConfig{{Name: "title", Selector: ".missing", Type: "text"}},
	})
	items, err := ex.Extract(html)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0]["title"] != "纯文本公告标题" {
		t.Fatalf("unexpected title fallback: %v", items[0]["title"])
	}
}

func TestApplyTransformRegexp(t *testing.T) {
	got := applyTransform("2026/07/07 公告", `regexp("/","-")`)
	if got != "2026-07-07 公告" {
		t.Fatalf("unexpected regexp transform: %q", got)
	}
}

func TestResolveExtractedURLs(t *testing.T) {
	items := []ExtractResult{
		{"title": "root", "url": "/notice/1"},
		{"title": "relative", "url": "detail/2"},
		{"title": "absolute", "url": "https://other.example/3"},
	}
	if err := ResolveExtractedURLs("https://example.com/news/list", items); err != nil {
		t.Fatalf("ResolveExtractedURLs failed: %v", err)
	}
	if items[0]["url"] != "https://example.com/notice/1" {
		t.Fatalf("unexpected root-relative URL: %v", items[0]["url"])
	}
	if items[1]["url"] != "https://example.com/news/detail/2" {
		t.Fatalf("unexpected relative URL: %v", items[1]["url"])
	}
	if items[2]["url"] != "https://other.example/3" {
		t.Fatalf("absolute URL should remain unchanged: %v", items[2]["url"])
	}
}

func TestScanHTMLWithRulesOnlyUsesSavedRules(t *testing.T) {
	setupScanRuleDB(t)
	html := `<body><main><ul class="news-list"><li><a href="/1">2026 面试公告发布</a></li><li><a href="/2">2026 录用公示</a></li><li><a href="/3">体检通知</a></li></ul></main></body>`

	// 未保存规则时返回空候选，不再做启发式发现
	res, err := scanHTMLWithRules(html, "https://example.com/news")
	if err != nil {
		t.Fatalf("scanHTMLWithRules failed: %v", err)
	}
	if len(res.Containers) != 0 {
		t.Fatalf("expected no candidates without saved rules, got %+v", res.Containers)
	}

	// 保存规则后仅返回规则候选
	rule := &database.ScanRuleTemplate{
		Name:      "gov-news",
		Container: ".news-list",
		Item:      "li",
		Priority:  80,
		Enabled:   true,
	}
	if err := ApplyScanRuleScope(rule, "https://example.com/news", ScanRuleScopeExact); err != nil {
		t.Fatalf("apply rule scope: %v", err)
	}
	if err := database.CreateScanRuleTemplate(rule); err != nil {
		t.Fatalf("create scan rule: %v", err)
	}

	res2, err := scanHTMLWithRules(html, "https://example.com/news")
	if err != nil {
		t.Fatalf("scanHTMLWithRules with rule failed: %v", err)
	}
	if len(res2.Containers) != 1 {
		t.Fatalf("expected exactly the saved-rule candidate, got %+v", res2.Containers)
	}
	candidate := res2.Containers[0]
	if candidate.Strategy != "template_gov-news" {
		t.Fatalf("strategy = %q, want template_gov-news", candidate.Strategy)
	}
	if len(candidate.SampleItems) != 3 {
		t.Fatalf("expected 3 sample items, got %+v", candidate.SampleItems)
	}
	if title, _ := candidate.SampleItems[0]["title"].(string); !strings.Contains(title, "面试公告发布") {
		t.Fatalf("unexpected sample title: %+v", candidate.SampleItems[0])
	}
}

func TestScanHTMLWithRulesAppliesDefaultFieldsAndURLResolution(t *testing.T) {
	setupScanRuleDB(t)
	html := `<body><ul class="news-list"><li><a href="/1">第一条公告</a></li><li><a href="/2">第二条公告</a></li></ul></body>`
	rule := &database.ScanRuleTemplate{
		Name:      "default-fields",
		Container: ".news-list",
		Item:      "li",
		Priority:  50,
		Enabled:   true,
	}
	if err := ApplyScanRuleScope(rule, "https://example.com/news", ScanRuleScopeExact); err != nil {
		t.Fatalf("apply rule scope: %v", err)
	}
	if err := database.CreateScanRuleTemplate(rule); err != nil {
		t.Fatalf("create scan rule: %v", err)
	}

	res, err := scanHTMLWithRules(html, "https://example.com/news")
	if err != nil {
		t.Fatalf("scanHTMLWithRules failed: %v", err)
	}
	if len(res.Containers) != 1 {
		t.Fatalf("expected one candidate, got %+v", res.Containers)
	}
	candidate := res.Containers[0]
	// 无自定义字段的规则应使用默认 title/url 字段，且相对链接已解析为绝对链接
	if url, _ := candidate.SampleItems[0]["url"].(string); url != "https://example.com/1" {
		t.Fatalf("unexpected resolved url: %+v", candidate.SampleItems[0])
	}
	items, err := NewExtractor(ScanConfigToSelectors(candidate.Config)).Extract(html)
	if err != nil {
		t.Fatalf("extract candidate config: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items from rule config, got %d: %+v", len(items), items)
	}
}

func TestScanHTMLWithRulesOrdersCandidatesByPriority(t *testing.T) {
	setupScanRuleDB(t)
	html := `<body><ul class="news-list"><li><a href="/1">公告</a></li><li><a href="/2">公告</a></li></ul><ol class="other-list"><li><a href="/3">公告</a></li><li><a href="/4">公告</a></li></ol></body>`
	for _, rule := range []*database.ScanRuleTemplate{
		{Name: "low-priority", Container: ".other-list", Item: "li", Priority: 10, Enabled: true},
		{Name: "high-priority", Container: ".news-list", Item: "li", Priority: 90, Enabled: true},
	} {
		if err := ApplyScanRuleScope(rule, "https://example.com/news", ScanRuleScopeExact); err != nil {
			t.Fatalf("apply rule scope: %v", err)
		}
		if err := database.CreateScanRuleTemplate(rule); err != nil {
			t.Fatalf("create scan rule: %v", err)
		}
	}

	res, err := scanHTMLWithRules(html, "https://example.com/news")
	if err != nil {
		t.Fatalf("scanHTMLWithRules failed: %v", err)
	}
	if len(res.Containers) != 2 {
		t.Fatalf("expected two candidates, got %+v", res.Containers)
	}
	if res.Containers[0].Strategy != "template_high-priority" || res.Containers[1].Strategy != "template_low-priority" {
		t.Fatalf("candidates must be ordered by priority: %+v", res.Containers)
	}
}
