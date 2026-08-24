package database

import (
	"encoding/json"
	"time"
)

// Site 监控站点配置
type Site struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string `gorm:"uniqueIndex;size:255"`
	URL       string `gorm:"size:512"`
	Container string `gorm:"size:255"`
	Item      string `gorm:"size:255"`
	GroupName string `gorm:"size:100;default:默认;index"`
	// CheckInterval 检查间隔（秒），默认 3600
	CheckInterval int        `gorm:"default:3600"`
	IsActive      bool       `gorm:"default:false"`
	LastCheckAt   *time.Time `gorm:"index"`
	// NotifyFilter 推送过滤模式: all=推送所有更新, keyword=仅推送命中关键词的更新
	NotifyFilter string `gorm:"size:20;default:all"`
	// NotifyKeywords 推送关键词（逗号分隔），仅 NotifyFilter=keyword 时生效
	NotifyKeywords string `gorm:"size:500"`
	// NotifyAccountIDs 启用的推送账户 ID 列表（JSON 数组，如 "[1,3,5]"）
	NotifyAccountIDs string `gorm:"size:500"`
	Fields           []SiteField

	// FetchConfig JSON 抓取配置
	FetchConfig string `gorm:"type:text"`
	// BaselineStatus 基线状态: pending, ready, needs_baseline
	BaselineStatus string `gorm:"size:20;default:pending"`
	// ConfigVersion 配置版本号，修改选择器/字段时递增
	ConfigVersion int `gorm:"default:1"`
}

// SiteField 提取字段配置
type SiteField struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	SiteID    uint   `gorm:"index"`
	Name      string `gorm:"size:100"`
	Selector  string `gorm:"size:255"`
	Type      string `gorm:"size:20;default:text"`
	Attr      string `gorm:"size:50"`
	Transform string `gorm:"size:255"`
}

// UpdateRecord 变更历史记录
type UpdateRecord struct {
	ID         uint       `gorm:"primarykey"`
	CreatedAt  time.Time  `gorm:"index"`
	SiteID     uint       `gorm:"index"`
	Title      string     `gorm:"size:500"`
	URL        string     `gorm:"size:512"`
	Summary    string     `gorm:"size:1000"`
	Content    string     `gorm:"type:text"`
	Notified   bool       `gorm:"default:false"`
	NotifiedAt *time.Time `gorm:"index"`
	IsRead     bool       `gorm:"default:false"`
}

// NotificationAccount 推送账户配置
type NotificationAccount struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string `gorm:"size:100;uniqueIndex"`
	Service   string `gorm:"size:50"`
	// ConfigJSON 序列化的账户配置（pushplus: {token,channel}, webhook: {url,method}）
	ConfigJSON string `gorm:"type:text"`
}

// ScanRuleTemplate 可复用的扫描规则模板。
type ScanRuleTemplate struct {
	ID          uint `gorm:"primarykey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Name        string          `gorm:"size:100;uniqueIndex"`
	URLContains string          `gorm:"size:255;index"`
	SourceURL   string          `gorm:"size:1024"`
	ScopeType   string          `gorm:"size:20;index"`
	MatchHost   string          `gorm:"size:255;index"`
	MatchPath   string          `gorm:"size:512"`
	MatchQuery  string          `gorm:"size:1024"`
	Container   string          `gorm:"size:255"`
	Item        string          `gorm:"size:255"`
	Priority    int             `gorm:"default:50"`
	Enabled     bool            `gorm:"default:true;index"`
	Description string          `gorm:"size:500"`
	FetchConfig string          `gorm:"type:text"`
	Fields      []ScanRuleField `gorm:"foreignKey:RuleID"`
}

// ScanRuleField 扫描规则模板字段定义。
type ScanRuleField struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	RuleID    uint   `gorm:"index"`
	Name      string `gorm:"size:100"`
	Selector  string `gorm:"size:255"`
	Type      string `gorm:"size:20;default:text"`
	Attr      string `gorm:"size:50"`
	Transform string `gorm:"size:255"`
}

// TableName 自定义表名
func (Site) TableName() string                { return "sites" }
func (SiteField) TableName() string           { return "site_fields" }
func (UpdateRecord) TableName() string        { return "update_records" }
func (NotificationAccount) TableName() string { return "notification_accounts" }
func (ScanRuleTemplate) TableName() string    { return "scan_rule_templates" }
func (ScanRuleField) TableName() string       { return "scan_rule_fields" }

// SystemSetting 系统设置键值对
type SystemSetting struct {
	ID    uint   `gorm:"primarykey"`
	Key   string `gorm:"uniqueIndex;size:100"`
	Value string `gorm:"type:text"`
}

func (SystemSetting) TableName() string { return "system_settings" }

// GetCheckInterval 返回 time.Duration 形式的检查间隔
func (s *Site) GetCheckInterval() time.Duration {
	switch {
	case s.CheckInterval <= 0:
		return 1 * time.Hour
	default:
		return time.Duration(s.CheckInterval) * time.Second
	}
}

// GetNotifyAccountIDs 解析启用的推送账户 ID 列表
func (s *Site) GetNotifyAccountIDs() []uint {
	if s.NotifyAccountIDs == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(s.NotifyAccountIDs), &ids); err != nil {
		return nil
	}
	return ids
}
