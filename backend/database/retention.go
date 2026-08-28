package database

import (
	"log"
)

// RetentionUpdateRecordsPerSite 每个站点保留的最新变更记录条数。
// 后续如需可配置，可迁移到 SystemSetting。
const RetentionUpdateRecordsPerSite = 500

// RetentionPushLogs 全局保留的最新推送记录条数。
const RetentionPushLogs = 2000

// RunRetention 执行数据保留清理，删除超出保留策略的历史数据。
func RunRetention() error {
	result := DB.Exec(`
		DELETE FROM update_records WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY site_id ORDER BY id DESC) AS rn
				FROM update_records
			) WHERE rn > ?
		)
	`, RetentionUpdateRecordsPerSite)
	if result.Error != nil {
		log.Printf("[DB] 清理变更记录失败: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("[DB] 已清理变更记录 %d 条", result.RowsAffected)
	}

	// 推送记录保留最新 RetentionPushLogs 条（全局）。
	// 推送被跳过时每个检查周期都会写一条记录，需要保留策略防止无限增长。
	logResult := DB.Exec(`
		DELETE FROM push_logs WHERE id NOT IN (
			SELECT id FROM (
				SELECT id FROM push_logs ORDER BY id DESC LIMIT ?
			)
		)
	`, RetentionPushLogs)
	if logResult.Error != nil {
		log.Printf("[DB] 清理推送记录失败: %v", logResult.Error)
		return logResult.Error
	}
	if logResult.RowsAffected > 0 {
		log.Printf("[DB] 已清理推送记录 %d 条", logResult.RowsAffected)
	}
	return nil
}
