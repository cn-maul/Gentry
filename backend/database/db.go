package database

import (
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
	if err := DB.AutoMigrate(&Site{}, &SiteField{}, &UpdateRecord{}, &NotificationAccount{}, &ScanRuleTemplate{}, &ScanRuleField{}, &SystemSetting{}, &MonitorSnapshot{}, &MonitorEvent{}, &NotificationDelivery{}); err != nil {
		return err
	}

	// 为旧 site 记录设置默认策略类型
	DB.Model(&Site{}).Where("strategy_type = ''").Update("strategy_type", "presence")
	DB.Model(&Site{}).Where("baseline_status = ''").Update("baseline_status", "pending")
	DB.Model(&Site{}).Where("config_version = 0").Update("config_version", 1)
	DB.Model(&MonitorEvent{}).Where("delivery_status = ''").Update("delivery_status", "pending")

	log.Printf("[DB] 数据库就绪: %s", dbPath)
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
