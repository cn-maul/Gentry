package monitor

import (
	"time"

	"github.com/cn-maul/Gentry/database"
)

// Observation 表示一次提取的规范化观测结果
type Observation struct {
	ItemKey string
	Fields  map[string]TypedValue
	Raw     map[string]interface{}
	SeenAt  time.Time
}

// TypedValue 带类型的值
type TypedValue struct {
	Value    string
	DataType string
	Minor    int64
	Currency string
	Valid    bool
}

// ExtractionValidationSample 是配置验证时返回给前端的只读样本。
type ExtractionValidationSample struct {
	ItemKey    string `json:"item_key"`
	Raw        string `json:"raw"`
	Normalized string `json:"normalized,omitempty"`
	Currency   string `json:"currency,omitempty"`
}

// ExtractionValidationResult 汇总一次只读提取验证的结果。
type ExtractionValidationResult struct {
	ExtractedItems int                          `json:"extracted_items"`
	Samples        []ExtractionValidationSample `json:"samples"`
}

// Snapshot 当前状态快照（内存表示）
type Snapshot struct {
	ItemKey           string
	Payload           map[string]interface{}
	NumericValues     map[string]int64
	Fingerprint       string
	DefinitionVersion int
	FirstSeenAt       time.Time
	LastSeenAt        time.Time
	MissingChecks     int
	Currency          string
	PriceMinor        int64
	PriceValid        bool
}

// ChangeEvent 不可变变化事件
type ChangeEvent struct {
	SiteID            uint
	EventType         string
	ItemKey           string
	Title             string
	URL               string
	Before            map[string]interface{}
	After             map[string]interface{}
	OldValue          string
	NewValue          string
	ChangeAmount      int64
	ChangePercent     float64
	Currency          string
	DedupeKey         string
	DefinitionVersion int
	OccurredAt        time.Time
}

// DetectionRule 检测规则配置
type DetectionRule struct {
	Type            string         `json:"type"`
	Identity        IdentityConfig `json:"identity"`
	Conditions      []Condition    `json:"conditions,omitempty"`
	OnFirstBaseline string         `json:"on_first_baseline"`
	Cooldown        int            `json:"cooldown,omitempty"`
}

// IdentityConfig 身份字段配置
type IdentityConfig struct {
	Field  string   `json:"field,omitempty"`
	Fields []string `json:"fields,omitempty"`
	Source string   `json:"source,omitempty"`
}

// Condition 字段变化条件
type Condition struct {
	Field     string           `json:"field"`
	ValueType string           `json:"value_type"`
	Operator  string           `json:"operator"`
	Threshold *ThresholdConfig `json:"threshold,omitempty"`
}

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	Amount  string  `json:"amount,omitempty"`
	Percent float64 `json:"percent,omitempty"`
	Value   string  `json:"value,omitempty"`
}

// EvaluationResult 检测器评估结果
type EvaluationResult struct {
	NextSnapshots []Snapshot
	Events        []ChangeEvent
}

// EventFormatter 事件格式化接口
type EventFormatter interface {
	Format(event ChangeEvent) (title string, content string)
}

// SnapshotSet 按 item_key 索引的快照集合
type SnapshotSet map[string]Snapshot

// SiteSelectorsFromSite 将 database.Site 转换为 SiteSelectors
// 用于统一 Site 到选择器配置的转换逻辑
func SiteSelectorsFromSite(site *database.Site) SiteSelectors {
	selectors := SiteSelectors{
		Container: site.Container,
		Item:      site.Item,
		Fields:    make([]FieldConfig, len(site.Fields)),
	}
	for i, f := range site.Fields {
		selectors.Fields[i] = FieldConfig{
			Name:      f.Name,
			Selector:  f.Selector,
			Type:      f.Type,
			Attr:      f.Attr,
			Transform: f.Transform,
		}
	}
	return selectors
}
