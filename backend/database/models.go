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

	// StrategyType 监控策略: presence（新增检测）, field_transition（字段变化）
	StrategyType string `gorm:"size:50;default:presence;index"`
	// StrategyConfig JSON 策略配置
	StrategyConfig string `gorm:"type:text"`
	// FetchConfig JSON 抓取配置
	FetchConfig string `gorm:"type:text"`
	// BaselineStatus 基线状态: pending, ready, needs_baseline
	BaselineStatus string `gorm:"size:20;default:pending"`
	// ConfigVersion 配置版本号，修改选择器/字段时递增
	ConfigVersion int `gorm:"default:1"`
	// DataType 字段数据类型映射（JSON），如 {"price":"money","title":"text"}
	FieldDataTypes string `gorm:"type:text"`
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

// MonitorSnapshot 当前观测状态快照
type MonitorSnapshot struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	SiteID            uint      `gorm:"uniqueIndex:idx_site_item;index" json:"site_id"`
	ItemKey           string    `gorm:"uniqueIndex:idx_site_item;size:512" json:"item_key"`
	PayloadJSON       string    `gorm:"type:text" json:"payload_json"`
	NumericValuesJSON string    `gorm:"type:text" json:"numeric_values_json"`
	Fingerprint       string    `gorm:"size:64;index" json:"fingerprint"`
	DefinitionVersion int       `gorm:"default:1" json:"definition_version"`
	FirstSeenAt       time.Time `json:"first_seen_at"`
	LastSeenAt        time.Time `gorm:"index" json:"last_seen_at"`
	MissingChecks     int       `gorm:"default:0" json:"missing_checks"`
	Currency          string    `gorm:"size:10" json:"currency"`
	PriceMinor        int64     `gorm:"default:0" json:"price_minor"`
	PriceValid        bool      `gorm:"default:false" json:"price_valid"`
}

func (MonitorSnapshot) TableName() string { return "monitor_snapshots" }

// MonitorEvent 不可变历史事件
type MonitorEvent struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	CreatedAt         time.Time `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	SiteID            uint      `gorm:"uniqueIndex:idx_site_dedupe;index" json:"site_id"`
	EventType         string    `gorm:"size:50;index" json:"event_type"`
	ItemKey           string    `gorm:"size:512;index" json:"item_key"`
	Title             string    `gorm:"size:500" json:"title"`
	URL               string    `gorm:"size:512" json:"url"`
	BeforeJSON        string    `gorm:"type:text" json:"-"`
	AfterJSON         string    `gorm:"type:text" json:"-"`
	OldValue          string    `gorm:"size:500" json:"old_value"`
	NewValue          string    `gorm:"size:500" json:"new_value"`
	ChangeAmount      int64     `gorm:"default:0" json:"change_amount"`
	ChangePercent     float64   `gorm:"default:0" json:"change_percent"`
	Currency          string    `gorm:"size:10" json:"currency"`
	DedupeKey         string    `gorm:"uniqueIndex:idx_site_dedupe;size:64" json:"-"`
	DefinitionVersion int       `gorm:"default:1" json:"definition_version"`
	OccurredAt        time.Time `gorm:"index" json:"occurred_at"`
	Notified          bool      `gorm:"default:false;index" json:"notified"`
	DeliveryStatus    string    `gorm:"size:20;default:pending;index" json:"delivery_status"`
}

func (MonitorEvent) TableName() string { return "monitor_events" }

// NotificationDelivery 投递任务
type NotificationDelivery struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	EventID       uint       `gorm:"uniqueIndex:idx_event_account;index" json:"event_id"`
	AccountID     uint       `gorm:"uniqueIndex:idx_event_account;index" json:"account_id"`
	SiteID        uint       `gorm:"index" json:"site_id"`
	Status        string     `gorm:"size:20;default:pending;index" json:"status"`
	Attempts      int        `gorm:"default:0" json:"attempts"`
	NextAttemptAt *time.Time `gorm:"index" json:"next_attempt_at"`
	LeaseUntil    *time.Time `gorm:"index" json:"lease_until"`
	LastError     string     `gorm:"size:500" json:"last_error"`
	SentAt        *time.Time `json:"sent_at"`
}

func (NotificationDelivery) TableName() string { return "notification_deliveries" }

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
