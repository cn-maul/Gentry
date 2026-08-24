package database

import (
	"log"
)

// RetentionUpdateRecordsPerSite 每个站点保留的最新变更记录条数。
// 后续如需可配置，可迁移到 SystemSetting。
const RetentionUpdateRecordsPerSite = 500

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
	return nil
}
