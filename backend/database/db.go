package database

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init 初始化 SQLite 数据库并自动迁移
func Init(dbPath string) error {
	// SQLite 并发配置：WAL 允许读写并发，busy_timeout 避免瞬时锁冲突直接报错。
	// busy_timeout 是连接级 PRAGMA，通过 DSN 设置保证连接池每条连接都生效。
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}
	var err error
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	// 监控循环、投递队列和 Web handler 会并发写库，SQLite 单写者，
	// 限制单连接串行化写入配合 busy_timeout 最稳。
	if sqlDB, err := DB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	// 自动迁移 Schema
	if err := DB.AutoMigrate(&Site{}, &SiteField{}, &UpdateRecord{}, &NotificationAccount{}, &ScanRuleTemplate{}, &ScanRuleField{}, &SystemSetting{}); err != nil {
		return err
	}

	DB.Model(&Site{}).Where("baseline_status = ''").Update("baseline_status", "pending")
	DB.Model(&Site{}).Where("config_version = 0").Update("config_version", 1)

	if err := dropLegacyPriceMonitors(); err != nil {
		return err
	}

	log.Printf("[DB] 数据库就绪: %s", dbPath)
	return nil
}

// dropLegacyPriceMonitors 一次性迁移：删除旧版价格监控（field_transition）站点
// 及其全部关联数据。价格监控功能已移除，这些站点不再有可用语义。
func dropLegacyPriceMonitors() error {
	// 新库由 AutoMigrate 建表，不存在旧版策略列，无需迁移。
	if !DB.Migrator().HasColumn(&Site{}, "strategy_type") {
		return nil
	}
	result := DB.Exec("DELETE FROM sites WHERE strategy_type = 'field_transition'")
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	log.Printf("[DB] 已移除 %d 个旧版价格监控站点", result.RowsAffected)

	// 清理已无站点归属的关联数据。monitor_* / notification_deliveries 表
	// 属于已删除的旧引擎，先清理数据；表结构保留不影响运行。
	for _, table := range []string{"site_fields", "update_records", "monitor_snapshots", "monitor_events", "notification_deliveries"} {
		if !DB.Migrator().HasTable(table) {
			continue
		}
		if err := DB.Exec(fmt.Sprintf("DELETE FROM %s WHERE site_id NOT IN (SELECT id FROM sites)", table)).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}

// Now 返回当前时间（统一时间源）
func Now() time.Time {
	return time.Now()
}

// GetSetting 获取系统设置
func GetSetting(key string) (string, bool) {
	var s SystemSetting
	DB.Where("key = ?", key).Limit(1).Find(&s)
	if s.Key == "" {
		return "", false
	}
	return s.Value, true
}

// SetSetting 设置系统设置
func SetSetting(key, value string) error {
	return DB.Where("key = ?", key).Assign(SystemSetting{Value: value}).FirstOrCreate(&SystemSetting{Key: key}).Error
}
