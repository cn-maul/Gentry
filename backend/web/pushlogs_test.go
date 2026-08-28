package web

import (
	"testing"

	"github.com/cn-maul/Gentry/database"
)

func TestParseStringList(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: []string{}},
		{name: "null", raw: "null", want: []string{}},
		{name: "valid", raw: `["a","b"]`, want: []string{"a", "b"}},
		{name: "invalid json", raw: "not-json", want: []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseStringList(c.raw)
			if len(got) != len(c.want) {
				t.Fatalf("parseStringList(%q) = %v, want %v", c.raw, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("parseStringList(%q) = %v, want %v", c.raw, got, c.want)
				}
			}
		})
	}
}

func TestPushLogFromModel(t *testing.T) {
	log := database.PushLog{
		ID:           7,
		SiteID:       2,
		SiteName:     "测试站点",
		Status:       "failed",
		Reason:       "",
		AccountNames: `["pushplus"]`,
		ItemCount:    1,
		Titles:       `["公告 A"]`,
		Detail:       "「pushplus」(pushplus): 请求失败",
		RecordIDs:    "[1,2]",
	}
	resp := pushLogFromModel(log)
	if resp.ID != 7 || resp.SiteID != 2 || resp.SiteName != "测试站点" {
		t.Fatalf("basic fields not mapped: %+v", resp)
	}
	if resp.Status != "failed" || resp.Detail == "" {
		t.Fatalf("status/detail not mapped: %+v", resp)
	}
	if len(resp.AccountNames) != 1 || resp.AccountNames[0] != "pushplus" {
		t.Fatalf("account_names not parsed: %v", resp.AccountNames)
	}
	if len(resp.Titles) != 1 || resp.Titles[0] != "公告 A" {
		t.Fatalf("titles not parsed: %v", resp.Titles)
	}
	if resp.ItemCount != 1 {
		t.Fatalf("item_count = %d, want 1", resp.ItemCount)
	}
}
