package monitor

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/fetcher"
)

// ScanResult 一次规则扫描返回的候选列表
type ScanResult struct {
	URL        string          `json:"url"`
	Containers []ContainerInfo `json:"containers"`
}

// ContainerInfo 单条扫描规则的命中结果
type ContainerInfo struct {
	ContainerCSS string            `json:"container_css"`
	ItemCSS      string            `json:"item_css"`
	ItemCount    int               `json:"item_count"`
	SampleItems  []ExtractResult   `json:"sample_items"`
	Config       ScanMonitorConfig `json:"config"`
	Strategy     string            `json:"strategy,omitempty"`
	RuleName     string            `json:"rule_name,omitempty"`
	RuleAddress  string            `json:"rule_address,omitempty"`
	Diagnostics  []string          `json:"diagnostics,omitempty"`
}

// ScanMonitorConfig 从扫描结果生成的监控配置
type ScanMonitorConfig struct {
	Container string            `json:"container"`
	Item      string            `json:"item"`
	Fields    []ScanFieldConfig `json:"fields"`
	Fetch     *FetchConfig      `json:"fetch_config,omitempty"`
}

// ScanFieldConfig 扫描配置中的字段定义
type ScanFieldConfig struct {
	Name      string `json:"name"`
	Selector  string `json:"selector"`
	Type      string `json:"type"`
	Attr      string `json:"attr,omitempty"`
	Transform string `json:"transform,omitempty"`
}

// ScanSettings 扫描输入
type ScanSettings struct {
	URL string `json:"url"`
}

// SmartScan 按已保存的扫描规则识别页面内容结构，生成可保存的监控配置候选。
// JSON API 规则无需抓取被监控页面本身；HTML 规则遍历所有命中 URL 的启用
// 规则。无规则命中时返回空候选列表，由前端提示用户先创建规则。
func SmartScan(settings *ScanSettings) (*ScanResult, error) {
	if settings == nil {
		return nil, fmt.Errorf("扫描配置不能为空")
	}
	f := fetcher.New()
	apiCandidates := scanJSONTemplateCandidates(settings, f)
	if len(apiCandidates) > 0 {
		return &ScanResult{URL: settings.URL, Containers: apiCandidates}, nil
	}
	html, err := f.Fetch(settings.URL)
	if err != nil {
		return nil, fmt.Errorf("抓取页面失败: %w", err)
	}
	return scanHTMLWithRules(html, settings.URL)
}

