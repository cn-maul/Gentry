package monitor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cn-maul/Gentry/database"
)

func setupMonitorPersistenceDB(t *testing.T) {
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

func createPriceMonitorSite(t *testing.T) *database.Site {
	t.Helper()
	site := &database.Site{
		Name:           "price-monitor",
		URL:            "https://example.com/product/1",
		Container:      "body",
		StrategyType:   "field_transition",
		StrategyConfig: `{"type":"field_transition","identity":{"source":"source_url"},"conditions":[{"field":"price","value_type":"money","operator":"decreased"}],"on_first_baseline":"silent"}`,
		FieldDataTypes: `{"price":"money"}`,
		ConfigVersion:  1,
		Fields: []database.SiteField{
			{Name: "title", Selector: "h1", Type: "text"},
			{Name: "price", Selector: ".price", Type: "text"},
		},
	}
	if err := database.CreateSiteWithFields(site); err != nil {
		t.Fatalf("create site: %v", err)
	}
	return site
}

func TestSaveResultsKeepsNewestPageItemFirstWithinEachFetch(t *testing.T) {
	setupMonitorPersistenceDB(t)

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
	if err := monitor.saveResults(firstFetch); err != nil {
		t.Fatalf("save first fetch: %v", err)
	}

	time.Sleep(time.Millisecond)
	if err := monitor.saveResults([]ExtractResult{
		{"title": "latest fetch", "url": "https://example.com/4"},
	}); err != nil {
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

func TestValidateExtractionReturnsPriceSamples(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1>测试商品</h1><span class="price">¥123.45</span></body></html>`))
	}))
	defer server.Close()

	site := &database.Site{
		Name: "validation-sample", URL: server.URL, Container: "body", StrategyType: "field_transition",
		StrategyConfig: `{"type":"field_transition","identity":{"source":"source_url"},"conditions":[{"field":"price","value_type":"money","operator":"decreased"}],"on_first_baseline":"silent"}`,
		FieldDataTypes: `{"price":"money"}`,
		Fields: []database.SiteField{
			{Name: "title", Selector: "h1", Type: "text"},
			{Name: "price", Selector: ".price", Type: "text"},
		},
	}
	engine, err := NewEngine(site)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	report, err := engine.ValidateExtraction(context.Background())
	if err != nil {
		t.Fatalf("validate extraction: %v", err)
	}
	if report.ExtractedItems != 1 || len(report.Samples) != 1 {
		t.Fatalf("unexpected validation report: %+v", report)
	}
	sample := report.Samples[0]
	if sample.Raw != "¥123.45" || sample.Normalized != "¥123.45" || sample.Currency != "CNY" {
		t.Fatalf("unexpected price sample: %+v", sample)
	}
}

func TestValidateExtractionSupportsPresenceLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><ul><li><a href="/a">公告 A</a></li><li><a href="/b">公告 B</a></li></ul></body></html>`))
	}))
	defer server.Close()

	site := &database.Site{
		Name: "presence-validation", URL: server.URL, Container: "ul", Item: "li", StrategyType: "presence",
		StrategyConfig: `{"type":"presence","identity":{"source":"source_url"},"on_first_baseline":"silent"}`,
		Fields: []database.SiteField{
			{Name: "title", Selector: "a", Type: "text"},
			{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
		},
	}
	engine, err := NewEngine(site)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	report, err := engine.ValidateExtraction(context.Background())
	if err != nil {
		t.Fatalf("validate presence extraction: %v", err)
	}
	if report.ExtractedItems != 2 || len(report.Samples) != 2 {
		t.Fatalf("unexpected presence validation report: %+v", report)
	}
	if report.Samples[0].ItemKey == report.Samples[1].ItemKey {
		t.Fatalf("presence list samples must have stable unique keys: %+v", report.Samples)
	}
}

func TestPersistEvaluationWithoutAccountsIsSkipped(t *testing.T) {
	setupMonitorPersistenceDB(t)
	site := createPriceMonitorSite(t)
	event := ChangeEvent{
		EventType: "price_dropped", ItemKey: "SKU-1", Title: "商品",
		URL: site.URL, Before: map[string]interface{}{"price": "CNY100.00"}, After: map[string]interface{}{"price": "CNY90.00"},
		OldValue: "¥100.00", NewValue: "¥90.00", ChangeAmount: 1000, Currency: "CNY", OccurredAt: time.Now(),
	}
	event.DedupeKey = GenerateDedupeKey(site.ID, site.ConfigVersion, event.EventType, event.ItemKey, ComputeFingerprint(event.Before), ComputeFingerprint(event.After))
	result := EvaluationResult{
		NextSnapshots: []Snapshot{{ItemKey: "SKU-1", Payload: event.After, Fingerprint: ComputeFingerprint(event.After), PriceMinor: 9000, PriceValid: true, Currency: "CNY"}},
		Events:        []ChangeEvent{event},
	}
	if err := PersistEvaluation(site, false, result, nil); err != nil {
		t.Fatalf("persist evaluation: %v", err)
	}
	var stored database.MonitorEvent
	if err := database.GetDB().Where("site_id = ?", site.ID).First(&stored).Error; err != nil {
		t.Fatalf("load event: %v", err)
	}
	if stored.DeliveryStatus != "skipped" {
		t.Errorf("event without accounts should be skipped, got %s", stored.DeliveryStatus)
	}
	var deliveries, legacyRecords int64
	database.GetDB().Model(&database.NotificationDelivery{}).Where("event_id = ?", stored.ID).Count(&deliveries)
	database.GetDB().Model(&database.UpdateRecord{}).Where("site_id = ?", site.ID).Count(&legacyRecords)
	if deliveries != 0 || legacyRecords != 0 {
		t.Errorf("unexpected compatibility rows: deliveries=%d update_records=%d", deliveries, legacyRecords)
	}
}

