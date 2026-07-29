package monitor

import (
	"encoding/json"
	"time"

	"github.com/cn-maul/Gentry/database"
)

// LoadSnapshots 加载指定监控器当前定义版本的快照。
func LoadSnapshots(siteID uint, definitionVersion int) (SnapshotSet, error) {
	var dbSnapshots []database.MonitorSnapshot
	if err := database.GetDB().Where("site_id = ? AND definition_version = ?", siteID, definitionVersion).Find(&dbSnapshots).Error; err != nil {
		return nil, err
	}

	result := make(SnapshotSet, len(dbSnapshots))
	for _, s := range dbSnapshots {
		payload := make(map[string]interface{})
		if s.PayloadJSON != "" {
			json.Unmarshal([]byte(s.PayloadJSON), &payload)
		}
		result[s.ItemKey] = Snapshot{
			ItemKey:           s.ItemKey,
			Payload:           payload,
			Fingerprint:       s.Fingerprint,
			DefinitionVersion: s.DefinitionVersion,
			FirstSeenAt:       s.FirstSeenAt,
			LastSeenAt:        s.LastSeenAt,
			MissingChecks:     s.MissingChecks,
			Currency:          s.Currency,
			PriceMinor:        s.PriceMinor,
			PriceValid:        s.PriceValid,
		}
	}
	return result, nil
}

// UpdateBaselineStatus 更新监控器基线状态
func UpdateBaselineStatus(siteID uint, status string) error {
	return database.GetDB().Model(&database.Site{}).Where("id = ?", siteID).Update("baseline_status", status).Error
}

// IncrementConfigVersion 递增配置版本
func IncrementConfigVersion(siteID uint) error {
	return database.GetDB().Exec("UPDATE sites SET config_version = config_version + 1 WHERE id = ?", siteID).Error
}

// Now 返回当前时间
func Now() time.Time {
	return time.Now()
}