// scanHTMLWithRules 对页面应用全部命中的 HTML 扫描规则，按规则优先级排序。
func scanHTMLWithRules(html, pageURL string) (*ScanResult, error) {
	rules := matchingHTMLScanRules(pageURL)
	if len(rules) == 0 {
		return &ScanResult{URL: pageURL}, nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	result := &ScanResult{URL: pageURL}
	for _, rule := range rules {
		if doc.Find(rule.container).Length() == 0 {
			continue
		}
		result.Containers = append(result.Containers, applyHTMLScanRule(html, pageURL, rule))
	}
	return result, nil
}

func applyHTMLScanRule(html, pageURL string, rule htmlScanRule) ContainerInfo {
	fields := scanRuleFieldsToConfigFields(rule.fields)
	if len(fields) == 0 {
		fields = defaultScanFields()
	}
	config := ScanMonitorConfig{Container: rule.container, Item: rule.item, Fields: fields}
	items, err := NewExtractor(ScanConfigToSelectors(config)).Extract(html)
	if err != nil {
		items = nil
	}
	if len(items) > 0 {
		if err := ResolveExtractedURLs(pageURL, items); err != nil {
			log.Printf("[SmartScan] 规则「%s」链接解析失败: %v", rule.name, err)
		}
	}

	diagnostics := []string{fmt.Sprintf("命中扫描规则「%s」", rule.name)}
	samples := items
	if len(samples) > 10 {
		samples = samples[:10]
	}
	if len(samples) == 0 {
		diagnostics = append(diagnostics, "规则未提取到条目，请检查选择器")
	} else {
		diagnostics = append(diagnostics, fmt.Sprintf("提取到 %d 个条目", len(items)))
	}
	return ContainerInfo{
		ContainerCSS: rule.container,
		ItemCSS:      rule.item,
		ItemCount:    len(items),
		SampleItems:  samples,
		Config:       config,
		Strategy:     rule.name,
		RuleName:     cleanScanRuleName(rule.name),
		RuleAddress:  rule.address,
		Diagnostics:  diagnostics,
	}
}

// cleanScanRuleName 去除内部前缀（template_/rule_），得到面向用户的规则名。
func cleanScanRuleName(name string) string {
	name = strings.TrimPrefix(name, "template_")
	name = strings.TrimPrefix(name, "rule_")
	return name
}

// defaultScanFields 规则未定义字段时的默认提取：条目文本作标题，条目内首个链接作 URL。
func defaultScanFields() []ScanFieldConfig {
	return []ScanFieldConfig{
		{Name: "title", Selector: "", Type: "text"},
		{Name: "url", Selector: "a[href]", Type: "attr", Attr: "href"},
	}
}

func scanJSONTemplateCandidates(settings *ScanSettings, client *fetcher.Fetcher) []ContainerInfo {
	db := database.GetDB()
	if db == nil {
		return nil
	}
	var templates []database.ScanRuleTemplate
	if err := db.Preload("Fields").Where("enabled = ?", true).Order("priority desc, id asc").Find(&templates).Error; err != nil {
		log.Printf("[ScannerRules] 加载 JSON API 规则失败: %v", err)
		return nil
	}
	var candidates []ContainerInfo
	for _, template := range templates {
		if !ScanRuleMatchesURL(template, settings.URL) {
			continue
		}
		config, err := ParseFetchConfig(template.FetchConfig, settings.URL)
		if err != nil || config.Mode != FetchModeAPIJSON {
			continue
		}
		candidate, err := buildJSONScanCandidate("template_"+template.Name, templateDisplayAddress(template), config, settings.URL, template.Fields, client)
		if err != nil {
			log.Printf("[ScannerRules] JSON API 规则「%s」扫描失败: %v", template.Name, err)
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func buildJSONScanCandidate(strategy, address string, config FetchConfig, sourceURL string, fields []database.ScanRuleField, client *fetcher.Fetcher) (ContainerInfo, error) {
	resolved, err := ResolveFetchConfig(context.Background(), config, sourceURL, client)
	if err != nil {
		return ContainerInfo{}, err
	}
	body, err := client.FetchContextWithHeaders(context.Background(), resolved.URL, resolved.Headers)
	if err != nil {
		return ContainerInfo{}, err
	}
	siteFields := make([]database.SiteField, 0, len(fields))
	for _, field := range fields {
		siteFields = append(siteFields, database.SiteField{
			Name: field.Name, Selector: field.Selector, Type: field.Type,
			Attr: field.Attr, Transform: field.Transform,
		})
	}
	items, err := extractJSONResults(body, resolved, siteFields)
	if err != nil {
		return ContainerInfo{}, err
	}
	fieldConfigs := scanRuleFieldsToConfigFields(fields)
	return ContainerInfo{
		ContainerCSS: config.ItemsPath, ItemCSS: "*", ItemCount: len(items),
		SampleItems: items, Strategy: strategy,
		RuleName:    cleanScanRuleName(strategy),
		RuleAddress: address,
		Diagnostics: []string{fmt.Sprintf("命中 JSON API 规则「%s」", strategy), fmt.Sprintf("JSON API 提取到 %d 个条目", len(items))},
		Config:      ScanMonitorConfig{Container: config.ItemsPath, Item: "*", Fields: fieldConfigs, Fetch: &config},
	}, nil
}

func ScanConfigToSelectors(config ScanMonitorConfig) SiteSelectors {
	fields := make([]FieldConfig, 0, len(config.Fields))
	for _, f := range config.Fields {
		fields = append(fields, FieldConfig{Name: f.Name, Selector: f.Selector, Type: f.Type, Attr: f.Attr, Transform: f.Transform})
	}
	return SiteSelectors{Container: config.Container, Item: config.Item, Fields: fields}
}

func ScanFieldsToSiteFields(fields []ScanFieldConfig) []database.SiteField {
	result := make([]database.SiteField, 0, len(fields))
	for _, f := range fields {
		ft := f.Type
		if ft == "" {
			ft = "text"
		}
		result = append(result, database.SiteField{Name: f.Name, Selector: f.Selector, Type: ft, Attr: f.Attr, Transform: f.Transform})
	}
	return result
}

func scanRuleFieldsToConfigFields(fields []database.ScanRuleField) []ScanFieldConfig {
	result := make([]ScanFieldConfig, 0, len(fields))
	for _, f := range fields {
		ft := f.Type
		if ft == "" {
			ft = "text"
		}
		result = append(result, ScanFieldConfig{Name: f.Name, Selector: f.Selector, Type: ft, Attr: f.Attr, Transform: f.Transform})
	}
	return result
}