func TestPersistEvaluationRejectsStaleDefinition(t *testing.T) {
	setupMonitorPersistenceDB(t)
	site := createPriceMonitorSite(t)
	if err := database.GetDB().Model(&database.Site{}).Where("id = ?", site.ID).Update("config_version", 2).Error; err != nil {
		t.Fatalf("update version: %v", err)
	}
	err := PersistEvaluation(site, false, EvaluationResult{NextSnapshots: []Snapshot{{ItemKey: "SKU-1", Payload: map[string]interface{}{"price": "CNY90.00"}}}}, nil)
	if !errors.Is(err, ErrStaleDefinition) {
		t.Fatalf("expected ErrStaleDefinition, got %v", err)
	}
	var snapshots int64
	database.GetDB().Model(&database.MonitorSnapshot{}).Where("site_id = ?", site.ID).Count(&snapshots)
	if snapshots != 0 {
		t.Errorf("stale evaluation must not write snapshots, got %d", snapshots)
	}
}

func TestSnapshotUpsertMovesToCurrentDefinitionVersion(t *testing.T) {
	setupMonitorPersistenceDB(t)
	site := createPriceMonitorSite(t)
	first := EvaluationResult{NextSnapshots: []Snapshot{{ItemKey: "SKU-1", Payload: map[string]interface{}{"price": "CNY100.00"}, Fingerprint: "v1", PriceMinor: 10000, PriceValid: true, Currency: "CNY"}}}
	if err := PersistEvaluation(site, true, first, nil); err != nil {
		t.Fatalf("persist v1: %v", err)
	}
	if err := database.GetDB().Model(&database.Site{}).Where("id = ?", site.ID).Update("config_version", 2).Error; err != nil {
		t.Fatalf("update version: %v", err)
	}
	site.ConfigVersion = 2
	second := EvaluationResult{NextSnapshots: []Snapshot{{ItemKey: "SKU-1", Payload: map[string]interface{}{"price": "CNY90.00"}, Fingerprint: "v2", PriceMinor: 9000, PriceValid: true, Currency: "CNY"}}}
	if err := PersistEvaluation(site, false, second, nil); err != nil {
		t.Fatalf("persist v2: %v", err)
	}
	v1, err := LoadSnapshots(site.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := LoadSnapshots(site.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(v1) != 0 || len(v2) != 1 || v2["SKU-1"].Fingerprint != "v2" {
		t.Errorf("unexpected versioned snapshots: v1=%v v2=%v", v1, v2)
	}
}

func TestAggregateEventDeliveryStatuses(t *testing.T) {
	setupMonitorPersistenceDB(t)
	site := createPriceMonitorSite(t)
	tests := []struct {
		name     string
		statuses []string
		want     string
	}{
		{"no targets", nil, "skipped"},
		{"all sent", []string{"sent", "sent"}, "delivered"},
		{"all skipped", []string{"skipped", "skipped"}, "skipped"},
		{"sent and dead", []string{"sent", "dead"}, "partial"},
		{"sent and skipped", []string{"sent", "skipped"}, "partial"},
		{"dead and skipped", []string{"dead", "skipped"}, "failed"},
		{"still retrying", []string{"sent", "failed"}, "pending"},
	}
	for i, tt := range tests {
		event := database.MonitorEvent{
			SiteID: site.ID, EventType: "price_dropped", ItemKey: tt.name,
			DedupeKey:  GenerateDedupeKey(site.ID, site.ConfigVersion, "price_dropped", tt.name, "before", "after"),
			OccurredAt: time.Now(), DeliveryStatus: "pending",
		}
		if err := database.GetDB().Create(&event).Error; err != nil {
			t.Fatalf("create event %d: %v", i, err)
		}
		for accountIndex, status := range tt.statuses {
			delivery := database.NotificationDelivery{EventID: event.ID, SiteID: site.ID, AccountID: uint(accountIndex + 1), Status: status}
			if err := database.GetDB().Create(&delivery).Error; err != nil {
				t.Fatalf("create delivery: %v", err)
			}
		}
		if err := aggregateEventStatus(event.ID); err != nil {
			t.Fatalf("aggregate %s: %v", tt.name, err)
		}
		if err := database.GetDB().First(&event, event.ID).Error; err != nil {
			t.Fatal(err)
		}
		if event.DeliveryStatus != tt.want {
			t.Errorf("%s status = %s, want %s", tt.name, event.DeliveryStatus, tt.want)
		}
	}
}

func TestCheckNowSerializesConcurrentChecks(t *testing.T) {
	setupMonitorPersistenceDB(t)
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
		_, _ = w.Write([]byte(`<html><body><h1>商品</h1><span class="price">¥100.00</span></body></html>`))
	}))
	defer server.Close()

	site := &database.Site{
		Name: "serialized-monitor", URL: server.URL, Container: "body", StrategyType: "field_transition",
		StrategyConfig: `{"type":"field_transition","identity":{"source":"source_url"},"conditions":[{"field":"price","value_type":"money","operator":"decreased"}],"on_first_baseline":"silent"}`,
		FieldDataTypes: `{"price":"money"}`, ConfigVersion: 1,
		Fields: []database.SiteField{
			{Name: "title", Selector: "h1", Type: "text"},
			{Name: "price", Selector: ".price", Type: "text"},
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
