package monitor

import (
	"testing"
)

func TestMatchKeywords_EmptyKeywords(t *testing.T) {
	item := ExtractResult{"title": "2025年面试公告"}
	if !matchKeywords(item, nil) {
		t.Error("nil keywords should match everything")
	}
	if !matchKeywords(item, []string{}) {
		t.Error("empty keywords should match everything")
	}
}

func TestMatchKeywords_ExactHit(t *testing.T) {
	item := ExtractResult{
		"title": "2025年面试公告",
		"url":   "https://example.com/msgg/123",
	}
	if !matchKeywords(item, []string{"面试"}) {
		t.Error("should match keyword 面试 in title")
	}
}

func TestMatchKeywords_Miss(t *testing.T) {
	item := ExtractResult{
		"title": "2025年体检公告",
		"url":   "https://example.com/tjgg/456",
	}
	if matchKeywords(item, []string{"面试", "录用"}) {
		t.Error("should not match when no keyword appears in any field")
	}
}

func TestMatchKeywords_CaseInsensitive(t *testing.T) {
	item := ExtractResult{
		"title": "2025年Interview Announcement",
	}
	if !matchKeywords(item, []string{"interview"}) {
		t.Error("should be case-insensitive (lowercase keyword)")
	}
	if !matchKeywords(item, []string{"INTERVIEW"}) {
		t.Error("should be case-insensitive (uppercase keyword)")
	}
	if !matchKeywords(item, []string{"InteRview"}) {
		t.Error("should be case-insensitive (mixed case keyword)")
	}
}

func TestMatchKeywords_MatchInURL(t *testing.T) {
	item := ExtractResult{
		"title": "2025年招录公告",
		"url":   "https://example.com/luyong/789",
	}
	if !matchKeywords(item, []string{"luyong"}) {
		t.Error("should match keyword in url field")
	}
}

func TestMatchKeywords_MultipleKeywordsAnyHit(t *testing.T) {
	item := ExtractResult{
		"title": "2025年公示",
	}
	if !matchKeywords(item, []string{"面试", "录用", "公示"}) {
		t.Error("should match when at least one keyword hits (公示)")
	}
	if matchKeywords(item, []string{"面试", "录用"}) {
		t.Error("should not match when no keyword hits")
	}
}

func TestMatchKeywords_EmptyStringKeywordSkipped(t *testing.T) {
	item := ExtractResult{
		"title": "2025年面试公告",
	}
	if !matchKeywords(item, []string{"", "面试", ""}) {
		t.Error("empty string keywords should be skipped, still match 面试")
	}
}

func TestMatchKeywords_NonStringValue(t *testing.T) {
	item := ExtractResult{
		"title": "2025年面试公告",
		"count": 42, // non-string value
	}
	if !matchKeywords(item, []string{"面试"}) {
		t.Error("non-string values should be skipped gracefully, still match 面试 in title")
	}
}

// ---- filterByKeywords ----

func TestFilterByKeywords_EmptyKeywordsReturnsAll(t *testing.T) {
	items := []ExtractResult{
		{"title": "面试公告"},
		{"title": "体检公告"},
	}
	result := filterByKeywords(items, "")
	if len(result) != 2 {
		t.Errorf("empty keywords should return all items, got %d", len(result))
	}
}

func TestFilterByKeywords_FiltersCorrectly(t *testing.T) {
	items := []ExtractResult{
		{"title": "2025年面试公告"},
		{"title": "2025年体检公告"},
		{"title": "2025年录用公示"},
	}
	result := filterByKeywords(items, "面试,录用")
	if len(result) != 2 {
		t.Fatalf("expected 2 matched items, got %d", len(result))
	}
	if result[0]["title"] != "2025年面试公告" {
		t.Errorf("first matched item should be 面试公告, got %v", result[0]["title"])
	}
	if result[1]["title"] != "2025年录用公示" {
		t.Errorf("second matched item should be 录用公示, got %v", result[1]["title"])
	}
}

func TestFilterByKeywords_NoMatches(t *testing.T) {
	items := []ExtractResult{
		{"title": "2025年体检公告"},
		{"title": "2025年考察通知"},
	}
	result := filterByKeywords(items, "面试,录用")
	if len(result) != 0 {
		t.Errorf("expected no matches, got %d", len(result))
	}
}

func TestFilterByKeywords_MixedCase(t *testing.T) {
	items := []ExtractResult{
		{"title": "2025 Announcement of Interview"},
		{"title": "2025体检公告"},
	}
	result := filterByKeywords(items, "interview")
	if len(result) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result))
	}
	if result[0]["title"] != "2025 Announcement of Interview" {
		t.Errorf("expected the Interview item, got %v", result[0]["title"])
	}
}
