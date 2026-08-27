package monitor

import (
	"strings"
	"testing"
)

// 特征化测试：锁定旧配置修复启发式的现有行为。
// 这些测试描述的是「当前行为」而非「期望设计」；若未来决定移除兼容层，
// 应先迁移受影响的存量监控配置，再删除对应用例。

const compatNarrowHTML = `<html><body>
<nav class="main-nav"><ul class="nav-list"><li><a href="/home">首页</a></li></ul></nav>
<ul class="news-list">
<li><a href="/a.html">2026 年第一季度招聘公告汇总</a><span>2026-07-07</span></li>
<li><a href="/b.html">2026 年上半年录用人员公示</a><span>2026-07-06</span></li>
</ul>
<footer><ul class="foot-list"><li><a href="/about">关于</a></li></ul></footer>
</body></html>`

const compatRecoverHTML = `<html><body>
<div class="list">
<a href="/n1.html">2026 年度考试录用公告第一号</a>
<a href="/n2.jhtml">2026 年度考试录用公告第二号</a>
<a href="/n3.htm">2026 年度考试录用公告第三号</a>
<ul class="meta"><li><span>浏览 120</span></li><li><span>评论 30</span></li></ul>
</div>
</body></html>`

func TestCompatNarrowsBroadContainerToBestList(t *testing.T) {
	items, err := NewExtractor(SiteSelectors{
		Container: "ul",
		Item:      "li",
		Fields:    []FieldConfig{{Name: "title", Type: "text"}},
	}).Extract(compatNarrowHTML)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected narrowing to news-list (2 items), got %d: %+v", len(items), items)
	}
	title, _ := items[0]["title"].(string)
	if !strings.Contains(title, "招聘公告") {
		t.Fatalf("expected news-list titles, got: %+v", items)
	}
}

func TestCompatKeepsAmbiguousBroadContainer(t *testing.T) {
	// 两个候选得分接近（都无日期/详情链接加成）时不得收窄，
	// 避免破坏确实需要合并多个列表的存量配置。
	html := `<html><body>
	<ul class="part-a"><li>条目甲内容足够长</li><li>条目乙内容足够长</li></ul>
	<ul class="part-b"><li>条目丙内容足够长</li></ul>
	</body></html>`
	items, err := NewExtractor(SiteSelectors{
		Container: "ul",
		Item:      "li",
		Fields:    []FieldConfig{{Name: "title", Type: "text"}},
	}).Extract(html)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected merged extraction without narrowing, got %d", len(items))
	}
}

func TestCompatRecoversMetadataItemSelector(t *testing.T) {
	items, err := NewExtractor(SiteSelectors{
		Container: "div.list",
		Item:      "ul.meta li",
		Fields:    []FieldConfig{{Name: "title", Type: "text"}},
	}).Extract(compatRecoverHTML)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected recovery to direct anchors (3 items), got %d: %+v", len(items), items)
	}
	url, _ := items[0]["url"].(string)
	if !strings.HasSuffix(url, "/n1.html") {
		t.Fatalf("expected url injected from anchor href, got: %q", url)
	}
	title, _ := items[0]["title"].(string)
	if !strings.Contains(title, "公告") {
		t.Fatalf("unexpected title: %+v", items[0])
	}
}

func TestCompatPreciseConfigUnaffected(t *testing.T) {
	html := `<html><body><ul class="news-list">
	<li><a href="/1.html">第一条通知</a></li>
	<li><a href="/2.html">第二条通知</a></li>
	</ul></body></html>`
	items, err := NewExtractor(SiteSelectors{
		Container: "ul.news-list",
		Item:      "li",
		Fields: []FieldConfig{
			{Name: "title", Selector: "a", Type: "text"},
			{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
		},
	}).Extract(html)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if url, _ := items[0]["url"].(string); url != "/1.html" {
		t.Fatalf("unexpected url: %+v", items[0])
	}
}
