package monitor

import (
	"strings"
	"testing"
)

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

func TestSmartScanBuildsScopedSelectorForUnclassifiedContentList(t *testing.T) {
	html := `<body>
		<div class="sideMenu"><ul class="submenu">
			<li><a href="/zfgb/">政府公报</a></li><li><a href="/cfqz/">处罚强制</a></li><li><a href="/xzfw/">行政许可和对外管理服务</a></li>
		</ul></div>
		<div class="scroll_main1"><div class="zfxxgk_zdgkc"><ul>
			<li><a href="/2026/07-23/3653721.html">2026年公开选调工作人员公告</a><span>2026-07-23</span></li>
			<li><a href="/2026/07-17/3652898.html">2026年公开招聘递补公告</a><span>2026-07-17</span></li>
			<li><a href="/2026/07-01/3650588.html">2026年公开招聘资格确认公告</a><span>2026-07-01</span></li>
		</ul></div></div>
	</body>`
	res, err := smartScanHTML(html, []string{"公开选调"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatal("expected announcement candidate")
	}
	candidate := res.Containers[0]
	if candidate.Config.Container == "ul" {
		t.Fatalf("scanner must scope a non-unique ul selector: %+v", candidate.Config)
	}
	items, err := NewExtractor(scanConfigToSelectors(candidate.Config)).Extract(html)
	if err != nil {
		t.Fatalf("extract candidate config: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 scoped announcement items, got %d: %+v", len(items), items)
	}
}

func TestSmartScanPrefersDirectArticleAnchorsOverMetadataLists(t *testing.T) {
	html := `<body><div class="common-box dir-list"><div class="box-content"><ul class="page-list">
		<a class="list_item_pubinfo_a" href="/D49Y/10166257.jhtml"><span>2026年郑东新区事业单位公开招聘笔试拟加分人员公示</span></a>
		<ul class="pubinfo-card"><li>330126167/2026-00059</li><li>郑东新区组织人事和社会保障局</li><li>事业单位公开招聘</li><li>2026-07-21</li></ul>
		<ul class="pubinfo-card-list-m hidden m-shown"><li>2026-07-21</li><li>330126167/2026-00059</li></ul>
		<a class="list_item_pubinfo_a" href="/D49Y/10135265.jhtml"><span>2026年郑东新区事业单位公开招聘笔试加分资格审核通知</span></a>
		<ul class="pubinfo-card"><li>330126167/2026-00053</li><li>郑东新区组织人事和社会保障局</li><li>事业单位招聘</li><li>2026-07-02</li></ul>
		<ul class="pubinfo-card-list-m hidden m-shown"><li>2026-07-02</li><li>330126167/2026-00053</li></ul>
		<a class="list_item_pubinfo_a" href="/D1703X/9699707.jhtml"><span>2025年郑东新区事业单位公开招聘考试总成绩及体检公告</span></a>
		<ul class="pubinfo-card"><li>330126167/2025-00081</li><li>郑东新区组织人事和社会保障局</li><li>事业单位招聘</li><li>2025-11-02</li></ul>
		<ul class="pubinfo-card-list-m hidden m-shown"><li>2025-11-02</li><li>330126167/2025-00081</li></ul>
	</ul></div></div></body>`
	res, err := smartScanHTML(html, []string{"公开招聘"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatal("expected public information list candidate")
	}
	candidate := res.Containers[0]
	if candidate.Config.Item != "a.list_item_pubinfo_a" {
		t.Fatalf("item selector = %q, want direct article anchors; candidates=%+v", candidate.Config.Item, res.Containers)
	}
	items, err := NewExtractor(scanConfigToSelectors(candidate.Config)).Extract(html)
	if err != nil {
		t.Fatalf("extract candidate config: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 articles, got %d: %+v", len(items), items)
	}
	if items[0]["title"] != "2026年郑东新区事业单位公开招聘笔试拟加分人员公示" || items[0]["url"] != "/D49Y/10166257.jhtml" {
		t.Fatalf("unexpected first article: %+v", items[0])
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

func TestSmartScanGenericULList(t *testing.T) {
	html := `<body><main><ul class="news-list"><li><a href="/notice-1.html">2026 面试公告发布</a><time>2026-07-07</time></li><li><a href="/notice-2.html">2026 录用公示</a><time>2026-07-06</time></li></ul></main></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("Smart scan failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected at least one container")
	}
	candidate := res.Containers[0]
	if candidate.Config.Container == "" || candidate.Config.Item == "" {
		t.Fatalf("expected persistable config, got %+v", candidate.Config)
	}
	if len(candidate.Config.Fields) == 0 || candidate.Config.Fields[0].Name != "title" {
		t.Fatalf("expected title field in config: %+v", candidate.Config.Fields)
	}
	selectors := scanConfigToSelectors(candidate.Config)
	ex := NewExtractor(selectors)
	items, err := ex.Extract(html)
	if err != nil {
		t.Fatalf("extract with config failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected extracted items from candidate config")
	}
	if !strings.Contains(getTitle(items[0]), "面试公告发布") {
		t.Fatalf("unexpected extracted title: %v", items[0]["title"])
	}
}

func TestSmartScanDoesNotUseSiteSpecificBuiltInRules(t *testing.T) {
	html := `<body><div><div class="time_line_item__rQXQP"><ul class="ant-timeline"><li class="ant-timeline-item">2026.070718:48深圳水官高速延长线停止收费，两个月前水官高速已停收</li><li class="ant-timeline-item">18:42智谱基石解禁日获近七成基石投资者表态长期持有</li></ul></div></div></body>`
	res, err := smartScanHTMLWithSettings(html, &ScanSettings{URL: "https://www.thepaper.cn/expressNews", Keywords: []string{"深圳水官高速延长线停止收费", "智谱基石解禁日"}})
	if err != nil {
		t.Fatalf("smartScanHTMLWithSettings failed: %v", err)
	}
	for _, candidate := range res.Containers {
		if strings.HasPrefix(candidate.Strategy, "rule_") {
			t.Fatalf("unexpected site-specific built-in rule: %+v", candidate)
		}
	}
}

func TestSmartScanThePaperTimelineLikeList(t *testing.T) {
	html := `<body><div><div class="time_line_item__rQXQP"><ul class="ant-timeline"><li class="ant-timeline-item">2026.070718:48深圳水官高速延长线停止收费，两个月前水官高速已停收</li><li class="ant-timeline-item">18:42智谱基石解禁日获近七成基石投资者表态长期持有</li></ul></div></div></body>`
	res, err := smartScanHTML(html, []string{"深圳水官高速延长线停止收费", "智谱基石解禁日"})
	if err != nil {
		t.Fatalf("Smart scan failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected timeline container")
	}
	candidate := res.Containers[0]
	if candidate.Config.Item == "" {
		t.Fatalf("expected item selector")
	}
	selectors := scanConfigToSelectors(candidate.Config)
	ex := NewExtractor(selectors)
	items, err := ex.Extract(html)
	if err != nil {
		t.Fatalf("extract with config failed: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(items))
	}
	if !strings.Contains(getTitle(items[0]), "深圳水官高速延长线停止收费") {
		t.Fatalf("unexpected first title: %v", items[0]["title"])
	}
}

func TestSmartScanGenericCardList(t *testing.T) {
	html := `<body><section class="content-list"><div class="list-item"><h3 class="title"><a href="/a.html">重点项目名单公示</a></h3><span class="date">2026-07-07</span></div><div class="list-item"><h3 class="title"><a href="/b.html">招聘面试安排</a></h3><span class="date">2026-07-06</span></div></section></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("Smart scan failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected card list candidate")
	}
	candidate := res.Containers[0]
	if candidate.Strategy == "" {
		t.Fatalf("expected strategy metadata")
	}
	if candidate.Confidence == 0 {
		t.Fatalf("expected non-zero confidence")
	}
	if candidate.Config.Item == "" {
		t.Fatalf("expected item selector")
	}
	selectors := scanConfigToSelectors(candidate.Config)
	ex := NewExtractor(selectors)
	items, err := ex.Extract(html)
	if err != nil {
		t.Fatalf("extract with config failed: %v", err)
	}
	if len(items) == 0 || !strings.Contains(getTitle(items[1]), "面试安排") {
		t.Fatalf("unexpected extracted items: %+v", items)
	}
}

func TestSmartScanLinkClusterStrategy(t *testing.T) {
	html := `<body><section class="links"><a href="/a">深圳楼市新政发布</a><a href="/b">面试公告更新</a><a href="/c">录用公示名单</a></section></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected link cluster candidate")
	}
	found := false
	for _, candidate := range res.Containers {
		if candidate.Strategy == "link_cluster" || strings.Contains(strings.Join(candidate.Diagnostics, "|"), "直系链接子项") || strings.Contains(strings.Join(candidate.Diagnostics, "|"), "同时命中策略 link_cluster") {
			found = true
			selectors := scanConfigToSelectors(candidate.Config)
			ex := NewExtractor(selectors)
			items, err := ex.Extract(html)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}
			if len(items) == 0 {
				t.Fatalf("expected extracted link-cluster items")
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected link_cluster strategy or diagnostic in candidates: %+v", res.Containers)
	}
}

func TestSmartScanTableStrategy(t *testing.T) {
	html := `<body><table class="notice-table"><tbody><tr><td><a href="/1">深圳招聘公告</a></td><td>2026-07-07</td></tr><tr><td><a href="/2">深圳录用公示</a></td><td>2026-07-06</td></tr></tbody></table></body>`
	res, err := smartScanHTML(html, []string{"招聘"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected table candidate")
	}
	found := false
	for _, candidate := range res.Containers {
		if candidate.Strategy == "table_rows" || strings.Contains(strings.Join(candidate.Diagnostics, "|"), "表格数据") {
			found = true
			selectors := scanConfigToSelectors(candidate.Config)
			ex := NewExtractor(selectors)
			items, err := ex.Extract(html)
			if err != nil {
				t.Fatalf("extract failed: %v", err)
			}
			if len(items) == 0 {
				t.Fatalf("expected extracted table items")
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected table_rows strategy or diagnostic in candidates: %+v", res.Containers)
	}
}

func TestSmartScanFieldInferenceIncludesDateAndSummaryWhenAvailable(t *testing.T) {
	html := `<body><section class="content-list"><div class="list-item"><h3 class="title"><a href="/a.html">正式面试公告</a></h3><p class="summary">请于本周五前确认参加面试。</p><span class="date">2026-07-07</span></div><div class="list-item"><h3 class="title"><a href="/b.html">第二条录用公示</a></h3><p class="summary">录用公示名单已经发布。</p><span class="date">2026-07-06</span></div><div class="list-item"><h3 class="title"><a href="/c.html">第三条招考通知</a></h3><p class="summary">请持续关注后续安排。</p><span class="date">2026-07-05</span></div></section></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected candidate")
	}
	candidate := res.Containers[0]
	fieldNames := map[string]bool{}
	for _, field := range candidate.Config.Fields {
		fieldNames[field.Name] = true
	}
	if !fieldNames["date"] {
		t.Fatalf("expected inferred date field: %+v", candidate.Config.Fields)
	}
	if !fieldNames["summary"] {
		t.Fatalf("expected inferred summary field: %+v", candidate.Config.Fields)
	}
	selectors := scanConfigToSelectors(candidate.Config)
	ex := NewExtractor(selectors)
	items, err := ex.Extract(html)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected extracted items")
	}
	if items[0]["date"] != "2026-07-07" {
		t.Fatalf("unexpected extracted date: %v", items[0]["date"])
	}
	summary, ok := items[0]["summary"].(string)
	if !ok || !strings.Contains(summary, "确认参加面试") {
		t.Fatalf("unexpected extracted summary: %v", items[0]["summary"])
	}
}

func TestScanCandidatePreviewSaveParity(t *testing.T) {
	html := `<body><ul class="news-list"><li><a href="/notice-1.html">2026 面试公告发布</a></li><li><a href="/notice-2.html">2026 录用公示</a></li></ul></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected candidate")
	}
	candidate := res.Containers[0]
	if len(candidate.SampleItems) == 0 {
		t.Fatalf("expected preview samples")
	}
	selectors := scanConfigToSelectors(candidate.Config)
	ex := NewExtractor(selectors)
	items, err := ex.Extract(html)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected saved-config extraction items")
	}
	previewTitle := getTitle(candidate.SampleItems[0])
	runtimeTitle := getTitle(items[0])
	if previewTitle != runtimeTitle {
		t.Fatalf("preview/save mismatch: preview=%q runtime=%q", previewTitle, runtimeTitle)
	}
}

func TestSmartScanRanksContentListAboveNavNoise(t *testing.T) {
	html := `<body><div class="nav"><a href="/home">首页</a><a href="/more">更多</a><a href="/notice">面试公告</a></div><section class="content-list"><div class="list-item"><h3 class="title"><a href="/a.html">正式面试公告</a></h3></div><div class="list-item"><h3 class="title"><a href="/b.html">第二条录用公示</a></h3></div><div class="list-item"><h3 class="title"><a href="/c.html">第三条招考通知</a></h3></div></section></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected ranked candidates")
	}
	first := res.Containers[0]
	if strings.Contains(first.Config.Container, ".nav") || first.Strategy == "link_cluster" {
		t.Fatalf("expected content list to outrank nav noise, got %+v", first)
	}
}

