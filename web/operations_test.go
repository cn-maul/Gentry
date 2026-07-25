package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/monitor"
)

func TestDBQuickScanRuleDerivesRouteScope(t *testing.T) {
	req := quickScanRuleRequest{
		Name:      "招聘公告路由",
		URL:       "https://example.com/jobs/list/?b=2&a=1",
		ScopeType: monitor.ScanRuleScopeRoute,
		Config: monitor.ScanMonitorConfig{
			Container: ".notice-list",
			Item:      "li",
			Fields: []monitor.ScanFieldConfig{
				{Name: "title", Selector: "a", Type: "text"},
				{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
			},
		},
	}
	rule, err := dbQuickScanRuleFromRequest(&req)
	if err != nil {
		t.Fatalf("dbQuickScanRuleFromRequest failed: %v", err)
	}
	if rule.ScopeType != monitor.ScanRuleScopeRoute || rule.MatchHost != "example.com" || rule.MatchPath != "/jobs/list" || rule.MatchQuery != "a=1&b=2" {
		t.Fatalf("unexpected derived scope: %+v", rule)
	}
	if rule.URLContains != "" {
		t.Fatalf("new scoped rules must not use legacy URLContains: %q", rule.URLContains)
	}
}

func TestDBQuickScanRuleRejectsConfigWithoutTitle(t *testing.T) {
	req := quickScanRuleRequest{
		Name: "invalid",
		URL:  "https://example.com/list",
		Config: monitor.ScanMonitorConfig{
			Container: ".list",
			Item:      "a",
			Fields:    []monitor.ScanFieldConfig{{Name: "url", Type: "attr", Attr: "href"}},
		},
	}
	if _, err := dbQuickScanRuleFromRequest(&req); err == nil {
		t.Fatal("expected missing title field to be rejected")
	}
}

func TestDBQuickScanRulePreservesJSONAPISource(t *testing.T) {
	fetchConfig := &monitor.FetchConfig{
		Mode: monitor.FetchModeAPIJSON, URL: "https://shop.example/api/skus?id=31",
		ItemsPath: "data", FilterPath: "is_selling", FilterEquals: "true",
		Headers: map[string]string{"Referer": "https://shop.example/products/31"},
	}
	req := quickScanRuleRequest{
		Name: "SKU 价格", URL: "https://shop.example/products/31", ScopeType: monitor.ScanRuleScopeExact,
		Config: monitor.ScanMonitorConfig{
			Container: "data", Item: "*", Fetch: fetchConfig,
			Fields: []monitor.ScanFieldConfig{
				{Name: "title", Selector: "name", Type: "text"},
				{Name: "sku", Selector: "products_no", Type: "text"},
				{Name: "price", Selector: "sell_price", Type: "text"},
			},
		},
	}
	rule, err := dbQuickScanRuleFromRequest(&req)
	if err != nil {
		t.Fatalf("dbQuickScanRuleFromRequest failed: %v", err)
	}
	parsed, err := monitor.ParseFetchConfig(rule.FetchConfig, req.URL)
	if err != nil {
		t.Fatalf("saved fetch config invalid: %v", err)
	}
	if parsed.Mode != monitor.FetchModeAPIJSON || parsed.ItemsPath != "data" || parsed.Headers["Referer"] == "" {
		t.Fatalf("unexpected fetch config: %+v", parsed)
	}
}

func TestDBQuickScanRulePreservesDynamicVariablesAndSectionScope(t *testing.T) {
	fetchConfig := &monitor.FetchConfig{
		Mode: monitor.FetchModeAPIJSON, URL: "https://shop.example/api/skus?id={{goods_id}}",
		ItemsPath: "data", Headers: map[string]string{"Referer": "{{page_url}}"},
		Variables: map[string]monitor.FetchVariable{
			"goods_id": {Source: "html", Selector: "#goods_id", Attr: "value"},
		},
	}
	req := quickScanRuleRequest{
		Name: "商城 SKU", URL: "https://shop.example/products/adguard", ScopeType: monitor.ScanRuleScopeSection,
		Config: monitor.ScanMonitorConfig{
			Container: "data", Item: "*", Fetch: fetchConfig,
			Fields: []monitor.ScanFieldConfig{
				{Name: "title", Selector: "name", Type: "text"},
				{Name: "price", Selector: "price", Type: "text"},
			},
		},
	}
	rule, err := dbQuickScanRuleFromRequest(&req)
	if err != nil {
		t.Fatalf("dbQuickScanRuleFromRequest failed: %v", err)
	}
	if rule.ScopeType != monitor.ScanRuleScopeSection || rule.MatchPath != "/products" {
		t.Fatalf("unexpected section scope: %+v", rule)
	}
	parsed, err := monitor.ParseFetchConfig(rule.FetchConfig, req.URL)
	if err != nil {
		t.Fatalf("saved fetch config invalid: %v", err)
	}
	if parsed.Variables["goods_id"].Selector != "#goods_id" || parsed.Headers["Referer"] != "{{page_url}}" {
		t.Fatalf("dynamic fetch config was not preserved: %+v", parsed)
	}
}

func TestMergeMaskedSensitiveConfigPreservesStoredSecret(t *testing.T) {
	existing := `{"token":"abcdef123456"}`
	merged, err := mergeMaskedSensitiveConfig("pushplus", map[string]interface{}{
		"token":   "abc****456",
		"channel": "mail",
	}, existing)
	if err != nil {
		t.Fatalf("mergeMaskedSensitiveConfig failed: %v", err)
	}
	if merged["token"] != "abcdef123456" {
		t.Fatalf("stored token was not preserved: %v", merged["token"])
	}
}

func TestMonitorSnapshotResponseIncludesFormattedPrice(t *testing.T) {
	payload, err := json.Marshal(monitorSnapshotResponse{
		MonitorSnapshot: database.MonitorSnapshot{ItemKey: "sku-1", PriceMinor: 12345, PriceValid: true, Currency: "CNY"},
		PriceDisplay:    "¥123.45",
	})
	if err != nil {
		t.Fatalf("marshal snapshot response: %v", err)
	}
	encoded := string(payload)
	if !strings.Contains(encoded, `"price_minor":12345`) || !strings.Contains(encoded, `"price_display":"¥123.45"`) {
		t.Fatalf("snapshot response is missing price fields: %s", encoded)
	}
}

func TestDetectionFingerprintCoversFullFieldSemantics(t *testing.T) {
	fields := []fieldRequest{{Name: "price", Selector: ".price", Type: "text", Attr: "", Transform: "trim"}}
	base := computeDetectionFingerprint(
		"https://example.com", "body", "", fields, "field_transition",
		`{"type":"field_transition","identity":{"source":"source_url"},"conditions":[{"field":"price","value_type":"money","operator":"decreased"}],"on_first_baseline":"silent"}`,
		`{"price":"money"}`,
	)
	changedAttr := append([]fieldRequest(nil), fields...)
	changedAttr[0].Attr = "data-price"
	if base == computeDetectionFingerprint("https://example.com", "body", "", changedAttr, "field_transition", `{"type":"field_transition","identity":{"source":"source_url"},"conditions":[{"field":"price","value_type":"money","operator":"decreased"}],"on_first_baseline":"silent"}`, `{"price":"money"}`) {
		t.Error("changing Attr must change the detection fingerprint")
	}
	changedTransform := append([]fieldRequest(nil), fields...)
	changedTransform[0].Transform = "lower"
	if base == computeDetectionFingerprint("https://example.com", "body", "", changedTransform, "field_transition", `{"type":"field_transition","identity":{"source":"source_url"},"conditions":[{"field":"price","value_type":"money","operator":"decreased"}],"on_first_baseline":"silent"}`, `{"price":"money"}`) {
		t.Error("changing Transform must change the detection fingerprint")
	}
}

func TestDetectionFingerprintCanonicalizesOrder(t *testing.T) {
	fieldsA := []fieldRequest{
		{Name: "title", Selector: "h1", Type: "text"},
		{Name: "price", Selector: ".price", Type: "text"},
	}
	fieldsB := []fieldRequest{fieldsA[1], fieldsA[0]}
	configA := `{"type":"field_transition","identity":{"field":"title"},"conditions":[{"field":"price","value_type":"money","operator":"decreased"}],"on_first_baseline":"silent"}`
	configB := `{"on_first_baseline":"silent","conditions":[{"operator":"decreased","value_type":"money","field":"price"}],"identity":{"field":"title"},"type":"field_transition"}`
	fpA := computeDetectionFingerprint("https://example.com", "body", "", fieldsA, "field_transition", configA, `{"title":"text","price":"money"}`)
	fpB := computeDetectionFingerprint("https://example.com", "body", "", fieldsB, "field_transition", configB, `{"price":"money","title":"text"}`)
	if fpA != fpB {
		t.Error("field order and JSON key order should not change the detection fingerprint")
	}
}

func TestDetectionFingerprintIncludesFetchConfig(t *testing.T) {
	fields := []fieldRequest{{Name: "price", Selector: "sell_price", Type: "text"}}
	base := computeDetectionFingerprintWithFetch("https://shop.example/product", "data", "*", fields, "field_transition", "{}", `{"price":"money"}`, `{"mode":"api_json","url":"https://shop.example/api?id=1","items_path":"data"}`)
	changed := computeDetectionFingerprintWithFetch("https://shop.example/product", "data", "*", fields, "field_transition", "{}", `{"price":"money"}`, `{"mode":"api_json","url":"https://shop.example/api?id=2","items_path":"data"}`)
	if base == changed {
		t.Fatal("changing the API source must change the detection fingerprint")
	}
}
