package database

import (
	"fmt"

	"gorm.io/gorm"
)

// CreateScanRuleTemplate 事务性地创建规则模板及字段。
func CreateScanRuleTemplate(rule *ScanRuleTemplate) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Fields").Create(rule).Error; err != nil {
			return fmt.Errorf("创建扫描规则失败: %w", err)
		}
		if len(rule.Fields) > 0 {
			for i := range rule.Fields {
				rule.Fields[i].RuleID = rule.ID
			}
			if err := tx.Create(&rule.Fields).Error; err != nil {
				return fmt.Errorf("创建扫描规则字段失败: %w", err)
			}
		}
		return nil
	})
}

// UpdateScanRuleTemplate 事务性地更新规则模板及字段。
func UpdateScanRuleTemplate(rule *ScanRuleTemplate, fields []ScanRuleField) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(rule).Error; err != nil {
			return fmt.Errorf("保存扫描规则失败: %w", err)
		}
		if err := tx.Where("rule_id = ?", rule.ID).Delete(&ScanRuleField{}).Error; err != nil {
			return fmt.Errorf("删除扫描规则旧字段失败: %w", err)
		}
		if len(fields) > 0 {
			for i := range fields {
				fields[i].RuleID = rule.ID
			}
			if err := tx.Create(&fields).Error; err != nil {
				return fmt.Errorf("创建扫描规则字段失败: %w", err)
			}
		}
		return nil
	})
}

// DeleteScanRuleTemplate 事务性地删除规则模板及字段。
func DeleteScanRuleTemplate(ruleID uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_id = ?", ruleID).Delete(&ScanRuleField{}).Error; err != nil {
			return fmt.Errorf("删除扫描规则字段失败: %w", err)
		}
		if err := tx.Delete(&ScanRuleTemplate{}, ruleID).Error; err != nil {
			return fmt.Errorf("删除扫描规则失败: %w", err)
		}
		return nil
	})
}
