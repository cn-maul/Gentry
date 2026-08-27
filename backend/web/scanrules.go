package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/fetcher"
	"github.com/cn-maul/Gentry/llm"
	"github.com/cn-maul/Gentry/monitor"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// scanRuleRequest 创建/更新扫描规则的请求体。
type scanRuleRequest struct {
	Name        string          `json:"name" binding:"required"`
	URLContains string          `json:"url_contains" binding:"required_without=ScopeType"`
	SourceURL   string          `json:"source_url"`
	ScopeType   string          `json:"scope_type"`
	Container   string          `json:"container" binding:"required"`
	Item        string          `json:"item" binding:"required"`
	Priority    int             `json:"priority"`
	Enabled     *bool           `json:"enabled"`
	Description string          `json:"description"`
	FetchConfig json.RawMessage `json:"fetch_config"`
	Fields      []fieldRequest  `json:"fields"`
}

type quickScanRuleRequest struct {
	Name      string                    `json:"name" binding:"required"`
	URL       string                    `json:"url" binding:"required"`
	Keywords  string                    `json:"keywords"`
	ScopeType string                    `json:"scope_type"`
	Config    monitor.ScanMonitorConfig `json:"config" binding:"required"`
}

type scanRuleImportRequest struct {
	Name        string          `json:"name"`
	URLContains string          `json:"url_contains"`
	SourceURL   string          `json:"source_url"`
	ScopeType   string          `json:"scope_type"`
	MatchHost   string          `json:"match_host"`
	MatchPath   string          `json:"match_path"`
	MatchQuery  string          `json:"match_query"`
	Container   string          `json:"container"`
	Item        string          `json:"item"`
	Priority    int             `json:"priority"`
	Enabled     *bool           `json:"enabled"`
	Description string          `json:"description"`
	FetchConfig json.RawMessage `json:"fetch_config"`
	Fields      []fieldRequest  `json:"fields"`
}

