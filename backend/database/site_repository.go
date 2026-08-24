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

// UpdateSiteWithFields 事务性地替换站点定义及其字段。
// 任何步骤失败都会整体回滚，保留旧配置。
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

// DeleteSiteCascade 事务性地级联删除站点及其关联数据。
func DeleteSiteCascade(siteID uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
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
