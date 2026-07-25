package monitor

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"

	"github.com/cn-maul/Gentry/database"
)

// Detector 检测器接口
type Detector interface {
	// Validate 验证检测规则配置
	Validate(schema ExtractionSchema, config json.RawMessage) error
	// Evaluate 比较前后状态并返回结果
	Evaluate(previous SnapshotSet, current []Observation) EvaluationResult
}

// ExtractionSchema 提取模式
type ExtractionSchema struct {
	Container string        `json:"container"`
	Item      string        `json:"item"`
	Fields    []FieldConfig `json:"fields"`
}

// NewDetector 根据类型创建检测器
func NewDetector(ruleType string, rule DetectionRule) Detector {
	switch ruleType {
	case "presence":
		return &PresenceDetector{}
	case "field_transition":
		return NewFieldTransitionDetector(rule)
	default:
		return &PresenceDetector{}
	}
}

// NormalizeAndValidateSiteDefinition 规范化并校验监控定义。
// 创建、更新和引擎启动必须复用此入口，避免前后端校验语义漂移。
func NormalizeAndValidateSiteDefinition(site *database.Site) error {
	if site == nil {
		return fmt.Errorf("site is required")
	}
	parsedURL, err := url.ParseRequestURI(strings.TrimSpace(site.URL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("URL 必须是有效的绝对地址")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL 仅支持 http 或 https")
	}
	canonicalFetchConfig, err := CanonicalFetchConfig(site.FetchConfig, site.URL)
	if err != nil {
		return err
	}
	site.FetchConfig = canonicalFetchConfig
	if strings.TrimSpace(site.Container) == "" {
		return fmt.Errorf("容器选择器不能为空")
	}
	if site.StrategyType == "" {
		site.StrategyType = "presence"
	}

	rule, err := ParseDetectionRule(site.StrategyConfig)
	if err != nil {
		return err
	}
	if rule.Type != site.StrategyType {
		return fmt.Errorf("strategy_type=%s 与 strategy_config.type=%s 不一致", site.StrategyType, rule.Type)
	}
	if rule.Type != "presence" && rule.Type != "field_transition" {
		return fmt.Errorf("不支持的监控类型: %s", rule.Type)
	}

	fieldNames := make(map[string]struct{}, len(site.Fields))
	schema := ExtractionSchema{Container: site.Container, Item: site.Item, Fields: make([]FieldConfig, 0, len(site.Fields))}
	for i := range site.Fields {
		if site.Fields[i].Type == "" {
			site.Fields[i].Type = "text"
		}
		field := site.Fields[i]
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return fmt.Errorf("字段名称不能为空")
		}
		if _, exists := fieldNames[name]; exists {
			return fmt.Errorf("字段名称重复: %s", name)
		}
		if field.Type != "text" && field.Type != "attr" {
			return fmt.Errorf("字段 %s 使用了不支持的提取类型: %s", name, field.Type)
		}
		site.Fields[i].Name = name
		fieldNames[name] = struct{}{}
		schema.Fields = append(schema.Fields, FieldConfig{
			Name: name, Selector: field.Selector, Type: field.Type, Attr: field.Attr, Transform: field.Transform,
		})
	}
	if len(fieldNames) == 0 {
		return fmt.Errorf("至少需要配置一个提取字段")
	}

	dataTypes := make(map[string]string)
	if strings.TrimSpace(site.FieldDataTypes) != "" {
		if err := json.Unmarshal([]byte(site.FieldDataTypes), &dataTypes); err != nil {
			return fmt.Errorf("field_data_types 不是有效 JSON: %w", err)
		}
	}
	if dataTypes == nil {
		dataTypes = make(map[string]string)
	}
	for field, dataType := range dataTypes {
		if _, ok := fieldNames[field]; !ok {
			return fmt.Errorf("field_data_types 引用了不存在的字段: %s", field)
		}
		switch dataType {
		case "text", "money", "decimal", "integer", "url":
		default:
			return fmt.Errorf("字段 %s 使用了不支持的数据类型: %s", field, dataType)
		}
	}
	if err := validateDetectionRule(*rule, schema, fieldNames, dataTypes); err != nil {
		return err
	}

	canonicalRule, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("规范化策略配置失败: %w", err)
	}
	canonicalDataTypes, err := json.Marshal(dataTypes)
	if err != nil {
		return fmt.Errorf("规范化字段类型失败: %w", err)
	}
	site.StrategyConfig = string(canonicalRule)
	site.FieldDataTypes = string(canonicalDataTypes)
	return nil
}