type scanRuleResponse struct {
	ID          uint            `json:"id"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Name        string          `json:"name"`
	URLContains string          `json:"url_contains"`
	SourceURL   string          `json:"source_url"`
	ScopeType   string          `json:"scope_type"`
	MatchHost   string          `json:"match_host"`
	MatchPath   string          `json:"match_path"`
	MatchQuery  string          `json:"match_query"`
	Container   string          `json:"container"`
	Item        string          `json:"item"`
	Priority    int             `json:"priority"`
	Enabled     bool            `json:"enabled"`
	Description string          `json:"description"`
	FetchConfig json.RawMessage `json:"fetch_config,omitempty"`
	Fields      []fieldRequest  `json:"fields"`
}

type scanRuleExportDocument struct {
	Version    int                `json:"version"`
	ExportedAt time.Time          `json:"exported_at"`
	Rules      []scanRuleResponse `json:"rules"`
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// ===== 模型互转 =====

func scanRuleFromModel(rule database.ScanRuleTemplate) scanRuleResponse {
	var fetchConfig json.RawMessage
	if rule.FetchConfig != "" {
		fetchConfig = json.RawMessage(rule.FetchConfig)
	}
	return scanRuleResponse{
		ID: rule.ID, CreatedAt: rule.CreatedAt, UpdatedAt: rule.UpdatedAt,
		Name: rule.Name, URLContains: rule.URLContains, SourceURL: rule.SourceURL,
		ScopeType: rule.ScopeType, MatchHost: rule.MatchHost, MatchPath: rule.MatchPath, MatchQuery: rule.MatchQuery,
		Container: rule.Container, Item: rule.Item, Priority: rule.Priority, Enabled: rule.Enabled,
		Description: rule.Description, FetchConfig: fetchConfig,
		Fields: scanRuleFieldsToRequests(rule.Fields),
	}
}

func scanRulesFromModels(rules []database.ScanRuleTemplate) []scanRuleResponse {
	out := make([]scanRuleResponse, 0, len(rules))
	for _, rule := range rules {
		out = append(out, scanRuleFromModel(rule))
	}
	return out
}

// scanRulesForExport 导出时把同站目录规则的 source_url 还原为目录 URL，保证可再导入。
func scanRulesForExport(rules []database.ScanRuleTemplate) []scanRuleResponse {
	out := scanRulesFromModels(rules)
	for index := range out {
		if out[index].ScopeType != monitor.ScanRuleScopeSection || out[index].MatchHost == "" || out[index].MatchPath == "" {
			continue
		}
		scheme := "https"
		if source, err := url.Parse(out[index].SourceURL); err == nil && (source.Scheme == "http" || source.Scheme == "https") {
			scheme = source.Scheme
		}
		scopeURL := &url.URL{
			Scheme: scheme,
			Host:   out[index].MatchHost,
			Path:   strings.TrimSuffix(out[index].MatchPath, "/") + "/",
		}
		out[index].SourceURL = scopeURL.String()
	}
	return out
}

func normalizedPriority(priority int) int {
	if priority <= 0 {
		return 50
	}
	return priority
}

func enabledOrDefault(enabled *bool) bool {
	return enabled == nil || *enabled
}

func dbScanRuleFromRequest(req *scanRuleRequest) *database.ScanRuleTemplate {
	return &database.ScanRuleTemplate{
		Name:        req.Name,
		URLContains: req.URLContains,
		Container:   req.Container,
		Item:        req.Item,
		Priority:    normalizedPriority(req.Priority),
		Enabled:     enabledOrDefault(req.Enabled),
		Description: req.Description,
		FetchConfig: rawJSON(req.FetchConfig),
		Fields:      scanRuleFieldsFromFieldRequests(req.Fields),
	}
}

// ===== 规则校验 =====

func validateScanRuleDefinition(container, item string, fields []fieldRequest) error {
	container = strings.TrimSpace(container)
	item = strings.TrimSpace(item)
	if container == "" || item == "" {
		return fmt.Errorf("扫描结果缺少容器或列表项配置")
	}
	if _, err := cascadia.Compile(container); err != nil {
		return fmt.Errorf("容器选择器无效: %w", err)
	}
	if _, err := cascadia.Compile(item); err != nil {
		return fmt.Errorf("列表项选择器无效: %w", err)
	}
	hasTitle := false
	for _, field := range fields {
		if strings.TrimSpace(field.Name) == "" {
			return fmt.Errorf("字段名称不能为空")
		}
		if field.Name == "title" {
			hasTitle = true
		}
		if strings.TrimSpace(field.Selector) != "" {
			if _, err := cascadia.Compile(field.Selector); err != nil {
				return fmt.Errorf("字段 %q 的选择器无效: %w", field.Name, err)
			}
		}
	}
	if !hasTitle {
		return fmt.Errorf("规则至少需要一个 title 字段")
	}
	return nil
}

func validateScanRuleConfig(container, item string, fields []fieldRequest, fetchConfig *monitor.FetchConfig) error {
	if fetchConfig == nil || fetchConfig.Mode == "" || fetchConfig.Mode == monitor.FetchModeHTML {
		return validateScanRuleDefinition(container, item, fields)
	}
	if fetchConfig.Mode != monitor.FetchModeAPIJSON {
		return fmt.Errorf("不支持的规则数据源模式: %s", fetchConfig.Mode)
	}
	if strings.TrimSpace(fetchConfig.URL) == "" || strings.TrimSpace(fetchConfig.ItemsPath) == "" {
		return fmt.Errorf("JSON API 规则缺少接口 URL 或列表路径")
	}
	hasTitle := false
	for _, field := range fields {
		if strings.TrimSpace(field.Name) == "" || strings.TrimSpace(field.Selector) == "" {
			return fmt.Errorf("JSON API 规则字段名称和路径不能为空")
		}
		if field.Name == "title" {
			hasTitle = true
		}
	}
	if !hasTitle {
		return fmt.Errorf("规则至少需要一个 title 字段")
	}
	return nil
}

// validateScanRuleSource 校验规则数据源 URL 的出网安全。
func validateScanRuleSource(rule *database.ScanRuleTemplate, fallbackURL string) error {
	if strings.TrimSpace(rule.FetchConfig) == "" {
		return nil
	}
	config, err := monitor.ParseFetchConfig(rule.FetchConfig, fallbackURL)
	if err != nil {
		return err
	}
	validationURL, err := monitor.FetchURLValidationTarget(config.URL)
	if err != nil {
		return err
	}
	return validateOutboundURL(validationURL)
}

// dbQuickScanRuleFromRequest 由快速扫描候选构建规则模板。
func dbQuickScanRuleFromRequest(req *quickScanRuleRequest) (*database.ScanRuleTemplate, error) {
	fields := fieldRequestsFromScanConfigs(req.Config.Fields)
	if err := validateScanRuleConfig(req.Config.Container, req.Config.Item, fields, req.Config.Fetch); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	rule := &database.ScanRuleTemplate{
		Name:        name,
		Container:   strings.TrimSpace(req.Config.Container),
		Item:        strings.TrimSpace(req.Config.Item),
		Priority:    50,
		Enabled:     true,
		Description: "通过快速扫描生成",
		FetchConfig: rawJSON(mustJSON(req.Config.Fetch)),
		Fields:      scanRuleFieldsFromFieldRequests(fields),
	}
	if err := monitor.ApplyScanRuleScope(rule, req.URL, req.ScopeType); err != nil {
		return nil, err
	}
	return rule, nil
}

func mustJSON(value interface{}) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return data
}

// dbImportedScanRule 校验并构建导入的规则模板。
func dbImportedScanRule(req scanRuleImportRequest) (*database.ScanRuleTemplate, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	rule := &database.ScanRuleTemplate{
		Name:        name,
		URLContains: strings.TrimSpace(req.URLContains),
		Container:   strings.TrimSpace(req.Container),
		Item:        strings.TrimSpace(req.Item),
		Priority:    normalizedPriority(req.Priority),
		Enabled:     enabledOrDefault(req.Enabled),
		Description: req.Description,
		FetchConfig: rawJSON(req.FetchConfig),
		Fields:      scanRuleFieldsFromFieldRequests(req.Fields),
	}

	var parsedFetch *monitor.FetchConfig
	if len(req.FetchConfig) > 0 && string(req.FetchConfig) != "null" {
		var config monitor.FetchConfig
		if err := json.Unmarshal(req.FetchConfig, &config); err != nil {
			return nil, fmt.Errorf("fetch_config 无效: %w", err)
		}
		parsedFetch = &config
	}
	if err := validateScanRuleConfig(rule.Container, rule.Item, req.Fields, parsedFetch); err != nil {
		return nil, err
	}

	if req.ScopeType == "" {
		if rule.URLContains == "" {
			return nil, fmt.Errorf("旧版规则缺少 url_contains")
		}
		return rule, nil
	}
	// 同站目录规则：校验 source_url 与匹配范围一致后再推导 scope。
	if req.ScopeType == monitor.ScanRuleScopeSection && strings.TrimSpace(req.MatchHost) != "" && strings.TrimSpace(req.MatchPath) != "" {
		source, err := url.Parse(strings.TrimSpace(req.SourceURL))
		if err != nil || source.Scheme == "" || source.Host == "" {
			return nil, fmt.Errorf("同站目录规则的 source_url 无效")
		}
		matchPath := path.Clean("/" + strings.TrimPrefix(req.MatchPath, "/"))
		if !strings.EqualFold(source.Host, req.MatchHost) || strings.TrimSuffix(path.Clean(source.Path), "/") != strings.TrimSuffix(matchPath, "/") {
			return nil, fmt.Errorf("同站目录规则的 source_url 与匹配范围不一致")
		}
		scopeProbe := *source
		scopeProbe.Path = strings.TrimSuffix(matchPath, "/") + "/__gentry_scope__"
		if err := monitor.ApplyScanRuleScope(rule, scopeProbe.String(), req.ScopeType); err != nil {
			return nil, err
		}
		rule.SourceURL = source.String()
		return rule, nil
	}
	if err := monitor.ApplyScanRuleScope(rule, req.SourceURL, req.ScopeType); err != nil {
		return nil, err
	}
	return rule, nil
}

// ===== 处理器 =====

func listScanRules(c *gin.Context) {
	var rules []database.ScanRuleTemplate
	if err := database.GetDB().Preload("Fields").Order("priority desc, created_at asc").Find(&rules).Error; err != nil {
		fail(c, http.StatusInternalServerError, "加载扫描规则失败: "+err.Error())
		return
	}
	ok(c, scanRulesFromModels(rules))
}

// reloadScanRule 按 ID 重新加载含字段的规则。
func reloadScanRule(id uint) (*database.ScanRuleTemplate, error) {
	var rule database.ScanRuleTemplate
	if err := database.GetDB().Preload("Fields").First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func createScanRule(c *gin.Context) {
	var req scanRuleRequest
	if !bindJSON(c, &req) {
		return
	}
	rule := dbScanRuleFromRequest(&req)
	if err := validateScanRuleSource(rule, ""); err != nil {
		fail(c, http.StatusBadRequest, "规则数据源无效: "+err.Error())
		return
	}
	if err := database.CreateScanRuleTemplate(rule); err != nil {
		fail(c, http.StatusConflict, "创建扫描规则失败: "+err.Error())
		return
	}
	reloaded, err := reloadScanRule(rule.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取新建扫描规则失败: "+err.Error())
		return
	}
	created(c, scanRuleFromModel(*reloaded))
}

func quickCreateScanRule(c *gin.Context) {
	var req quickScanRuleRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		fail(c, http.StatusBadRequest, "URL 无效: "+err.Error())
		return
	}
	rule, err := dbQuickScanRuleFromRequest(&req)
	if err != nil {
		fail(c, http.StatusBadRequest, "规则无效: "+err.Error())
		return
	}
	if err := validateScanRuleSource(rule, req.URL); err != nil {
		fail(c, http.StatusBadRequest, "规则数据源无效: "+err.Error())
		return
	}
	if err := database.CreateScanRuleTemplate(rule); err != nil {
		fail(c, http.StatusConflict, "创建扫描规则失败: "+err.Error())
		return
	}
	reloaded, reloadErr := reloadScanRule(rule.ID)
	if reloadErr != nil {
		fail(c, http.StatusInternalServerError, "读取新建扫描规则失败: "+reloadErr.Error())
		return
	}
	created(c, scanRuleFromModel(*reloaded))
}

func exportScanRules(c *gin.Context) {
	var rules []database.ScanRuleTemplate
	if err := database.GetDB().Preload("Fields").Order("priority desc, created_at asc").Find(&rules).Error; err != nil {
		fail(c, http.StatusInternalServerError, "导出扫描规则失败: "+err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="gentry-scan-rules.json"`)
	ok(c, scanRuleExportDocument{Version: 1, ExportedAt: time.Now(), Rules: scanRulesForExport(rules)})
}

