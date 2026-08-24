package web

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/monitor"
)

// fieldRequest 提取字段的请求/响应表示，监控器与扫描规则共用。
type fieldRequest struct {
	Name      string `json:"name" binding:"required"`
	Selector  string `json:"selector"`
	Type      string `json:"type"`
	Attr      string `json:"attr"`
	Transform string `json:"transform"`
}

// addMonitorRequest 创建/更新监控器的请求体。
type addMonitorRequest struct {
	Name             string          `json:"name" binding:"required"`
	URL              string          `json:"url" binding:"required"`
	Container        string          `json:"container" binding:"required"`
	Item             string          `json:"item"`
	Group            string          `json:"group"`
	CheckInterval    int             `json:"check_interval"`
	IsActive         bool            `json:"is_active"`
	NotifyFilter     string          `json:"notify_filter"`
	NotifyKeywords   string          `json:"notify_keywords"`
	NotifyAccountIDs json.RawMessage `json:"notify_account_ids"`
	Fields           []fieldRequest  `json:"fields"`
	FetchConfig      json.RawMessage `json:"fetch_config"`
}

// monitorConfigResponse 监控器完整配置（编辑回显）。
type monitorConfigResponse struct {
	ID               uint            `json:"id"`
	Name             string          `json:"name"`
	URL              string          `json:"url"`
	Container        string          `json:"container"`
	Item             string          `json:"item"`
	Group            string          `json:"group"`
	CheckInterval    int             `json:"check_interval"`
	IsActive         bool            `json:"is_active"`
	NotifyFilter     string          `json:"notify_filter"`
	NotifyKeywords   string          `json:"notify_keywords"`
	NotifyAccountIDs []uint          `json:"notify_account_ids"`
	Fields           []fieldRequest  `json:"fields"`
	FetchConfig      json.RawMessage `json:"fetch_config,omitempty"`
	BaselineStatus   string          `json:"baseline_status,omitempty"`
}

// ===== 字段表示互转 =====

// defaultFieldType 补全字段类型默认值。
func defaultFieldType(t string) string {
	if t == "" {
		return "text"
	}
	return t
}

func fieldRequestsFromSiteFields(fields []database.SiteField) []fieldRequest {
	out := make([]fieldRequest, 0, len(fields))
	for _, f := range fields {
		out = append(out, fieldRequest{Name: f.Name, Selector: f.Selector, Type: f.Type, Attr: f.Attr, Transform: f.Transform})
	}
	return out
}

func siteFieldsFromFieldRequests(fields []fieldRequest) []database.SiteField {
	out := make([]database.SiteField, 0, len(fields))
	for _, f := range fields {
		out = append(out, database.SiteField{
			Name: f.Name, Selector: f.Selector, Type: defaultFieldType(f.Type),
			Attr: f.Attr, Transform: f.Transform,
		})
	}
	return out
}

func scanRuleFieldsFromFieldRequests(fields []fieldRequest) []database.ScanRuleField {
	out := make([]database.ScanRuleField, 0, len(fields))
	for _, f := range fields {
		out = append(out, database.ScanRuleField{
			Name: f.Name, Selector: f.Selector, Type: defaultFieldType(f.Type),
			Attr: f.Attr, Transform: f.Transform,
		})
	}
	return out
}

func scanRuleFieldsToRequests(fields []database.ScanRuleField) []fieldRequest {
	out := make([]fieldRequest, 0, len(fields))
	for _, f := range fields {
		out = append(out, fieldRequest{Name: f.Name, Selector: f.Selector, Type: f.Type, Attr: f.Attr, Transform: f.Transform})
	}
	return out
}

func fieldRequestsFromScanConfigs(fields []monitor.ScanFieldConfig) []fieldRequest {
	out := make([]fieldRequest, 0, len(fields))
	for _, f := range fields {
		out = append(out, fieldRequest{Name: f.Name, Selector: f.Selector, Type: f.Type, Attr: f.Attr, Transform: f.Transform})
	}
	return out
}

// scanConfigsFromFieldRequests 转为扫描配置字段（类型补默认值）。
func scanConfigsFromFieldRequests(fields []fieldRequest) []monitor.ScanFieldConfig {
	out := make([]monitor.ScanFieldConfig, 0, len(fields))
	for _, f := range fields {
		out = append(out, monitor.ScanFieldConfig{
			Name: f.Name, Selector: f.Selector, Type: defaultFieldType(f.Type),
			Attr: f.Attr, Transform: f.Transform,
		})
	}
	return out
}

