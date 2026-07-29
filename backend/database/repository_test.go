package database

import (
	"os"
	"testing"

	"gorm.io/gorm"
)

// setupTestDB 初始化内存 SQLite 用于测试
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir, err := os.MkdirTemp("", "alterbot-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	// 保存原 DB 并在 cleanup 恢复
	origDB := DB
	t.Cleanup(func() { DB = origDB })

	if err := Init(dir + "/test.db"); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	return DB
}

func TestCreateSiteWithFields(t *testing.T) {
	setupTestDB(t)

	site := &Site{
		Name:      "测试站点",
		URL:       "https://example.com",
		Container: "div.list",
		Item:      "a",
		Fields: []SiteField{
			{Name: "title", Selector: "a", Type: "text"},
			{Name: "url", Selector: "a", Type: "attr", Attr: "href"},
		},
	}

	if err := CreateSiteWithFields(site); err != nil {
		t.Fatalf("CreateSiteWithFields 失败: %v", err)
	}
	if site.ID == 0 {
		t.Fatal("创建后 site.ID 应为非零值")
	}

	// 验证级联写入
	var loaded Site
	if err := GetDB().Preload("Fields").First(&loaded, site.ID).Error; err != nil {
		t.Fatalf("读取站点失败: %v", err)
	}
	if len(loaded.Fields) != 2 {
		t.Fatalf("期望 2 个字段，得到 %d", len(loaded.Fields))
	}
	if loaded.Fields[0].Name != "title" {
		t.Errorf("期望字段名 title，得到 %s", loaded.Fields[0].Name)
	}
}