func importScanRules(c *gin.Context) {
	var document struct {
		Version int                     `json:"version"`
		Rules   []scanRuleImportRequest `json:"rules"`
	}
	if !bindJSON(c, &document) {
		return
	}
	if document.Version != 1 {
		fail(c, http.StatusBadRequest, fmt.Sprintf("不支持的规则文件版本: %d", document.Version))
		return
	}
	if len(document.Rules) == 0 || len(document.Rules) > 500 {
		fail(c, http.StatusBadRequest, "规则文件必须包含 1 到 500 条规则")
		return
	}

	prepared := make([]*database.ScanRuleTemplate, 0, len(document.Rules))
	seen := make(map[string]struct{}, len(document.Rules))
	for index, imported := range document.Rules {
		rule, err := dbImportedScanRule(imported)
		if err != nil {
			fail(c, http.StatusBadRequest, fmt.Sprintf("第 %d 条规则无效: %v", index+1, err))
			return
		}
		if err := validateScanRuleSource(rule, rule.SourceURL); err != nil {
			fail(c, http.StatusBadRequest, fmt.Sprintf("第 %d 条规则数据源无效: %v", index+1, err))
			return
		}
		if _, exists := seen[rule.Name]; exists {
			fail(c, http.StatusBadRequest, fmt.Sprintf("规则文件中名称重复: %s", rule.Name))
			return
		}
		seen[rule.Name] = struct{}{}
		prepared = append(prepared, rule)
	}

	// 跳过与现有规则重名的条目。
	var existingNames []string
	if err := database.GetDB().Model(&database.ScanRuleTemplate{}).Pluck("name", &existingNames).Error; err != nil {
		fail(c, http.StatusInternalServerError, "读取现有规则失败: "+err.Error())
		return
	}
	existing := make(map[string]struct{}, len(existingNames))
	for _, name := range existingNames {
		existing[name] = struct{}{}
	}
	toCreate := make([]*database.ScanRuleTemplate, 0, len(prepared))
	skipped := 0
	for _, rule := range prepared {
		if _, exists := existing[rule.Name]; exists {
			skipped++
			continue
		}
		toCreate = append(toCreate, rule)
	}

	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		for _, rule := range toCreate {
			if err := tx.Omit("Fields").Create(rule).Error; err != nil {
				return err
			}
			for i := range rule.Fields {
				rule.Fields[i].RuleID = rule.ID
			}
			if len(rule.Fields) > 0 {
				if err := tx.Create(&rule.Fields).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		fail(c, http.StatusInternalServerError, "导入扫描规则失败: "+err.Error())
		return
	}
	ok(c, map[string]int{"imported": len(toCreate), "skipped": skipped})
}

func updateScanRule(c *gin.Context) {
	var req scanRuleRequest
	if !bindJSON(c, &req) {
		return
	}

	id := c.Param("id")
	var rule database.ScanRuleTemplate
	if err := database.GetDB().Preload("Fields").First(&rule, id).Error; err != nil {
		fail(c, http.StatusNotFound, "扫描规则不存在")
		return
	}

	updated := dbScanRuleFromRequest(&req)
	var parsedFetch *monitor.FetchConfig
	if updated.FetchConfig != "" {
		config, parseErr := monitor.ParseFetchConfig(updated.FetchConfig, firstNonEmptyString(req.SourceURL, rule.SourceURL))
		if parseErr != nil {
			fail(c, http.StatusBadRequest, "规则数据源无效: "+parseErr.Error())
			return
		}
		parsedFetch = &config
	}
	if err := validateScanRuleConfig(updated.Container, updated.Item, req.Fields, parsedFetch); err != nil {
		fail(c, http.StatusBadRequest, "规则无效: "+err.Error())
		return
	}
	if err := validateScanRuleSource(updated, rule.SourceURL); err != nil {
		fail(c, http.StatusBadRequest, "规则数据源无效: "+err.Error())
		return
	}

	rule.Name = updated.Name
	rule.URLContains = updated.URLContains
	rule.Container = updated.Container
	rule.Item = updated.Item
	rule.Priority = updated.Priority
	rule.Enabled = updated.Enabled
	rule.Description = updated.Description
	rule.FetchConfig = updated.FetchConfig
	if req.ScopeType != "" {
		sourceURL := firstNonEmptyString(req.SourceURL, rule.SourceURL)
		if err := monitor.ApplyScanRuleScope(&rule, sourceURL, req.ScopeType); err != nil {
			fail(c, http.StatusBadRequest, "规则适用范围无效: "+err.Error())
			return
		}
	}
	if err := database.UpdateScanRuleTemplate(&rule, updated.Fields); err != nil {
		fail(c, http.StatusInternalServerError, "更新扫描规则失败: "+err.Error())
		return
	}
	reloaded, err := reloadScanRule(rule.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "读取更新后的扫描规则失败: "+err.Error())
		return
	}
	ok(c, scanRuleFromModel(*reloaded))
}