func TestSmartScanSuppressesFooterAndBreadcrumbNoise(t *testing.T) {
	html := `<body><div class="breadcrumb">招考 > 面试公告</div><footer id="footer-links"><a href="/notice">面试公告</a><a href="/about">关于我们</a><a href="/contact">联系我们</a></footer><main><ul class="news-list"><li><a href="/1">正式面试公告</a></li><li><a href="/2">录用名单</a></li><li><a href="/3">体检通知</a></li></ul></main></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected candidates")
	}
	first := res.Containers[0]
	if strings.Contains(first.Config.Container, "footer") || strings.Contains(first.Config.Container, "breadcrumb") {
		t.Fatalf("expected footer/breadcrumb noise to be suppressed, got %+v", first)
	}
}

func TestSmartScanOrdersMixedCandidatesDeterministically(t *testing.T) {
	html := `<body><section class="content-list"><div class="list-item"><h3 class="title"><a href="/a.html">正式面试公告</a></h3></div><div class="list-item"><h3 class="title"><a href="/b.html">第二条录用公示</a></h3></div><div class="list-item"><h3 class="title"><a href="/c.html">第三条招考通知</a></h3></div></section><table class="notice-table"><tbody><tr><td><a href="/t1">表格面试公告</a></td></tr><tr><td><a href="/t2">表格录用名单</a></td></tr></tbody></table><section class="links"><a href="/l1">首页</a><a href="/l2">面试公告</a><a href="/l3">更多</a></section></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) < 2 {
		t.Fatalf("expected multiple candidates")
	}
	if res.Containers[0].Strategy == "link_cluster" {
		t.Fatalf("expected link cluster not to outrank richer content: %+v", res.Containers)
	}
}