func rawJSON(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	return string(raw)
}

// jsonRawOrNull 将存储中的 JSON 字符串转为响应用 RawMessage（空值输出 null）。
func jsonRawOrNull(raw string) json.RawMessage {
	if raw == "" {
		return nil
	}
	return json.RawMessage(raw)
}

// ===== 推送账户 ID 归一化 =====

// normalizeNotifyAccountIDs 将请求中的推送账户 ID 归一化为去重升序的 JSON 数组字符串。
// 兼容历史格式：字符串形式的 JSON 数组。
func normalizeNotifyAccountIDs(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var ids []uint
	if err := json.Unmarshal(raw, &ids); err == nil {
		return marshalAccountIDs(ids)
	}
	var legacy string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return "", fmt.Errorf("notify_account_ids must be an array of numbers")
	}
	if legacy == "" {
		return "", nil
	}
	if err := json.Unmarshal([]byte(legacy), &ids); err != nil {
		return "", fmt.Errorf("notify_account_ids contains invalid JSON array: %w", err)
	}
	return marshalAccountIDs(ids)
}

func marshalAccountIDs(ids []uint) (string, error) {
	data, err := json.Marshal(uniqueAccountIDs(ids))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func uniqueAccountIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func applyNotifyAccountIDs(site *database.Site, raw json.RawMessage) error {
	ids, err := normalizeNotifyAccountIDs(raw)
	if err != nil {
		return err
	}
	site.NotifyAccountIDs = ids
	return nil
}

// ===== 监控器定义构建与校验 =====

// dbSiteFromRequest 从创建请求体构建 database.Site。
func dbSiteFromRequest(req *addMonitorRequest) (*database.Site, error) {
	group := req.Group
	if group == "" {
		group = "默认"
	}
	site := &database.Site{
		Name:           req.Name,
		URL:            req.URL,
		Container:      req.Container,
		Item:           req.Item,
		GroupName:      group,
		CheckInterval:  req.CheckInterval,
		IsActive:       req.IsActive,
		NotifyFilter:   req.NotifyFilter,
		NotifyKeywords: req.NotifyKeywords,
		FetchConfig:    rawJSON(req.FetchConfig),
		BaselineStatus: "pending",
		ConfigVersion:  1,
	}
	if err := applyNotifyAccountIDs(site, req.NotifyAccountIDs); err != nil {
		return nil, err
	}
	site.Fields = siteFieldsFromFieldRequests(req.Fields)
	return site, nil
}

// validateMonitorSourceURL 校验监控器的实际抓取 URL 出网安全。
func validateMonitorSourceURL(site *database.Site) error {
	config, err := monitor.ParseFetchConfig(site.FetchConfig, site.URL)
	if err != nil {
		return err
	}
	validationURL, err := monitor.FetchURLValidationTarget(config.URL)
	if err != nil {
		return err
	}
	return validateOutboundURL(validationURL)
}

// ===== 检测指纹 =====

// computeDetectionFingerprint 计算检测语义指纹，用于判断配置变化是否需要重建基线。
func computeDetectionFingerprint(url, container, item string, fields []fieldRequest, fetchConfig string) string {
	type canonicalField struct {
		Name      string `json:"name"`
		Selector  string `json:"selector"`
		Type      string `json:"type"`
		Attr      string `json:"attr"`
		Transform string `json:"transform"`
	}
	canonicalFields := make([]canonicalField, 0, len(fields))
	for _, field := range fields {
		canonicalFields = append(canonicalFields, canonicalField{
			Name: field.Name, Selector: field.Selector, Type: defaultFieldType(field.Type),
			Attr: field.Attr, Transform: field.Transform,
		})
	}
	sort.Slice(canonicalFields, func(i, j int) bool {
		left, _ := json.Marshal(canonicalFields[i])
		right, _ := json.Marshal(canonicalFields[j])
		return string(left) < string(right)
	})

	definition := struct {
		URL         string           `json:"url"`
		Container   string           `json:"container"`
		Item        string           `json:"item"`
		FetchConfig string           `json:"fetch_config"`
		Fields      []canonicalField `json:"fields"`
	}{
		URL: strings.TrimSpace(url), Container: container, Item: item,
		Fields:      canonicalFields,
		FetchConfig: canonicalJSONString(fetchConfig),
	}
	data, _ := json.Marshal(definition)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

// canonicalJSONString 规范化 JSON 字符串，便于等价比较。
func canonicalJSONString(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "{}"
	}
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return strings.TrimSpace(raw)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return string(data)
}