func deleteScanRule(c *gin.Context) {
	var rule database.ScanRuleTemplate
	if err := database.GetDB().First(&rule, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "扫描规则不存在")
		return
	}
	if err := database.DeleteScanRuleTemplate(rule.ID); err != nil {
		fail(c, http.StatusInternalServerError, "删除扫描规则失败: "+err.Error())
		return
	}
	ok(c, nil)
}

// ===== 规则测试 / 智能扫描 =====

func previewScan(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		fail(c, http.StatusBadRequest, "URL 无效: "+err.Error())
		return
	}

	result, err := monitor.SmartScan(&monitor.ScanSettings{URL: req.URL})
	if err != nil {
		fail(c, http.StatusInternalServerError, "扫描失败: "+err.Error())
		return
	}
	ok(c, result)
}

func testScanRule(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		fail(c, http.StatusBadRequest, "URL 无效: "+err.Error())
		return
	}

	rule, err := reloadScanRule(parseUintParam(c.Param("id")))
	if err != nil {
		fail(c, http.StatusNotFound, "扫描规则不存在")
		return
	}
	if !monitor.ScanRuleMatchesURL(*rule, req.URL) {
		fail(c, http.StatusBadRequest, "测试 URL 不在该规则的适用范围内")
		return
	}

	// JSON API 数据源走配置化提取；HTML 数据源走选择器提取。
	if rule.FetchConfig != "" {
		fetchConfig, parseErr := monitor.ParseFetchConfig(rule.FetchConfig, req.URL)
		if parseErr != nil {
			fail(c, http.StatusBadRequest, "规则数据源无效: "+parseErr.Error())
			return
		}
		if fetchConfig.Mode == monitor.FetchModeAPIJSON {
			testScanRuleAPI(c, req.URL, rule, fetchConfig)
			return
		}
	}
	testScanRuleHTML(c, req.URL, rule)
}