func TestResetMonitorBaseline(t *testing.T) {
	setupTestDB(t)
	site := &Site{Name: "baseline-test", URL: "https://example.com", Container: "body", BaselineStatus: "ready", ConfigVersion: 1}
	if err := CreateSiteWithFields(site); err != nil {
		t.Fatalf("create site: %v", err)
	}
	snapshot := MonitorSnapshot{SiteID: site.ID, ItemKey: "item", DefinitionVersion: 1}
	if err := GetDB().Create(&snapshot).Error; err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	version, err := ResetMonitorBaseline(site.ID)
	if err != nil {
		t.Fatalf("reset baseline: %v", err)
	}
	if version != 2 {
		t.Errorf("config version = %d, want 2", version)
	}
	var count int64
	GetDB().Model(&MonitorSnapshot{}).Where("site_id = ?", site.ID).Count(&count)
	if count != 0 {
		t.Errorf("snapshots should be deleted, got %d", count)
	}
	var stored Site
	if err := GetDB().First(&stored, site.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.BaselineStatus != "needs_baseline" {
		t.Errorf("baseline status = %s, want needs_baseline", stored.BaselineStatus)
	}
}

func TestCreateSiteWithFields_DuplicateName(t *testing.T) {
	setupTestDB(t)

	site1 := &Site{Name: "重复站点", URL: "https://example.com/1", Container: "div", Item: "a"}
	if err := CreateSiteWithFields(site1); err != nil {
		t.Fatalf("第一次创建失败: %v", err)
	}

	site2 := &Site{Name: "重复站点", URL: "https://example.com/2", Container: "div", Item: "a"}
	if err := CreateSiteWithFields(site2); err == nil {
		t.Fatal("期望重复名称创建失败，但成功了")
	}
}

func TestUpdateSiteWithFields(t *testing.T) {
	setupTestDB(t)

	site := &Site{
		Name:      "待更新站点",
		URL:       "https://example.com",
		Container: "div.list",
		Item:      "a",
		Fields: []SiteField{
			{Name: "title", Selector: "a", Type: "text"},
		},
	}
	if err := CreateSiteWithFields(site); err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 更新 URL 和字段
	site.URL = "https://example.com/updated"
	site.Fields = []SiteField{
		{Name: "title", Selector: "h2", Type: "text"},
		{Name: "date", Selector: "time", Type: "text"},
	}
	if err := UpdateSiteWithFields(site, site.Fields); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	var loaded Site
	if err := GetDB().Preload("Fields").First(&loaded, site.ID).Error; err != nil {
		t.Fatalf("读取更新后站点失败: %v", err)
	}
	if loaded.URL != "https://example.com/updated" {
		t.Errorf("URL 未更新，期望 %s，得到 %s", "https://example.com/updated", loaded.URL)
	}
	if len(loaded.Fields) != 2 {
		t.Fatalf("期望 2 个字段，得到 %d", len(loaded.Fields))
	}
}

func TestDeleteSiteCascade(t *testing.T) {
	setupTestDB(t)

	site := &Site{
		Name: "待删除站点",
		URL:  "https://example.com",
		Fields: []SiteField{
			{Name: "title", Selector: "a", Type: "text"},
		},
	}
	if err := CreateSiteWithFields(site); err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	// 添加一些更新记录
	records := []UpdateRecord{
		{SiteID: site.ID, Title: "更新1", URL: "/1"},
		{SiteID: site.ID, Title: "更新2", URL: "/2"},
	}
	for _, r := range records {
		if err := GetDB().Create(&r).Error; err != nil {
			t.Fatalf("创建更新记录失败: %v", err)
		}
	}

	if err := DeleteSiteCascade(site.ID); err != nil {
		t.Fatalf("DeleteSiteCascade 失败: %v", err)
	}

	// 验证站点已删除
	var count int64
	GetDB().Model(&Site{}).Where("id = ?", site.ID).Count(&count)
	if count != 0 {
		t.Error("站点未删除")
	}

	// 验证字段已级联删除
	GetDB().Model(&SiteField{}).Where("site_id = ?", site.ID).Count(&count)
	if count != 0 {
		t.Error("字段未级联删除")
	}

	// 验证更新记录已级联删除
	GetDB().Model(&UpdateRecord{}).Where("site_id = ?", site.ID).Count(&count)
	if count != 0 {
		t.Error("更新记录未级联删除")
	}
}

func TestCreateScanRuleTemplate(t *testing.T) {
	setupTestDB(t)

	rule := &ScanRuleTemplate{
		Name:        "测试规则",
		URLContains: "example.com",
		Container:   "div.list",
		Item:        "li",
		Priority:    50,
		Enabled:     true,
		Fields: []ScanRuleField{
			{Name: "title", Selector: "a", Type: "text"},
		},
	}

	if err := CreateScanRuleTemplate(rule); err != nil {
		t.Fatalf("CreateScanRuleTemplate 失败: %v", err)
	}
	if rule.ID == 0 {
		t.Fatal("创建后 rule.ID 应为非零值")
	}

	var loaded ScanRuleTemplate
	if err := GetDB().Preload("Fields").First(&loaded, rule.ID).Error; err != nil {
		t.Fatalf("读取规则失败: %v", err)
	}
	if len(loaded.Fields) != 1 {
		t.Fatalf("期望 1 个字段，得到 %d", len(loaded.Fields))
	}
}

func TestDeleteScanRuleTemplate(t *testing.T) {
	setupTestDB(t)

	rule := &ScanRuleTemplate{
		Name:        "待删除规则",
		URLContains: "example.com",
		Container:   "div",
		Item:        "a",
		Fields: []ScanRuleField{
			{Name: "title", Selector: "a", Type: "text"},
		},
	}
	if err := CreateScanRuleTemplate(rule); err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	if err := DeleteScanRuleTemplate(rule.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	var count int64
	GetDB().Model(&ScanRuleTemplate{}).Where("id = ?", rule.ID).Count(&count)
	if count != 0 {
		t.Error("规则未删除")
	}
	GetDB().Model(&ScanRuleField{}).Where("rule_id = ?", rule.ID).Count(&count)
	if count != 0 {
		t.Error("规则字段未级联删除")
	}
}
