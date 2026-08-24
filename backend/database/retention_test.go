package database

import (
	"fmt"
	"testing"
)

func TestInitEnablesWAL(t *testing.T) {
	setupTestDB(t)

	var mode string
	if err := DB.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
		t.Fatalf("查询 journal_mode 失败: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("期望 journal_mode 为 wal，得到 %s", mode)
	}
}

func TestRunRetentionUpdateRecords(t *testing.T) {
	setupTestDB(t)

	site := &Site{Name: "保留测试", URL: "https://example.com", Container: "div", Item: "a"}
	if err := DB.Create(site).Error; err != nil {
		t.Fatalf("创建站点失败: %v", err)
	}
	other := &Site{Name: "保留测试2", URL: "https://example.org", Container: "div", Item: "a"}
	if err := DB.Create(other).Error; err != nil {
		t.Fatalf("创建站点失败: %v", err)
	}

	// site 插入超量记录，other 插入少量记录（不应被删）
	total := RetentionUpdateRecordsPerSite + 50
	records := make([]UpdateRecord, 0, total)
	for i := 0; i < total; i++ {
		records = append(records, UpdateRecord{SiteID: site.ID, Title: fmt.Sprintf("t%d", i)})
	}
	if err := DB.CreateInBatches(records, 200).Error; err != nil {
		t.Fatalf("插入变更记录失败: %v", err)
	}
	if err := DB.Create(&UpdateRecord{SiteID: other.ID, Title: "keep"}).Error; err != nil {
		t.Fatalf("插入变更记录失败: %v", err)
	}

	if err := RunRetention(); err != nil {
		t.Fatalf("RunRetention 失败: %v", err)
	}

	var count int64
	DB.Model(&UpdateRecord{}).Where("site_id = ?", site.ID).Count(&count)
	if count != int64(RetentionUpdateRecordsPerSite) {
		t.Fatalf("期望保留 %d 条，得到 %d", RetentionUpdateRecordsPerSite, count)
	}
	// 保留的应是最新的（标题为最大序号段）
	var newest UpdateRecord
	DB.Where("site_id = ?", site.ID).Order("id desc").First(&newest)
	if newest.Title != fmt.Sprintf("t%d", total-1) {
		t.Errorf("最新记录应保留，得到 %s", newest.Title)
	}
	var oldest UpdateRecord
	DB.Where("site_id = ?", site.ID).Order("id asc").First(&oldest)
	if oldest.Title != fmt.Sprintf("t%d", total-RetentionUpdateRecordsPerSite) {
		t.Errorf("最旧保留记录不符，得到 %s", oldest.Title)
	}
	DB.Model(&UpdateRecord{}).Where("site_id = ?", other.ID).Count(&count)
	if count != 1 {
		t.Fatalf("其他站点记录不应被删除，得到 %d", count)
	}
}