// testScanRuleAPI 测试 JSON API 规则：直接按配置提取。
func testScanRuleAPI(c *gin.Context, testURL string, rule *database.ScanRuleTemplate, fetchConfig monitor.FetchConfig) {
	site := &database.Site{
		URL: testURL, Container: rule.Container, Item: rule.Item,
		FetchConfig: rule.FetchConfig,
	}
	for _, field := range rule.Fields {
		site.Fields = append(site.Fields, database.SiteField{
			Name: field.Name, Selector: field.Selector, Type: defaultFieldType(field.Type),
			Attr: field.Attr, Transform: field.Transform,
		})
	}
	items, extractErr := monitor.ExtractConfiguredSource(c.Request.Context(), site)
	if extractErr != nil {
		fail(c, http.StatusBadRequest, "规则提取失败: "+extractErr.Error())
		return
	}
	sampleItems := items
	if len(sampleItems) > 10 {
		sampleItems = sampleItems[:10]
	}
	ok(c, &monitor.ScanResult{
		URL: testURL,
		Containers: []monitor.ContainerInfo{{
			ContainerCSS: rule.Container,
			ItemCSS:      rule.Item, ItemCount: len(items),
			Config:      monitor.ScanMonitorConfig{Container: rule.Container, Item: rule.Item, Fields: scanConfigsFromFieldRequests(scanRuleFieldsToRequests(rule.Fields)), Fetch: &fetchConfig},
			Strategy:    "rule_test",
			Diagnostics: []string{"测试规则匹配成功", fmt.Sprintf("JSON API 提取到 %d 个条目", len(items))},
			SampleItems: sampleItems,
		}},
	})
}