func TestSmartScanTopThreeKeepsRealContent(t *testing.T) {
	html := `<body><div id="header-nav"><a href="/home">首页</a><a href="/notice">面试公告</a><a href="/login">登录</a></div><section class="content-list"><div class="list-item"><h3 class="title"><a href="/a.html">正式面试公告</a></h3></div><div class="list-item"><h3 class="title"><a href="/b.html">第二条录用公示</a></h3></div><div class="list-item"><h3 class="title"><a href="/c.html">第三条招考通知</a></h3></div></section><table class="notice-table"><tbody><tr><td><a href="/t1">表格面试公告</a></td></tr><tr><td><a href="/t2">表格录用名单</a></td></tr></tbody></table><section class="promo-links"><a href="/p1">更多</a><a href="/p2">专题</a><a href="/p3">广告</a></section><footer class="footer-links"><a href="/contact">联系我们</a><a href="/notice2">面试公告</a><a href="/about">关于我们</a></footer></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) != 3 {
		t.Fatalf("expected top 3 candidates, got %d", len(res.Containers))
	}
	foundContent := false
	for _, candidate := range res.Containers {
		if strings.Contains(candidate.Config.Container, "content-list") {
			foundContent = true
		}
	}
	if !foundContent {
		t.Fatalf("expected real content container to survive top-3 truncation: %+v", res.Containers)
	}
}

func TestSmartScanPrimaryStrategySelectionIsStable(t *testing.T) {
	html := `<body><section class="links"><a href="/a">正式面试公告</a><a href="/b">录用公示</a><a href="/c">第三条通知</a></section></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected candidates")
	}
	first := res.Containers[0]
	if first.Strategy == "" {
		t.Fatalf("expected stable primary strategy")
	}
	if !strings.Contains(strings.Join(first.Diagnostics, "|"), "提取到") {
		t.Fatalf("expected diagnostics to remain informative: %+v", first.Diagnostics)
	}
}

func TestSmartScanSuppressesNoiseByIDAsWellAsClass(t *testing.T) {
	html := `<body><div id="sidebar"><a href="/notice">面试公告</a><a href="/more">更多</a><a href="/login">登录</a></div><main><ul class="news-list"><li><a href="/1">正式面试公告</a></li><li><a href="/2">录用名单</a></li><li><a href="/3">体检通知</a></li></ul></main></body>`
	res, err := smartScanHTML(html, []string{"面试"})
	if err != nil {
		t.Fatalf("smartScanHTML failed: %v", err)
	}
	if len(res.Containers) == 0 {
		t.Fatalf("expected candidates")
	}
	first := res.Containers[0]
	if strings.Contains(first.Config.Container, "#sidebar") || strings.Contains(first.Config.Container, ".sidebar") {
		t.Fatalf("expected sidebar noise to be suppressed, got %+v", first)
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