func validateDetectionRule(rule DetectionRule, schema ExtractionSchema, fieldNames map[string]struct{}, dataTypes map[string]string) error {
	identitySources := 0
	if rule.Identity.Source != "" {
		identitySources++
		if rule.Identity.Source != "source_url" {
			return fmt.Errorf("不支持的 identity.source: %s", rule.Identity.Source)
		}
	}
	if rule.Identity.Field != "" {
		identitySources++
		if _, ok := fieldNames[rule.Identity.Field]; !ok {
			return fmt.Errorf("identity 字段不存在: %s", rule.Identity.Field)
		}
	}
	if len(rule.Identity.Fields) > 0 {
		identitySources++
		for _, field := range rule.Identity.Fields {
			if _, ok := fieldNames[field]; !ok {
				return fmt.Errorf("identity 字段不存在: %s", field)
			}
		}
	}
	if identitySources != 1 {
		return fmt.Errorf("identity 必须且只能配置 source、field 或 fields 中的一种")
	}
	if rule.OnFirstBaseline != "silent" && rule.OnFirstBaseline != "emit" {
		return fmt.Errorf("不支持的 on_first_baseline: %s", rule.OnFirstBaseline)
	}

	if rule.Type == "presence" {
		return nil
	}
	if len(rule.Conditions) != 1 {
		return fmt.Errorf("field_transition 当前必须配置且只能配置一个条件")
	}
	condition := rule.Conditions[0]
	if _, ok := fieldNames[condition.Field]; !ok {
		return fmt.Errorf("价格字段不存在: %s", condition.Field)
	}
	if rule.Identity.Field == condition.Field {
		return fmt.Errorf("identity 字段不能使用会发生变化的价格字段: %s", condition.Field)
	}
	for _, identityField := range rule.Identity.Fields {
		if identityField == condition.Field {
			return fmt.Errorf("identity 字段不能使用会发生变化的价格字段: %s", condition.Field)
		}
	}
	if condition.ValueType != "money" {
		return fmt.Errorf("价格条件 value_type 必须为 money")
	}
	if condition.Operator != "decreased" && condition.Operator != "at_or_below" {
		return fmt.Errorf("价格条件仅支持 decreased 或 at_or_below 操作符")
	}
	if configured, ok := dataTypes[condition.Field]; ok && configured != "money" {
		return fmt.Errorf("价格字段 %s 的数据类型必须为 money", condition.Field)
	}
	dataTypes[condition.Field] = "money"
	if condition.Operator == "at_or_below" {
		if condition.Threshold == nil || strings.TrimSpace(condition.Threshold.Value) == "" {
			return fmt.Errorf("到价提醒必须配置目标价格")
		}
		if _, err := parseMinorAmount(condition.Threshold.Value, 3); err != nil {
			return fmt.Errorf("目标价格无效: %w", err)
		}
		return nil
	}
	if threshold := condition.Threshold; threshold != nil {
		if threshold.Amount != "" {
			if _, err := parseMinorAmount(threshold.Amount, 3); err != nil {
				return fmt.Errorf("降价金额阈值无效: %w", err)
			}
		}
		if math.IsNaN(threshold.Percent) || math.IsInf(threshold.Percent, 0) || threshold.Percent < 0 || threshold.Percent > 100 {
			return fmt.Errorf("降价百分比阈值必须在 0 到 100 之间")
		}
	}
	_ = schema
	return nil
}

// ParseDetectionRule 解析检测规则配置
func ParseDetectionRule(configJSON string) (*DetectionRule, error) {
	if configJSON == "" {
		return &DetectionRule{Type: "presence", Identity: IdentityConfig{Source: "source_url"}, OnFirstBaseline: "silent"}, nil
	}
	var rule DetectionRule
	if err := json.Unmarshal([]byte(configJSON), &rule); err != nil {
		return nil, fmt.Errorf("解析检测规则失败: %w", err)
	}
	if rule.Type == "" {
		rule.Type = "presence"
	}
	if rule.OnFirstBaseline == "" {
		rule.OnFirstBaseline = "silent"
	}
	for i := range rule.Conditions {
		threshold := rule.Conditions[i].Threshold
		if threshold != nil {
			threshold.Amount = strings.TrimSpace(threshold.Amount)
			threshold.Value = strings.TrimSpace(threshold.Value)
			if threshold.Amount == "" && threshold.Percent == 0 && threshold.Value == "" {
				rule.Conditions[i].Threshold = nil
			}
		}
	}
	if rule.Identity.Source == "" && rule.Identity.Field == "" && len(rule.Identity.Fields) == 0 {
		rule.Identity.Source = "source_url"
	}
	return &rule, nil
}
