package monitor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/notify"
)

func setupMonitorTestDB(t *testing.T) {
	t.Helper()
	original := database.DB
	dir, err := os.MkdirTemp("", "gentry-monitor-test-*")
	if err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := database.Init(filepath.Join(dir, "monitor-test.db")); err != nil {
		t.Fatalf("init test database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		database.DB = original
		_ = os.RemoveAll(dir)
	})
}

func TestSaveResultsKeepsNewestPageItemFirstWithinEachFetch(t *testing.T) {
	setupMonitorTestDB(t)

	site := &database.Site{Name: "ordered-updates", URL: "https://example.com"}
	if err := database.GetDB().Create(site).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}
	monitor := &Monitor{site: site}

	firstFetch := []ExtractResult{
		{"title": "newest", "url": "https://example.com/3"},
		{"title": "middle", "url": "https://example.com/2"},
		{"title": "oldest", "url": "https://example.com/1"},
	}
	if err := monitor.saveResults(firstFetch, true); err != nil {
		t.Fatalf("save first fetch: %v", err)
	}

	time.Sleep(time.Millisecond)
	if err := monitor.saveResults([]ExtractResult{
		{"title": "latest fetch", "url": "https://example.com/4"},
	}, false); err != nil {
		t.Fatalf("save second fetch: %v", err)
	}

	var records []database.UpdateRecord
	if err := database.GetDB().Where("site_id = ?", site.ID).
		Order("created_at desc, id asc").Find(&records).Error; err != nil {
		t.Fatalf("load records: %v", err)
	}

	want := []string{"latest fetch", "newest", "middle", "oldest"}
	if len(records) != len(want) {
		t.Fatalf("record count = %d, want %d", len(records), len(want))
	}
	for i, title := range want {
		if records[i].Title != title {
			t.Fatalf("record %d title = %q, want %q", i, records[i].Title, title)
		}
	}
	if !records[1].CreatedAt.Equal(records[2].CreatedAt) || !records[2].CreatedAt.Equal(records[3].CreatedAt) {
		t.Fatal("records from one fetch must share the same created_at")
	}
}

func TestValidateExtractionSupportsPresenceLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><ul><li><a href="/a">公告 A</a></li><li><a href="/b">公告 B</a></li></ul></body></html>`))
	}))
	defer server.Close()

	site := &database.Site{
		Name: "presence-validation", URL: server.URL, Container: "ul", Item: "li",
		Fields: []database.SiteField{
			{Name: "title", Selector: "a", Type: "text"},
			{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
		},
	}
	report, err := ValidateExtraction(context.Background(), site)
	if err != nil {
		t.Fatalf("validate extraction: %v", err)
	}
	if report.ExtractedItems != 2 || len(report.Samples) != 2 {
		t.Fatalf("unexpected validation report: %+v", report)
	}
	if report.Samples[0].ItemKey == report.Samples[1].ItemKey {
		t.Fatalf("list samples must have stable unique keys: %+v", report.Samples)
	}
	if report.Samples[0].Raw != "公告 A" {
		t.Fatalf("unexpected sample raw value: %+v", report.Samples[0])
	}
}

func TestValidateExtractionRejectsEmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><p>没有列表</p></body></html>`))
	}))
	defer server.Close()

	site := &database.Site{
		Name: "empty-validation", URL: server.URL, Container: "ul", Item: "li",
		Fields: []database.SiteField{{Name: "title", Selector: "a", Type: "text"}},
	}
	if _, err := ValidateExtraction(context.Background(), site); err == nil {
		t.Fatal("expected empty extraction to fail validation")
	}
}