// testScanRuleHTML 测试 HTML 规则：抓取页面后按选择器提取。
func testScanRuleHTML(c *gin.Context, testURL string, rule *database.ScanRuleTemplate) {
	html, err := fetcher.New().Fetch(testURL)
	if err != nil {
		fail(c, http.StatusInternalServerError, "获取页面失败: "+err.Error())
		return
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		fail(c, http.StatusInternalServerError, "解析页面失败: "+err.Error())
		return
	}
	if doc.Find(rule.Container).Length() == 0 {
		fail(c, http.StatusBadRequest, fmt.Sprintf("容器选择器 %q 未匹配到元素", rule.Container))
		return
	}

	fields := scanConfigsFromFieldRequests(scanRuleFieldsToRequests(rule.Fields))
	if len(fields) == 0 {
		fields = append(fields, monitor.ScanFieldConfig{Name: "title", Type: "text"})
	}
	selectors := monitor.ScanConfigToSelectors(monitor.ScanMonitorConfig{
		Container: rule.Container,
		Item:      rule.Item,
		Fields:    fields,
	})
	items, err := monitor.NewExtractor(selectors).Extract(html)
	if err != nil {
		fail(c, http.StatusInternalServerError, "解析页面失败: "+err.Error())
		return
	}
	if err := monitor.ResolveExtractedURLs(testURL, items); err != nil {
		fail(c, http.StatusInternalServerError, "解析链接失败: "+err.Error())
		return
	}

	sampleItems := items
	if sampleItems == nil {
		sampleItems = []monitor.ExtractResult{}
	}

	diagnostics := []string{"测试规则匹配成功"}
	if len(items) == 0 {
		diagnostics = append(diagnostics, "未找到匹配项目")
	} else {
		diagnostics = append(diagnostics, fmt.Sprintf("容器: %s, 项目: %s", rule.Container, rule.Item))
	}
	// 即使没有匹配到项目也返回结果，由前端展示「未找到项目」。
	ok(c, &monitor.ScanResult{
		URL: testURL,
		Containers: []monitor.ContainerInfo{{
			ContainerCSS: rule.Container,
			ItemCSS:      rule.Item,
			ItemCount:    len(items),
			Config:       monitor.ScanMonitorConfig{Container: rule.Container, Item: rule.Item},
			Strategy:     "rule_test",
			Diagnostics:  diagnostics,
			SampleItems:  sampleItems,
		}},
	})
}

