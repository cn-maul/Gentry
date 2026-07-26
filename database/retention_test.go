package database

import (
	"fmt"
	"testing"
	"time"
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

func TestRunRetentionEventsAndDeliveries(t *testing.T) {
	setupTestDB(t)

	site := &Site{Name: "事件保留", URL: "https://example.com", Container: "div", Item: "a"}
	if err := DB.Create(site).Error; err != nil {
		t.Fatalf("创建站点失败: %v", err)
	}

	oldTime := time.Now().AddDate(0, 0, -(RetentionEventDays + 1))
	newTime := time.Now()

	oldEvent := &MonitorEvent{SiteID: site.ID, EventType: "item_added", DedupeKey: "old", CreatedAt: oldTime, OccurredAt: oldTime}
	newEvent := &MonitorEvent{SiteID: site.ID, EventType: "item_added", DedupeKey: "new", CreatedAt: newTime, OccurredAt: newTime}
	if err := DB.Create(oldEvent).Error; err != nil {
		t.Fatalf("创建事件失败: %v", err)
	}
	if err := DB.Create(newEvent).Error; err != nil {
		t.Fatalf("创建事件失败: %v", err)
	}

	// 属于 oldEvent 的投递将成为孤儿；newEvent 的 pending 投递必须保留；
	// 一条很旧的 sent 投递应被时间策略删除。
	orphanDelivery := &NotificationDelivery{EventID: oldEvent.ID, AccountID: 1, SiteID: site.ID, Status: "pending"}
	keepDelivery := &NotificationDelivery{EventID: newEvent.ID, AccountID: 1, SiteID: site.ID, Status: "pending"}
	staleSent := &NotificationDelivery{EventID: newEvent.ID, AccountID: 2, SiteID: site.ID, Status: "sent"}
	for _, d := range []*NotificationDelivery{orphanDelivery, keepDelivery, staleSent} {
		if err := DB.Create(d).Error; err != nil {
			t.Fatalf("创建投递失败: %v", err)
		}
	}
	staleCutoff := time.Now().AddDate(0, 0, -(RetentionDeliveryDays + 1))
	if err := DB.Model(&NotificationDelivery{}).Where("id = ?", staleSent.ID).
		Update("updated_at", staleCutoff).Error; err != nil {
		t.Fatalf("更新投递时间失败: %v", err)
	}

	if err := RunRetention(); err != nil {
		t.Fatalf("RunRetention 失败: %v", err)
	}

	var count int64
	DB.Model(&MonitorEvent{}).Count(&count)
	if count != 1 {
		t.Fatalf("期望剩余 1 个事件，得到 %d", count)
	}
	var remaining MonitorEvent
	DB.First(&remaining)
	if remaining.DedupeKey != "new" {
		t.Errorf("应保留新事件，得到 %s", remaining.DedupeKey)
	}

	var deliveries []NotificationDelivery
	DB.Find(&deliveries)
	if len(deliveries) != 1 || deliveries[0].ID != keepDelivery.ID {
		t.Fatalf("期望仅保留新事件的 pending 投递，得到 %d 条", len(deliveries))
	}
}

func TestRunRetentionSnapshots(t *testing.T) {
	setupTestDB(t)

	site := &Site{Name: "快照保留", URL: "https://example.com", Container: "div", Item: "a", ConfigVersion: 3}
	if err := DB.Create(site).Error; err != nil {
		t.Fatalf("创建站点失败: %v", err)
	}

	stale := &MonitorSnapshot{SiteID: site.ID, ItemKey: "old-item", DefinitionVersion: 2}
	current := &MonitorSnapshot{SiteID: site.ID, ItemKey: "cur-item", DefinitionVersion: 3}
	if err := DB.Create(stale).Error; err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}
	if err := DB.Create(current).Error; err != nil {
		t.Fatalf("创建快照失败: %v", err)
	}

	if err := RunRetention(); err != nil {
		t.Fatalf("RunRetention 失败: %v", err)
	}

	var snapshots []MonitorSnapshot
	DB.Find(&snapshots)
	if len(snapshots) != 1 || snapshots[0].ItemKey != "cur-item" {
		t.Fatalf("期望仅保留当前版本快照，得到 %d 条", len(snapshots))
	}
}
