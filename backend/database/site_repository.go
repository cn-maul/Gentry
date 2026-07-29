package database

import (
	"fmt"

	"gorm.io/gorm"
)

// CreateSiteWithFields 事务性地创建站点及其字段。
func CreateSiteWithFields(site *Site) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Fields").Create(site).Error; err != nil {
			return fmt.Errorf("创建站点失败: %w", err)
		}
		if len(site.Fields) > 0 {
			for i := range site.Fields {
				site.Fields[i].SiteID = site.ID
			}
			if err := tx.Create(&site.Fields).Error; err != nil {
				return fmt.Errorf("创建字段失败: %w", err)
			}
		}
		return nil
	})
}

// UpdateSiteWithFields 事务性地更新站点及其字段。
func UpdateSiteWithFields(site *Site, fields []SiteField) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Fields").Save(site).Error; err != nil {
			return fmt.Errorf("保存站点失败: %w", err)
		}
		if err := tx.Where("site_id = ?", site.ID).Delete(&SiteField{}).Error; err != nil {
			return fmt.Errorf("删除旧字段失败: %w", err)
		}
		if len(fields) > 0 {
			for i := range fields {
				fields[i].SiteID = site.ID
			}
			if err := tx.Create(&fields).Error; err != nil {
				return fmt.Errorf("创建字段失败: %w", err)
			}
		}
		return nil
	})
}

// UpdateMonitorDefinition 事务性更新监控器定义（含基线重建）。
// 任何步骤失败都会回滚，保留旧配置和旧快照。
func UpdateMonitorDefinition(site *Site, fields []SiteField, resetBaseline bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Fields").Save(site).Error; err != nil {
			return fmt.Errorf("保存站点失败: %w", err)
		}
		if err := tx.Where("site_id = ?", site.ID).Delete(&SiteField{}).Error; err != nil {
			return fmt.Errorf("删除旧字段失败: %w", err)
		}
		if len(fields) > 0 {
			for i := range fields {
				fields[i].SiteID = site.ID
			}
			if err := tx.Create(&fields).Error; err != nil {
				return fmt.Errorf("创建字段失败: %w", err)
			}
		}
		if resetBaseline {
			if err := tx.Where("site_id = ?", site.ID).Delete(&MonitorSnapshot{}).Error; err != nil {
				return fmt.Errorf("删除旧快照失败: %w", err)
			}
		}
		return nil
	})
}

// ResetMonitorBaseline 事务性删除快照、推进基线版本并更新状态。
func ResetMonitorBaseline(siteID uint) (int, error) {
	newVersion := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("site_id = ?", siteID).Delete(&MonitorSnapshot{}).Error; err != nil {
			return fmt.Errorf("删除快照失败: %w", err)
		}
		result := tx.Model(&Site{}).Where("id = ?", siteID).Updates(map[string]interface{}{
			"baseline_status": "needs_baseline",
			"config_version":  gorm.Expr("config_version + 1"),
		})
		if result.Error != nil {
			return fmt.Errorf("更新基线状态失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("监控器不存在: %d", siteID)
		}
		return tx.Model(&Site{}).Select("config_version").Where("id = ?", siteID).Scan(&newVersion).Error
	})
	return newVersion, err
}

// DeleteSiteCascade 事务性地级联删除站点及其关联数据。
func DeleteSiteCascade(siteID uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("site_id = ?", siteID).Delete(&NotificationDelivery{}).Error; err != nil {
			return fmt.Errorf("删除投递记录失败: %w", err)
		}
		if err := tx.Where("site_id = ?", siteID).Delete(&MonitorEvent{}).Error; err != nil {
			return fmt.Errorf("删除事件记录失败: %w", err)
		}
		if err := tx.Where("site_id = ?", siteID).Delete(&MonitorSnapshot{}).Error; err != nil {
			return fmt.Errorf("删除快照失败: %w", err)
		}
		if err := tx.Where("site_id = ?", siteID).Delete(&UpdateRecord{}).Error; err != nil {
			return fmt.Errorf("删除更新记录失败: %w", err)
		}
		if err := tx.Where("site_id = ?", siteID).Delete(&SiteField{}).Error; err != nil {
			return fmt.Errorf("删除字段失败: %w", err)
		}
		if err := tx.Delete(&Site{}, siteID).Error; err != nil {
			return fmt.Errorf("删除站点失败: %w", err)
		}
		return nil
	})
}