// parseUintParam 解析数字路由参数，非法输入返回 0（查不到记录）。
func parseUintParam(raw string) uint {
	id, _ := strconv.ParseUint(raw, 10, 64)
	return uint(id)
}

// captureScanRule 统一捕获端点：给定页面 URL 与关键词，返回候选规则草稿
// （选择器 + 样本 + 诊断信息）。草稿不会自动入库，由用户确认后走 quickCreate。
func captureScanRule(c *gin.Context) {
	var req struct {
		URL      string `json:"url" binding:"required"`
		Keywords string `json:"keywords"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		fail(c, http.StatusBadRequest, "URL 无效: "+err.Error())
		return
	}

	result, err := monitor.RunCapture(c.Request.Context(), monitor.CaptureRequest{
		URL:      req.URL,
		Keywords: splitKeywords(req.Keywords),
	})
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, llm.ErrNotConfigured) || llm.IsFatal(err):
			// 配置缺失或鉴权失败：客户端可修复，返回 400 与明确原因
			status = http.StatusBadRequest
		case strings.Contains(err.Error(), "抓取页面失败"):
			status = http.StatusBadGateway
		case strings.Contains(err.Error(), "未能生成可用的选择器"):
			status = http.StatusBadRequest
		}
		fail(c, status, err.Error())
		return
	}
	ok(c, result)
}

// testDraftMonitor 草稿直测：在不落库的情况下按候选/手填的选择器配置
// 执行一次只读提取，返回与 /monitors/validate 相同结构的验证报告。
// 替代前端「拼假监控请求」的旧测试方式。
func testDraftMonitor(c *gin.Context) {
	var req struct {
		URL         string          `json:"url" binding:"required"`
		Container   string          `json:"container" binding:"required"`
		Item        string          `json:"item"`
		Fields      []fieldRequest  `json:"fields"`
		FetchConfig json.RawMessage `json:"fetch_config"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		fail(c, http.StatusBadRequest, "URL 无效: "+err.Error())
		return
	}

	site := &database.Site{
		URL:       req.URL,
		Container: req.Container,
		Item:      req.Item,
		FetchConfig: func() string {
			if len(req.FetchConfig) > 0 && string(req.FetchConfig) != "null" {
				return string(req.FetchConfig)
			}
			return ""
		}(),
		Fields: siteFieldsFromFieldRequests(req.Fields),
	}
	if len(site.Fields) == 0 {
		site.Fields = []database.SiteField{{Name: "title", Selector: "", Type: "text"}}
	}
	if err := monitor.NormalizeAndValidateSiteDefinition(site); err != nil {
		fail(c, http.StatusBadRequest, "invalid draft config: "+err.Error())
		return
	}
	if err := validateMonitorSourceURL(site); err != nil {
		fail(c, http.StatusBadRequest, "invalid draft source: "+err.Error())
		return
	}
	report, err := monitor.ValidateExtraction(c.Request.Context(), site)
	if err != nil {
		fail(c, http.StatusBadRequest, "config validation failed: "+err.Error())
		return
	}
	ok(c, gin.H{
		"valid":           true,
		"status":          "valid",
		"extracted_items": report.ExtractedItems,
		"items": []gin.H{
			{
				"status":  "ok",
				"label":   "条目提取",
				"detail":  fmt.Sprintf("成功提取并验证 %d 条记录", report.ExtractedItems),
				"samples": report.Samples,
			},
		},
		"errors":  []string{},
		"summary": fmt.Sprintf("配置有效，共提取 %d 条记录；本次验证未写入基线或发送通知。", report.ExtractedItems),
	})
}
