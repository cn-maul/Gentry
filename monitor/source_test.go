package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/cn-maul/Gentry/database"
)

func TestExtractConfiguredJSONSourceFiltersAndProjectsSKUFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://shop.example/products/1" || r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			http.Error(w, "missing source headers", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":[
			{"products_no":"SKU-1","spec_array":[{"value":"个人版"},{"value":"半年版"},{"value":"3 台"}],"is_selling":true,"sell_price":"30.00"},
			{"products_no":"SKU-2","spec_array":[{"value":"家庭版"}],"is_selling":false,"sell_price":"999.00"}
		]}`))
	}))
	defer server.Close()

	site := &database.Site{
		URL:       "https://shop.example/products/1",
		Container: "data",
		Item:      "*",
		FetchConfig: `{"mode":"api_json","url":"` + server.URL +
			`","items_path":"data","filter_path":"is_selling","filter_equals":"true",` +
			`"headers":{"Referer":"https://shop.example/products/1","X-Requested-With":"XMLHttpRequest"}}`,
		Fields: []database.SiteField{
			{Name: "title", Selector: "spec_array.*.value", Type: "text"},
			{Name: "sku", Selector: "products_no", Type: "text"},
			{Name: "price", Selector: "sell_price", Type: "text"},
		},
	}
	items, err := ExtractConfiguredSource(context.Background(), site)
	if err != nil {
		t.Fatalf("ExtractConfiguredSource failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d, want 1: %+v", len(items), items)
	}
	if items[0]["sku"] != "SKU-1" || items[0]["price"] != "30.00" {
		t.Fatalf("unexpected projected fields: %+v", items[0])
	}
	if items[0]["title"] != "个人版 / 半年版 / 3 台" {
		t.Fatalf("title = %q", items[0]["title"])
	}
}

func TestCanonicalFetchConfigKeepsLegacyHTMLEmpty(t *testing.T) {
	canonical, err := CanonicalFetchConfig("", "https://example.com/product")
	if err != nil {
		t.Fatalf("CanonicalFetchConfig failed: %v", err)
	}
	if canonical != "" {
		t.Fatalf("legacy HTML config = %q, want empty", canonical)
	}
}

func TestDynamicFetchVariablesReuseOneRuleAcrossProductPages(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products/adguard":
			_, _ = w.Write([]byte(`<input id="goods_id" type="hidden" value="31">`))
		case "/products/reqable":
			_, _ = w.Write([]byte(`<input id="goods_id" type="hidden" value="893">`))
		case "/site/goods_skus":
			id := r.URL.Query().Get("goods_id")
			expectedPath := map[string]string{"31": "/products/adguard", "893": "/products/reqable"}[id]
			if r.Header.Get("Referer") != server.URL+expectedPath {
				http.Error(w, "invalid referer", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"sku":"SKU-` + id + `","name":"Product ` + id + `","price":"` + id + `.00"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	config := `{"mode":"api_json","url":"` + server.URL + `/site/goods_skus?goods_id={{goods_id}}",` +
		`"items_path":"data","headers":{"Referer":"{{page_url}}"},` +
		`"variables":{"goods_id":{"source":"html","selector":"#goods_id","attr":"value"}}}`
	for _, product := range []struct {
		path string
		id   string
	}{{"/products/adguard", "31"}, {"/products/reqable", "893"}} {
		site := &database.Site{
			URL: server.URL + product.path, Container: "data", Item: "*", FetchConfig: config,
			Fields: []database.SiteField{
				{Name: "title", Selector: "name", Type: "text"},
				{Name: "sku", Selector: "sku", Type: "text"},
				{Name: "price", Selector: "price", Type: "text"},
			},
		}
		items, err := ExtractConfiguredSource(context.Background(), site)
		if err != nil {
			t.Fatalf("extract %s: %v", product.path, err)
		}
		if len(items) != 1 || items[0]["sku"] != "SKU-"+product.id {
			t.Fatalf("unexpected %s items: %+v", product.path, items)
		}
	}
}

func TestDynamicFetchVariableRequiresMatchingElement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>no product id</body></html>`))
	}))
	defer server.Close()

	site := &database.Site{
		URL: server.URL,
		FetchConfig: `{"mode":"api_json","url":"https://shop.example/api?id={{goods_id}}",` +
			`"items_path":"data","variables":{"goods_id":{"selector":"#goods_id","attr":"value"}}}`,
	}
	if _, err := ExtractConfiguredSource(context.Background(), site); err == nil {
		t.Fatal("expected missing dynamic parameter element to fail")
	}
}

func TestFetchConfigRejectsDynamicHostAndUnknownVariable(t *testing.T) {
	for _, raw := range []string{
		`{"mode":"api_json","url":"https://{{host}}/api","items_path":"data","variables":{"host":{"selector":"#host"}}}`,
		`{"mode":"api_json","url":"https://shop.example/api?id={{missing}}","items_path":"data"}`,
	} {
		if _, err := ParseFetchConfig(raw, "https://shop.example/product"); err == nil {
			t.Fatalf("expected invalid fetch template to fail: %s", raw)
		}
	}
}

func TestSmartScanUsesSavedJSONRuleWithoutFetchingProductPage(t *testing.T) {
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

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"sku":"SKU-1","name":"Personal / 6 months / 3 devices","selling":true,"price":"30.00"}]}`))
	}))
	defer api.Close()

	pageURL := "https://unreachable.example/products/security-suite"
	rule := &database.ScanRuleTemplate{
		Name:        "shop-skus",
		Container:   "data",
		Item:        "*",
		Priority:    90,
		Enabled:     true,
		FetchConfig: `{"mode":"api_json","url":"` + api.URL + `","items_path":"data","filter_path":"selling","filter_equals":"true"}`,
		Fields: []database.ScanRuleField{
			{Name: "title", Selector: "name", Type: "text"},
			{Name: "sku", Selector: "sku", Type: "text"},
			{Name: "price", Selector: "price", Type: "text"},
		},
	}
	if err := ApplyExactScanRuleScope(rule, pageURL); err != nil {
		t.Fatalf("apply rule scope: %v", err)
	}
	if err := database.CreateScanRuleTemplate(rule); err != nil {
		t.Fatalf("create scan rule: %v", err)
	}

	result, err := SmartScan(&ScanSettings{URL: pageURL, StrategyType: "field_transition"})
	if err != nil {
		t.Fatalf("SmartScan failed: %v", err)
	}
	if len(result.Containers) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(result.Containers))
	}
	candidate := result.Containers[0]
	if candidate.Strategy != "template_shop-skus" {
		t.Fatalf("strategy = %q, want saved template", candidate.Strategy)
	}
	if candidate.Config.Fetch == nil || candidate.Config.Fetch.URL != api.URL {
		t.Fatalf("candidate did not preserve API rule: %+v", candidate.Config.Fetch)
	}
	if candidate.ItemCount != 1 || candidate.SampleItems[0]["price"] != "30.00" {
		t.Fatalf("unexpected API candidate: %+v", candidate)
	}
}
