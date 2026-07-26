package database

import (
	"log"
	"time"
)

// 数据保留策略常量。后续如需可配置，可迁移到 SystemSetting。
const (
	// RetentionUpdateRecordsPerSite 每个站点保留的最新变更记录条数
	RetentionUpdateRecordsPerSite = 500
	// RetentionEventDays 监控事件保留天数
	RetentionEventDays = 180
	// RetentionDeliveryDays 已终态投递任务保留天数
	RetentionDeliveryDays = 30
)

// RunRetention 执行数据保留清理，删除超出保留策略的历史数据。
// 各类清理相互独立，单类失败不影响其他类，返回最后一个错误。
func RunRetention() error {
	var lastErr error

	// 1. update_records：每个站点只保留最近 N 条（按 id 倒序）
	result := DB.Exec(`
		DELETE FROM update_records WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY site_id ORDER BY id DESC) AS rn
				FROM update_records
			) WHERE rn > ?
		)
	`, RetentionUpdateRecordsPerSite)
	lastErr = logRetention("变更记录", result.RowsAffected, result.Error, lastErr)

	// 2. monitor_events：删除过期事件
	eventCutoff := time.Now().AddDate(0, 0, -RetentionEventDays)
	result = DB.Exec(`DELETE FROM monitor_events WHERE created_at < ?`, eventCutoff)
	lastErr = logRetention("监控事件", result.RowsAffected, result.Error, lastErr)

	// 3. notification_deliveries：孤儿记录（事件已删除）
	result = DB.Exec(`
		DELETE FROM notification_deliveries
		WHERE event_id NOT IN (SELECT id FROM monitor_events)
	`)
	lastErr = logRetention("孤儿投递任务", result.RowsAffected, result.Error, lastErr)

	// 3b. notification_deliveries：过期的终态投递任务
	deliveryCutoff := time.Now().AddDate(0, 0, -RetentionDeliveryDays)
	result = DB.Exec(`
		DELETE FROM notification_deliveries
		WHERE status IN ('sent', 'skipped', 'dead') AND updated_at < ?
	`, deliveryCutoff)
	lastErr = logRetention("终态投递任务", result.RowsAffected, result.Error, lastErr)

	// 4. monitor_snapshots：删除旧配置版本遗留的快照
	result = DB.Exec(`
		DELETE FROM monitor_snapshots
		WHERE EXISTS (
			SELECT 1 FROM sites
			WHERE sites.id = monitor_snapshots.site_id
			  AND monitor_snapshots.definition_version < sites.config_version
		)
	`)
	lastErr = logRetention("过期快照", result.RowsAffected, result.Error, lastErr)

	return lastErr
}

// logRetention 记录单类清理结果，返回应向上传递的错误
func logRetention(label string, rows int64, err, lastErr error) error {
	if err != nil {
		log.Printf("[DB] 清理%s失败: %v", label, err)
		return err
	}
	if rows > 0 {
		log.Printf("[DB] 已清理%s %d 条", label, rows)
	}
	return lastErr
}