func TestCheckNowSerializesConcurrentChecks(t *testing.T) {
	setupMonitorTestDB(t)
	var active int32
	var maxActive int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&active, 1)
		for {
			maximum := atomic.LoadInt32(&maxActive)
			if current <= maximum || atomic.CompareAndSwapInt32(&maxActive, maximum, current) {
				break
			}
		}
		time.Sleep(80 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		_, _ = w.Write([]byte(`<html><body><ul><li><a href="/a">公告 A</a></li></ul></body></html>`))
	}))
	defer server.Close()

	site := &database.Site{
		Name: "serialized-monitor", URL: server.URL, Container: "ul", Item: "li", ConfigVersion: 1,
		Fields: []database.SiteField{
			{Name: "title", Selector: "a", Type: "text"},
			{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
		},
	}
	if err := database.CreateSiteWithFields(site); err != nil {
		t.Fatal(err)
	}
	m := NewDetachedMonitor(site)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.CheckNow(context.Background())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("check failed: %v", err)
		}
	}
	if max := atomic.LoadInt32(&maxActive); max != 1 {
		t.Errorf("fetches must be serialized, max concurrent fetches = %d", max)
	}
}

// TestCheckRetriesPendingNotifications 验证：某次检查因推送失败留下的待推送记录，
// 在后续检查中即使没有新内容也会被补推（防止「待推送」永久挂起）。
func TestCheckRetriesPendingNotifications(t *testing.T) {
	setupMonitorTestDB(t)

	var pushCount atomic.Int32
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><ul><li><a href="/a">公告 A</a></li></ul></body></html>`))
	}))
	defer page.Close()
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook.Close()

	site := &database.Site{
		Name: "retry-pending", URL: page.URL, Container: "ul", Item: "li", ConfigVersion: 1,
		Fields: []database.SiteField{
			{Name: "title", Selector: "a", Type: "text"},
			{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
		},
	}
	if err := database.CreateSiteWithFields(site); err != nil {
		t.Fatal(err)
	}

	// 配置一个 webhook 推送账户并绑定到站点
	account := &database.NotificationAccount{
		Name:       "test-webhook",
		Service:    "webhook",
		ConfigJSON: fmt.Sprintf(`{"url":%q}`, webhook.URL),
	}
	if err := database.GetDB().Create(account).Error; err != nil {
		t.Fatal(err)
	}
	site.NotifyAccountIDs = fmt.Sprintf("[%d]", account.ID)
	if err := database.GetDB().Model(site).Update("notify_account_ids", site.NotifyAccountIDs).Error; err != nil {
		t.Fatal(err)
	}
	notify.SetEnabled(true)
	defer notify.SetEnabled(false)

	m := NewDetachedMonitor(site)

	// 第一次检查：建立基线（页面内容入库，notified=true，不会推送）
	if _, err := m.CheckNow(context.Background()); err != nil {
		t.Fatalf("first check: %v", err)
	}
	if pushCount.Load() != 0 {
		t.Fatalf("baseline must not push, got %d pushes", pushCount.Load())
	}

	// 手动插入一条待推送记录，模拟「上次推送失败留下的待推送条目」
	pending := &database.UpdateRecord{
		SiteID:  site.ID,
		Title:   "公告 B",
		URL:     page.URL + "/b",
		Content: `{"title":"公告 B","url":"` + page.URL + `/b"}`,
	}
	if err := database.GetDB().Create(pending).Error; err != nil {
		t.Fatal(err)
	}

	// 第二次检查：页面无新内容，但应补推遗留的待推送记录
	if _, err := m.CheckNow(context.Background()); err != nil {
		t.Fatalf("second check: %v", err)
	}
	if pushCount.Load() != 1 {
		t.Fatalf("pending record must be pushed once, got %d", pushCount.Load())
	}

	var updated database.UpdateRecord
	if err := database.GetDB().First(&updated, pending.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !updated.Notified {
		t.Fatal("pending record must be marked notified after successful push")
	}
}

func TestFirstBaselineRecordsMarkedNotified(t *testing.T) {
	setupMonitorTestDB(t)

	site := &database.Site{Name: "baseline-notified", URL: "https://example.com"}
	if err := database.GetDB().Create(site).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}
	monitor := &Monitor{site: site}

	// 首次基线：markNotified=true
	baseline := []ExtractResult{
		{"title": "条目 A", "url": "https://example.com/a"},
		{"title": "条目 B", "url": "https://example.com/b"},
	}
	if err := monitor.saveResults(baseline, true); err != nil {
		t.Fatalf("save baseline: %v", err)
	}

	// 后续新条目：markNotified=false
	time.Sleep(time.Millisecond)
	if err := monitor.saveResults([]ExtractResult{
		{"title": "新增 C", "url": "https://example.com/c"},
	}, false); err != nil {
		t.Fatalf("save new item: %v", err)
	}

	var records []database.UpdateRecord
	if err := database.GetDB().Where("site_id = ?", site.ID).
		Order("created_at desc, id asc").Find(&records).Error; err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("record count = %d, want 3", len(records))
	}
	// 基线条目必须标记为已推送，避免虚假进入"待推送"统计
	for _, r := range records {
		if r.Title == "条目 A" || r.Title == "条目 B" {
			if !r.Notified {
				t.Fatalf("baseline record %q must be marked notified", r.Title)
			}
		}
		if r.Title == "新增 C" && r.Notified {
			t.Fatalf("new item %q must remain unnotified", r.Title)
		}
	}
}

// TestPushLogWrittenOnSuccess 验证：推送全部成功后写入一条 status=success 的推送记录，
// 且关联的 update_record 被标记为已通知。
func TestPushLogWrittenOnSuccess(t *testing.T) {
	setupMonitorTestDB(t)

	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook.Close()

	site := &database.Site{Name: "pushlog-success", URL: "https://example.com"}
	if err := database.GetDB().Create(site).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}
	account := &database.NotificationAccount{
		Name:       "test-webhook",
		Service:    "webhook",
		ConfigJSON: fmt.Sprintf(`{"url":%q}`, webhook.URL),
	}
	if err := database.GetDB().Create(account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	site.NotifyAccountIDs = fmt.Sprintf("[%d]", account.ID)
	if err := database.GetDB().Model(site).Update("notify_account_ids", site.NotifyAccountIDs).Error; err != nil {
		t.Fatalf("bind account: %v", err)
	}
	notify.SetEnabled(true)
	defer notify.SetEnabled(false)

	record := &database.UpdateRecord{
		SiteID:  site.ID,
		Title:   "公告 A",
		URL:     "https://example.com/a",
		Content: `{"title":"公告 A","url":"https://example.com/a"}`,
	}
	if err := database.GetDB().Create(record).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}

	m := NewDetachedMonitor(site)
	items := []ExtractResult{{"title": "公告 A", "url": "https://example.com/a"}}
	m.sendCombinedNotification(items, []uint{record.ID})

	var log database.PushLog
	if err := database.GetDB().First(&log).Error; err != nil {
		t.Fatalf("push log not written: %v", err)
	}
	if log.Status != "success" {
		t.Fatalf("push log status = %q, want success", log.Status)
	}
	if log.SiteName != site.Name || log.ItemCount != 1 {
		t.Fatalf("push log site=%q count=%d", log.SiteName, log.ItemCount)
	}

	var updated database.UpdateRecord
	if err := database.GetDB().First(&updated, record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !updated.Notified {
		t.Fatal("record must be marked notified after successful push")
	}
}

// TestPushLogWrittenOnSkipped 验证：推送开关关闭时写入一条 status=skipped 的推送记录，
// 且关联记录不被标记（等待用户开启开关后补推）。
func TestPushLogWrittenOnSkipped(t *testing.T) {
	setupMonitorTestDB(t)

	site := &database.Site{Name: "pushlog-skipped", URL: "https://example.com"}
	if err := database.GetDB().Create(site).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}
	record := &database.UpdateRecord{
		SiteID:  site.ID,
		Title:   "公告 B",
		URL:     "https://example.com/b",
		Content: `{"title":"公告 B","url":"https://example.com/b"}`,
	}
	if err := database.GetDB().Create(record).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}
	// 确保开关关闭（默认即关闭，此处显式设置以便测试独立）
	notify.SetEnabled(false)
	defer notify.SetEnabled(false)

	m := NewDetachedMonitor(site)
	items := []ExtractResult{{"title": "公告 B", "url": "https://example.com/b"}}
	m.sendCombinedNotification(items, []uint{record.ID})

	var log database.PushLog
	if err := database.GetDB().First(&log).Error; err != nil {
		t.Fatalf("push log not written: %v", err)
	}
	if log.Status != "skipped" {
		t.Fatalf("push log status = %q, want skipped", log.Status)
	}
	if log.Reason == "" {
		t.Fatal("skipped push log must carry a reason")
	}

	var updated database.UpdateRecord
	if err := database.GetDB().First(&updated, record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Notified {
		t.Fatal("record must remain unnotified when push is skipped")
	}
}

// TestBuildNotifyContentDefault 验证：未配置模板时使用内置默认标题/内容格式。
func TestBuildNotifyContentDefault(t *testing.T) {
	setupMonitorTestDB(t)
	items := []ExtractResult{
		{"title": "公告 A", "url": "https://example.com/a"},
		{"title": "公告 B", "url": "https://example.com/b"},
	}

	title, content := buildNotifyContent("测试站点", items)
	if title != "测试站点 有 2 条更新" {
		t.Fatalf("default title = %q", title)
	}
	want := "最新更新内容：\n1. 公告 A\n   https://example.com/a\n2. 公告 B\n   https://example.com/b"
	if content != want {
		t.Fatalf("default content = %q, want %q", content, want)
	}
}

// TestBuildNotifyContentTemplate 验证：选中的模板正确替换全部占位符，
// 未选中的模板不生效。
func TestBuildNotifyContentTemplate(t *testing.T) {
	setupMonitorTestDB(t)
	templates := []database.PushTemplate{
		{Name: "简洁", TitleTemplate: "{site_name} 有 {count} 条更新", ContentTemplate: "最新更新内容：\n{items}"},
		{Name: "正式", TitleTemplate: "{site_name} 新增 {count} 条公告", ContentTemplate: "公告汇总（{titles}）\n{items}\n——来自 {site_name}"},
	}
	if err := database.SetPushTemplates(templates); err != nil {
		t.Fatal(err)
	}
	if err := database.SetSetting("push_template_active", "正式"); err != nil {
		t.Fatal(err)
	}
	items := []ExtractResult{
		{"title": "公告 A", "url": "https://example.com/a"},
		{"title": "公告 B", "url": "https://example.com/b"},
	}

	title, content := buildNotifyContent("测试站点", items)
	if title != "测试站点 新增 2 条公告" {
		t.Fatalf("template title = %q", title)
	}
	want := "公告汇总（公告 A、公告 B）\n1. 公告 A\n   https://example.com/a\n2. 公告 B\n   https://example.com/b\n——来自 测试站点"
	if content != want {
		t.Fatalf("template content = %q, want %q", content, want)
	}

	// 切换选中到另一个模板
	if err := database.SetSetting("push_template_active", "简洁"); err != nil {
		t.Fatal(err)
	}
	title, _ = buildNotifyContent("测试站点", items)
	if title != "测试站点 有 2 条更新" {
		t.Fatalf("switched template title = %q", title)
	}
}

// TestBuildNotifyContentEmptyTemplate 验证：模板渲染结果为空（如 {items} 但无条目）时回退默认格式；
// 未知占位符会原样保留（便于用户发现拼写错误），不会触发回退。
func TestBuildNotifyContentEmptyTemplate(t *testing.T) {
	setupMonitorTestDB(t)
	templates := []database.PushTemplate{
		{Name: "空内容", ContentTemplate: "{items}"},
		{Name: "未知变量", TitleTemplate: "{unknown_var}"},
	}
	if err := database.SetPushTemplates(templates); err != nil {
		t.Fatal(err)
	}

	// 无条目时 {items} 渲染为空 → 内容回退默认
	if err := database.SetSetting("push_template_active", "空内容"); err != nil {
		t.Fatal(err)
	}
	_, content := buildNotifyContent("测试站点", nil)
	if content != "最新更新内容：" {
		t.Fatalf("empty template must fall back to default content, got %q", content)
	}

	// 未知占位符原样保留
	if err := database.SetSetting("push_template_active", "未知变量"); err != nil {
		t.Fatal(err)
	}
	title, _ := buildNotifyContent("测试站点", []ExtractResult{{"title": "公告 A", "url": "https://example.com/a"}})
	if title != "{unknown_var}" {
		t.Fatalf("unknown placeholder must be kept literal, got %q", title)
	}
}
