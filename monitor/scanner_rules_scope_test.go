package monitor

import (
	"testing"

	"github.com/cn-maul/Gentry/database"
)

func TestExactScanRuleScopeIsolatesPathHostAndQuery(t *testing.T) {
	rule := database.ScanRuleTemplate{}
	if err := ApplyScanRuleScope(&rule, "https://Example.com/notices/?b=2&a=1#top", ScanRuleScopeExact); err != nil {
		t.Fatalf("ApplyScanRuleScope failed: %v", err)
	}

	matches := []string{
		"http://example.com/notices?a=1&b=2",
		"https://EXAMPLE.COM/notices/?b=2&a=1",
	}
	for _, rawURL := range matches {
		if !ScanRuleMatchesURL(rule, rawURL) {
			t.Errorf("expected exact scope to match %s", rawURL)
		}
	}

	nonMatches := []string{
		"https://evil-example.com/notices?a=1&b=2",
		"https://example.com/other?a=1&b=2",
		"https://example.com/notices?a=1&b=3",
	}
	for _, rawURL := range nonMatches {
		if ScanRuleMatchesURL(rule, rawURL) {
			t.Errorf("expected exact scope not to match %s", rawURL)
		}
	}
}

func TestRouteScanRuleScopeUsesPathSegmentBoundary(t *testing.T) {
	rule := database.ScanRuleTemplate{}
	if err := ApplyScanRuleScope(&rule, "https://example.com/system/notices/", ScanRuleScopeRoute); err != nil {
		t.Fatalf("ApplyScanRuleScope failed: %v", err)
	}
	if !ScanRuleMatchesURL(rule, "https://example.com/system/notices/page/2") {
		t.Fatal("route scope should match a child route")
	}
	if ScanRuleMatchesURL(rule, "https://example.com/system/notices-old") {
		t.Fatal("route scope must respect path segment boundaries")
	}
	if ScanRuleMatchesURL(rule, "https://example.com/other/notices") {
		t.Fatal("route scope must isolate a different route")
	}
}

func TestQueryDrivenRouteKeepsQueryConstraint(t *testing.T) {
	rule := database.ScanRuleTemplate{}
	if err := ApplyScanRuleScope(&rule, "https://example.com/?a=dir&c=181790&f=203304", ScanRuleScopeRoute); err != nil {
		t.Fatalf("ApplyScanRuleScope failed: %v", err)
	}
	if !ScanRuleMatchesURL(rule, "https://example.com/?f=203304&a=dir&c=181790") {
		t.Fatal("canonical query order should match")
	}
	if ScanRuleMatchesURL(rule, "https://example.com/?a=dir&c=181790&f=999") {
		t.Fatal("different query route must not match")
	}
	if ScanRuleMatchesURL(rule, "https://example.com/other?a=dir&c=181790&f=203304") {
		t.Fatal("root query route must not match another path")
	}
}

func TestRouteScopeRejectsUnqualifiedRoot(t *testing.T) {
	rule := database.ScanRuleTemplate{}
	if err := ApplyScanRuleScope(&rule, "https://example.com/", ScanRuleScopeRoute); err == nil {
		t.Fatal("expected root-only route scope to be rejected")
	}
}

func TestSectionScopeMatchesSiblingPagesOnSameHost(t *testing.T) {
	rule := database.ScanRuleTemplate{}
	if err := ApplyScanRuleScope(&rule, "https://lizhi.shop/products/adguard", ScanRuleScopeSection); err != nil {
		t.Fatalf("ApplyScanRuleScope failed: %v", err)
	}
	if rule.MatchPath != "/products" {
		t.Fatalf("section match path = %q, want /products", rule.MatchPath)
	}
	for _, rawURL := range []string{
		"https://lizhi.shop/products/adguard",
		"https://lizhi.shop/products/reqable",
	} {
		if !ScanRuleMatchesURL(rule, rawURL) {
			t.Errorf("section scope should match %s", rawURL)
		}
	}
	for _, rawURL := range []string{
		"https://lizhi.shop/product/reqable",
		"https://other.example/products/reqable",
	} {
		if ScanRuleMatchesURL(rule, rawURL) {
			t.Errorf("section scope should not match %s", rawURL)
		}
	}
}

func TestGlobalAndLegacyScanRuleScopesRemainSupported(t *testing.T) {
	global := database.ScanRuleTemplate{}
	if err := ApplyScanRuleScope(&global, "https://source.example/list", ScanRuleScopeGlobal); err != nil {
		t.Fatalf("ApplyScanRuleScope failed: %v", err)
	}
	if !ScanRuleMatchesURL(global, "https://another.example/news") {
		t.Fatal("global structural rule should be eligible on another website")
	}

	legacy := database.ScanRuleTemplate{URLContains: "example.com/news"}
	if !ScanRuleMatchesURL(legacy, "https://example.com/news/list") {
		t.Fatal("legacy URLContains rule should remain supported")
	}
}
