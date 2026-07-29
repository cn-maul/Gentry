// Package api 定义 Gentry API 的类型和常量
// 本包提供前后端分离的 API 契约定义，供后端实现和前端 SDK 引用
package api

import "time"

// APIVersion 当前 API 版本
const APIVersion = "v1"

// ResponseCode 响应状态码
type ResponseCode int

const (
	// CodeSuccess 成功
	CodeSuccess ResponseCode = 0
	// CodeBadRequest 请求参数错误
	CodeBadRequest ResponseCode = 40001
	// CodeUnauthorized 未授权
	CodeUnauthorized ResponseCode = 40101
	// CodeForbidden 禁止访问
	CodeForbidden ResponseCode = 40301
	// CodeNotFound 资源不存在
	CodeNotFound ResponseCode = 40401
	// CodeConflict 资源冲突
	CodeConflict ResponseCode = 40901
	// CodeInternalServerError 服务器内部错误
	CodeInternalServerError ResponseCode = 50001
)

// APIResponse 标准 API 响应格式
type APIResponse struct {
	Code    int         `json:"code" example:"0"`
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int         `json:"code" example:"40001"`
	Message string      `json:"message" example:"invalid request"`
	Details interface{} `json:"details,omitempty"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Code    int         `json:"code" example:"0"`
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// Monitor 监控器信息
type Monitor struct {
	ID               uint              `json:"id" example:"1"`
	Name             string            `json:"name" example:"example-monitor"`
	URL              string            `json:"url" example:"https://example.com"`
	Container        string            `json:"container" example:"div.list"`
	Item             string            `json:"item" example:"a.item"`
	Group            string            `json:"group" example:"default"`
	CheckInterval    int               `json:"check_interval" example:"3600"`
	IsActive         bool              `json:"is_active" example:"true"`
	NotifyFilter     string            `json:"notify_filter" example:"all"`
	NotifyKeywords   string            `json:"notify_keywords" example:""`
	NotifyAccountIDs []uint            `json:"notify_account_ids" example:"1,2,3"`
	Fields           []MonitorField    `json:"fields"`
	StrategyType     string            `json:"strategy_type" example:"presence"`
	StrategyConfig   interface{}       `json:"strategy_config,omitempty"`
	FieldDataTypes   map[string]string `json:"field_data_types,omitempty"`
	FetchConfig      interface{}       `json:"fetch_config,omitempty"`
	BaselineStatus   string            `json:"baseline_status" example:"ready"`
	ConfigVersion    int               `json:"config_version" example:"1"`
}

// MonitorField 监控器字段配置
type MonitorField struct {
	Name      string `json:"name" example:"title"`
	Selector  string `json:"selector" example:"a.title"`
	Type      string `json:"type" example:"text"`
	Attr      string `json:"attr" example:"href"`
	Transform string `json:"transform" example:""`
}

// CreateMonitorRequest 创建监控器请求
type CreateMonitorRequest struct {
	Name             string            `json:"name" binding:"required" example:"example-monitor"`
	URL              string            `json:"url" binding:"required" example:"https://example.com"`
	Container        string            `json:"container" binding:"required" example:"div.list"`
	Item             string            `json:"item" example:"a.item"`
	Group            string            `json:"group" example:"default"`
	CheckInterval    int               `json:"check_interval" example:"3600"`
	IsActive         bool              `json:"is_active" example:"true"`
	NotifyFilter     string            `json:"notify_filter" example:"all"`
	NotifyKeywords   string            `json:"notify_keywords" example:""`
	NotifyAccountIDs []uint            `json:"notify_account_ids" example:"1,2,3"`
	Fields           []MonitorField    `json:"fields"`
	StrategyType     string            `json:"strategy_type" example:"presence"`
	StrategyConfig   interface{}       `json:"strategy_config,omitempty"`
	FieldDataTypes   map[string]string `json:"field_data_types,omitempty"`
	FetchConfig      interface{}       `json:"fetch_config,omitempty"`
}

// UpdateMonitorRequest 更新监控器请求
type UpdateMonitorRequest struct {
	Name             string            `json:"name" example:"example-monitor"`
	URL              string            `json:"url" example:"https://example.com"`
	Container        string            `json:"container" example:"div.list"`
	Item             string            `json:"item" example:"a.item"`
	Group            string            `json:"group" example:"default"`
	CheckInterval    int               `json:"check_interval" example:"3600"`
	IsActive         bool              `json:"is_active" example:"true"`
	NotifyFilter     string            `json:"notify_filter" example:"all"`
	NotifyKeywords   string            `json:"notify_keywords" example:""`
	NotifyAccountIDs []uint            `json:"notify_account_ids" example:"1,2,3"`
	Fields           []MonitorField    `json:"fields"`
	StrategyType     string            `json:"strategy_type" example:"presence"`
	StrategyConfig   interface{}       `json:"strategy_config,omitempty"`
	FieldDataTypes   map[string]string `json:"field_data_types,omitempty"`
	FetchConfig      interface{}       `json:"fetch_config,omitempty"`
}

// UpdateRecord 更新记录
type UpdateRecord struct {
	ID         uint      `json:"id" example:"1"`
	SiteID     uint      `json:"site_id" example:"1"`
	Title      string    `json:"title" example:"New item found"`
	URL        string    `json:"url" example:"https://example.com/item/1"`
	Content    string    `json:"content" example:"Item description"`
	Notified   bool      `json:"notified" example:"false"`
	NotifiedAt time.Time `json:"notified_at,omitempty"`
	IsRead     bool      `json:"is_read" example:"false"`
	CreatedAt  time.Time `json:"created_at"`
}

// MonitorEvent 监控事件
type MonitorEvent struct {
	ID              uint      `json:"id" example:"1"`
	SiteID          uint      `json:"site_id" example:"1"`
	EventType       string    `json:"event_type" example:"new_item"`
	Title           string    `json:"title" example:"New item found"`
	URL             string    `json:"url" example:"https://example.com/item/1"`
	Content         string    `json:"content" example:"Item description"`
	DeliveryStatus  string    `json:"delivery_status" example:"pending"`
	OccurredAt      time.Time `json:"occurred_at"`
}

// MonitorSnapshot 监控快照
type MonitorSnapshot struct {
	ID                 uint      `json:"id" example:"1"`
	SiteID             uint      `json:"site_id" example:"1"`
	DefinitionVersion  int       `json:"definition_version" example:"1"`
	Hash               string    `json:"hash" example:"abc123"`
	Title              string    `json:"title" example:"Item title"`
	URL                string    `json:"url" example:"https://example.com/item/1"`
	PriceValid         bool      `json:"price_valid" example:"true"`
	PriceMinor         int64     `json:"price_minor" example:"9999"`
	Currency           string    `json:"currency" example:"CNY"`
	PriceDisplay       string    `json:"price_display" example:"¥99.99"`
	FirstSeenAt        time.Time `json:"first_seen_at"`
	LastSeenAt         time.Time `json:"last_seen_at"`
}

// NotificationAccount 推送账户
type NotificationAccount struct {
	ID        uint                   `json:"id" example:"1"`
	Name      string                 `json:"name" example:"my-pushplus"`
	Service   string                 `json:"service" example:"pushplus"`
	Config    map[string]interface{} `json:"config"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// CreateNotificationAccountRequest 创建推送账户请求
type CreateNotificationAccountRequest struct {
	Name    string                 `json:"name" binding:"required" example:"my-pushplus"`
	Service string                 `json:"service" binding:"required" example:"pushplus"`
	Config  map[string]interface{} `json:"config" binding:"required"`
}

// UpdateNotificationAccountRequest 更新推送账户请求
type UpdateNotificationAccountRequest struct {
	Name    string                 `json:"name" example:"my-pushplus"`
	Service string                 `json:"service" example:"pushplus"`
	Config  map[string]interface{} `json:"config"`
}

// ScanRule 扫描规则
type ScanRule struct {
	ID          uint              `json:"id" example:"1"`
	Name        string            `json:"name" example:"example-rule"`
	URLContains string            `json:"url_contains" example:"example.com"`
	SourceURL   string            `json:"source_url" example:"https://example.com"`
	ScopeType   string            `json:"scope_type" example:"domain"`
	MatchHost   string            `json:"match_host" example:"example.com"`
	MatchPath   string            `json:"match_path" example:"/list"`
	MatchQuery  string            `json:"match_query" example:""`
	Container   string            `json:"container" example:"div.list"`
	Item        string            `json:"item" example:"a.item"`
	Priority    int               `json:"priority" example:"50"`
	Enabled     bool              `json:"enabled" example:"true"`
	Description string            `json:"description" example:"Scan example.com"`
	FetchConfig interface{}       `json:"fetch_config,omitempty"`
	Fields      []MonitorField    `json:"fields"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CreateScanRuleRequest 创建扫描规则请求
type CreateScanRuleRequest struct {
	Name        string         `json:"name" binding:"required" example:"example-rule"`
	URLContains string         `json:"url_contains" example:"example.com"`
	SourceURL   string         `json:"source_url" example:"https://example.com"`
	ScopeType   string         `json:"scope_type" example:"domain"`
	Container   string         `json:"container" binding:"required" example:"div.list"`
	Item        string         `json:"item" binding:"required" example:"a.item"`
	Priority    int            `json:"priority" example:"50"`
	Enabled     *bool          `json:"enabled" example:"true"`
	Description string         `json:"description" example:"Scan example.com"`
	FetchConfig interface{}    `json:"fetch_config,omitempty"`
	Fields      []MonitorField `json:"fields"`
}

// UpdateScanRuleRequest 更新扫描规则请求
type UpdateScanRuleRequest struct {
	Name        string         `json:"name" example:"example-rule"`
	URLContains string         `json:"url_contains" example:"example.com"`
	SourceURL   string         `json:"source_url" example:"https://example.com"`
	ScopeType   string         `json:"scope_type" example:"domain"`
	Container   string         `json:"container" example:"div.list"`
	Item        string         `json:"item" example:"a.item"`
	Priority    int            `json:"priority" example:"50"`
	Enabled     *bool          `json:"enabled" example:"true"`
	Description string         `json:"description" example:"Scan example.com"`
	FetchConfig interface{}    `json:"fetch_config,omitempty"`
	Fields      []MonitorField `json:"fields"`
}

// NotificationSettings 通知设置
type NotificationSettings struct {
	Enabled bool `json:"enabled" example:"true"`
}

// HealthCheckResponse 健康检查响应
type HealthCheckResponse struct {
	Status   string `json:"status" example:"ok"`
	Database bool   `json:"database" example:"true"`
	Monitors int    `json:"monitors" example:"5"`
}

// StatsResponse 统计信息响应
type StatsResponse struct {
	TotalMonitors    int64 `json:"total_monitors" example:"10"`
	RunningMonitors  int   `json:"running_monitors" example:"8"`
	TotalUpdates     int64 `json:"total_updates" example:"100"`
	UpdatesLastHour  int64 `json:"updates_last_hour" example:"5"`
	UnnotifiedUpdates int64 `json:"unnotified_updates" example:"3"`
	PushedToday      int64 `json:"pushed_today" example:"20"`
	TotalAccounts    int64 `json:"total_accounts" example:"5"`
}

// VersionResponse 版本信息响应
type VersionResponse struct {
	Version string `json:"version" example:"v1.1.2"`
}

// Pagination 分页参数
type Pagination struct {
	Page int `form:"page" json:"page" example:"1"`
	Size int `form:"size" json:"size" example:"20"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Total int64       `json:"total" example:"100"`
	Page  int         `json:"page" example:"1"`
	Size  int         `json:"size" example:"20"`
	Data  interface{} `json:"data"`
}

// UpdateRecordsResponse 更新记录响应
type UpdateRecordsResponse struct {
	Total   int64          `json:"total" example:"100"`
	Page    int            `json:"page" example:"1"`
	Size    int            `json:"size" example:"20"`
	Records []UpdateRecord `json:"records"`
}

// EventsResponse 事件列表响应
type EventsResponse struct {
	Total  int64          `json:"total" example:"100"`
	Page   int            `json:"page" example:"1"`
	Size   int            `json:"size" example:"20"`
	Events []MonitorEvent `json:"events"`
}

// SnapshotsResponse 快照列表响应
type SnapshotsResponse struct {
	Snapshots []MonitorSnapshot `json:"snapshots"`
}

// ScanResult 扫描结果
type ScanResult struct {
	URL        string          `json:"url" example:"https://example.com"`
	Containers []ContainerInfo `json:"containers"`
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	ContainerTag string   `json:"container_tag" example:"DIV"`
	ContainerCSS string   `json:"container_css" example:"div.list"`
	ItemTag      string   `json:"item_tag" example:"A"`
	ItemCSS      string   `json:"item_css" example:"a.item"`
	ItemCount    int      `json:"item_count" example:"10"`
	Config       interface{} `json:"config"`
	Strategy     string   `json:"strategy" example:"auto"`
	Confidence   float64  `json:"confidence" example:"0.95"`
	Diagnostics  []string `json:"diagnostics"`
	SampleItems  []interface{} `json:"sample_items"`
}

// SmartCreateRequest 智能创建请求
type SmartCreateRequest struct {
	Name             string      `json:"name" binding:"required" example:"example-monitor"`
	URL              string      `json:"url" binding:"required" example:"https://example.com"`
	ContainerCSS     string      `json:"container_css" example:"div.list"`
	Config           interface{} `json:"config"`
	Group            string      `json:"group" example:"default"`
	CheckInterval    int         `json:"check_interval" example:"3600"`
	IsActive         *bool       `json:"is_active" example:"true"`
	NotifyFilter     string      `json:"notify_filter" example:"all"`
	NotifyKeywords   string      `json:"notify_keywords" example:""`
	NotifyAccountIDs []uint      `json:"notify_account_ids" example:"1,2,3"`
}

// ValidateConfigRequest 验证配置请求
type ValidateConfigRequest struct {
	Name             string            `json:"name" example:"example-monitor"`
	URL              string            `json:"url" example:"https://example.com"`
	Container        string            `json:"container" example:"div.list"`
	Item             string            `json:"item" example:"a.item"`
	Fields           []MonitorField    `json:"fields"`
	StrategyType     string            `json:"strategy_type" example:"presence"`
	StrategyConfig   interface{}       `json:"strategy_config,omitempty"`
	FieldDataTypes   map[string]string `json:"field_data_types,omitempty"`
	FetchConfig      interface{}       `json:"fetch_config,omitempty"`
}

// ValidateConfigResponse 验证配置响应
type ValidateConfigResponse struct {
	Valid          bool        `json:"valid" example:"true"`
	Status         string      `json:"status" example:"valid"`
	ExtractedItems int         `json:"extracted_items" example:"10"`
	Items          []ItemValidation `json:"items"`
	Errors         []string    `json:"errors"`
	Summary        string      `json:"summary" example:"配置有效"`
}

// ItemValidation 条目验证结果
type ItemValidation struct {
	Status string `json:"status" example:"ok"`
	Label  string `json:"label" example:"条目提取"`
	Detail string `json:"detail" example:"成功提取并验证 10 条记录"`
	Samples []interface{} `json:"samples"`
}

// ManualCheckResponse 手动检查响应
type ManualCheckResponse struct {
	Events          []interface{} `json:"events"`
	Updates         []interface{} `json:"updates"`
	Count           int           `json:"count" example:"5"`
	IsFirstBaseline bool          `json:"is_first_baseline" example:"false"`
}

// MarkNotifiedResponse 标记已通知响应
type MarkNotifiedResponse struct {
	Updated int64 `json:"updated" example:"10"`
}

// ImportScanRulesResponse 导入扫描规则响应
type ImportScanRulesResponse struct {
	Imported int `json:"imported" example:"5"`
	Skipped  int `json:"skipped" example:"2"`
}

// NotificationProvider 推送服务商
type NotificationProvider struct {
	Service     string            `json:"service" example:"pushplus"`
	Name        string            `json:"name" example:"PushPlus"`
	Description string            `json:"description" example:"PushPlus 推送服务"`
	Fields      []ProviderField   `json:"fields"`
}

// ProviderField 服务商字段
type ProviderField struct {
	Name        string `json:"name" example:"token"`
	Label       string `json:"label" example:"Token"`
	Type        string `json:"type" example:"string"`
	Required    bool   `json:"required" example:"true"`
	Description string `json:"description" example:"PushPlus Token"`
}
