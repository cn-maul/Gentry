package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/fetcher"
)

const (
	FetchModeHTML    = "html"
	FetchModeAPIJSON = "api_json"
)

// FetchConfig describes how a monitor obtains its source document. Empty
// configuration remains the legacy HTML mode.
type FetchConfig struct {
	Mode         string                   `json:"mode"`
	URL          string                   `json:"url,omitempty"`
	ItemsPath    string                   `json:"items_path,omitempty"`
	FilterPath   string                   `json:"filter_path,omitempty"`
	FilterEquals string                   `json:"filter_equals,omitempty"`
	Headers      map[string]string        `json:"headers,omitempty"`
	Variables    map[string]FetchVariable `json:"variables,omitempty"`
}

// FetchVariable extracts a value from the monitored HTML page before the
// configured API request is made. Attr is optional; without it, text is used.
type FetchVariable struct {
	Source   string `json:"source,omitempty"`
	Selector string `json:"selector"`
	Attr     string `json:"attr,omitempty"`
}

var fetchTemplateVariablePattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)
var fetchVariableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ParseFetchConfig(raw, sourceURL string) (FetchConfig, error) {
	config := FetchConfig{Mode: FetchModeHTML, URL: strings.TrimSpace(sourceURL)}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			return FetchConfig{}, fmt.Errorf("fetch_config 不是有效 JSON: %w", err)
		}
	}
	config.Mode = strings.TrimSpace(config.Mode)
	if config.Mode == "" {
		config.Mode = FetchModeHTML
	}
	config.URL = strings.TrimSpace(config.URL)
	if config.URL == "" {
		config.URL = strings.TrimSpace(sourceURL)
	}
	if config.Mode != FetchModeHTML && config.Mode != FetchModeAPIJSON {
		return FetchConfig{}, fmt.Errorf("不支持的抓取模式: %s", config.Mode)
	}
	validationURL, err := FetchURLValidationTarget(config.URL)
	if err != nil {
		return FetchConfig{}, err
	}
	parsed, err := url.ParseRequestURI(validationURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return FetchConfig{}, fmt.Errorf("数据源 URL 必须是有效的绝对地址")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return FetchConfig{}, fmt.Errorf("数据源 URL 仅支持 http 或 https")
	}
	if config.Mode == FetchModeAPIJSON && strings.TrimSpace(config.ItemsPath) == "" {
		return FetchConfig{}, fmt.Errorf("JSON API 模式必须配置列表路径")
	}
	normalizedHeaders := make(map[string]string, len(config.Headers))
	for name, value := range config.Headers {
		canonical := http.CanonicalHeaderKey(strings.TrimSpace(name))
		switch canonical {
		case "Accept", "Accept-Language", "Referer", "X-Requested-With":
		default:
			return FetchConfig{}, fmt.Errorf("数据源请求头不受支持: %s", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return FetchConfig{}, fmt.Errorf("数据源请求头 %s 包含非法换行", canonical)
		}
		normalizedHeaders[canonical] = strings.TrimSpace(value)
	}
	config.Headers = normalizedHeaders
	for name, variable := range config.Variables {
		if !fetchVariableNamePattern.MatchString(name) {
			return FetchConfig{}, fmt.Errorf("动态参数名称无效: %s", name)
		}
		variable.Source = strings.TrimSpace(variable.Source)
		if variable.Source == "" {
			variable.Source = "html"
		}
		if variable.Source != "html" {
			return FetchConfig{}, fmt.Errorf("动态参数 %s 的来源不受支持: %s", name, variable.Source)
		}
		variable.Selector = strings.TrimSpace(variable.Selector)
		variable.Attr = strings.TrimSpace(variable.Attr)
		if variable.Selector == "" {
			return FetchConfig{}, fmt.Errorf("动态参数 %s 缺少页面选择器", name)
		}
		if _, err := cascadia.Compile(variable.Selector); err != nil {
			return FetchConfig{}, fmt.Errorf("动态参数 %s 的页面选择器无效: %w", name, err)
		}
		config.Variables[name] = variable
	}
	if err := validateFetchTemplateReferences(config); err != nil {
		return FetchConfig{}, err
	}
	return config, nil
}

// FetchURLValidationTarget returns an absolute URL with template values
// replaced by inert text. Placeholders are restricted to path/query/fragment;
// the destination scheme and host must always be explicit in the rule.
func FetchURLValidationTarget(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	match := fetchTemplateVariablePattern.FindStringIndex(raw)
	if match != nil {
		authorityStart := strings.Index(raw, "://")
		if authorityStart < 0 {
			return "", fmt.Errorf("数据源 URL 必须是有效的绝对地址")
		}
		rest := raw[authorityStart+3:]
		authorityEnd := len(raw)
		if offset := strings.IndexAny(rest, "/?#"); offset >= 0 {
			authorityEnd = authorityStart + 3 + offset
		}
		if match[0] <= authorityEnd {
			return "", fmt.Errorf("动态参数不能用于数据源 URL 的协议或主机")
		}
	}
	replaced, err := interpolateFetchTemplate(raw, map[string]string{}, true, true)
	if err != nil {
		return "", err
	}
	return replaced, nil
}

func validateFetchTemplateReferences(config FetchConfig) error {
	known := map[string]struct{}{"page_url": {}}
	for name := range config.Variables {
		known[name] = struct{}{}
	}
	for _, template := range append([]string{config.URL}, headerValues(config.Headers)...) {
		for _, match := range fetchTemplateVariablePattern.FindAllStringSubmatch(template, -1) {
			if _, ok := known[match[1]]; !ok {
				return fmt.Errorf("动态参数 %s 未配置", match[1])
			}
		}
		if strings.Contains(template, "{{") || strings.Contains(template, "}}") {
			cleaned := fetchTemplateVariablePattern.ReplaceAllString(template, "")
			if strings.Contains(cleaned, "{{") || strings.Contains(cleaned, "}}") {
				return fmt.Errorf("动态参数模板格式无效")
			}
		}
	}
	return nil
}

func headerValues(headers map[string]string) []string {
	values := make([]string, 0, len(headers))
	for _, value := range headers {
		values = append(values, value)
	}
	return values
}

func interpolateFetchTemplate(template string, values map[string]string, escape, validation bool) (string, error) {
	var interpolationErr error
	result := fetchTemplateVariablePattern.ReplaceAllStringFunc(template, func(token string) string {
		match := fetchTemplateVariablePattern.FindStringSubmatch(token)
		value, ok := values[match[1]]
		if !ok {
			if validation {
				value = "value"
			} else {
				interpolationErr = fmt.Errorf("动态参数 %s 没有可用值", match[1])
				return token
			}
		}
		if escape {
			return url.QueryEscape(value)
		}
		return value
	})
	if interpolationErr != nil {
		return "", interpolationErr
	}
	return result, nil
}

func CanonicalFetchConfig(raw, sourceURL string) (string, error) {
	config, err := ParseFetchConfig(raw, sourceURL)
	if err != nil {
		return "", err
	}
	if config.Mode == FetchModeHTML && config.URL == strings.TrimSpace(sourceURL) {
		return "", nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func extractSiteResults(ctx context.Context, site *database.Site, client *fetcher.Fetcher, htmlExtractor *Extractor) ([]ExtractResult, error) {
	config, err := ParseFetchConfig(site.FetchConfig, site.URL)
	if err != nil {
		return nil, err
	}
	resolved, err := ResolveFetchConfig(ctx, config, site.URL, client)
	if err != nil {
		return nil, err
	}
	body, err := client.FetchContextWithHeaders(ctx, resolved.URL, resolved.Headers)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	if resolved.Mode == FetchModeAPIJSON {
		return extractJSONResults(body, resolved, site.Fields)
	}
	return htmlExtractor.Extract(body)
}

// ResolveFetchConfig evaluates page variables and returns the concrete API
// request while preserving the rule template passed by the caller.
func ResolveFetchConfig(ctx context.Context, config FetchConfig, sourceURL string, client *fetcher.Fetcher) (FetchConfig, error) {
	values := map[string]string{"page_url": strings.TrimSpace(sourceURL)}
	if len(config.Variables) > 0 {
		pageBody, err := client.FetchContext(ctx, sourceURL)
		if err != nil {
			return FetchConfig{}, fmt.Errorf("抓取动态参数页面失败: %w", err)
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(pageBody))
		if err != nil {
			return FetchConfig{}, fmt.Errorf("解析动态参数页面失败: %w", err)
		}
		names := make([]string, 0, len(config.Variables))
		for name := range config.Variables {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			variable := config.Variables[name]
			selection := doc.Find(variable.Selector).First()
			if selection.Length() == 0 {
				return FetchConfig{}, fmt.Errorf("动态参数 %s 未匹配到页面元素", name)
			}
			var value string
			if variable.Attr != "" {
				var exists bool
				value, exists = selection.Attr(variable.Attr)
				if !exists {
					return FetchConfig{}, fmt.Errorf("动态参数 %s 的元素缺少属性 %s", name, variable.Attr)
				}
			} else {
				value = selection.Text()
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return FetchConfig{}, fmt.Errorf("动态参数 %s 提取结果为空", name)
			}
			values[name] = value
		}
	}

	resolved := config
	resolvedURL, err := interpolateFetchTemplate(config.URL, values, true, false)
	if err != nil {
		return FetchConfig{}, err
	}
	resolved.URL = resolvedURL
	if _, err := FetchURLValidationTarget(resolved.URL); err != nil {
		return FetchConfig{}, err
	}
	resolved.Headers = make(map[string]string, len(config.Headers))
	for name, value := range config.Headers {
		interpolated, err := interpolateFetchTemplate(value, values, false, false)
		if err != nil {
			return FetchConfig{}, err
		}
		if strings.ContainsAny(interpolated, "\r\n") {
			return FetchConfig{}, fmt.Errorf("动态参数使请求头 %s 包含非法换行", name)
		}
		resolved.Headers[name] = interpolated
	}
	return resolved, nil
}

// ExtractConfiguredSource runs a saved extraction definition without invoking
// detection or persistence. It is used by rule tests and scan previews.
func ExtractConfiguredSource(ctx context.Context, site *database.Site) ([]ExtractResult, error) {
	selectors := SiteSelectors{Container: site.Container, Item: site.Item, Fields: make([]FieldConfig, len(site.Fields))}
	for index, field := range site.Fields {
		selectors.Fields[index] = FieldConfig{
			Name: field.Name, Selector: field.Selector, Type: field.Type,
			Attr: field.Attr, Transform: field.Transform,
		}
	}
	return extractSiteResults(ctx, site, fetcher.New(), NewExtractor(selectors))
}

func extractJSONResults(body string, config FetchConfig, fields []database.SiteField) ([]ExtractResult, error) {
	var document interface{}
	if err := json.Unmarshal([]byte(body), &document); err != nil {
		return nil, fmt.Errorf("解析 JSON 响应失败: %w", err)
	}
	itemsValue, ok := lookupJSONPath(document, config.ItemsPath)
	if !ok {
		return nil, fmt.Errorf("JSON 列表路径 %q 未匹配到数据", config.ItemsPath)
	}
	items, ok := itemsValue.([]interface{})
	if !ok {
		return nil, fmt.Errorf("JSON 列表路径 %q 不是数组", config.ItemsPath)
	}
	results := make([]ExtractResult, 0, len(items))
	for _, item := range items {
		if config.FilterPath != "" {
			value, exists := lookupJSONPath(item, config.FilterPath)
			if !exists || !strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), strings.TrimSpace(config.FilterEquals)) {
				continue
			}
		}
		result := make(ExtractResult, len(fields))
		for _, field := range fields {
			value, exists := lookupJSONPath(item, field.Selector)
			if !exists {
				continue
			}
			text := stringifyJSONValue(value)
			if field.Transform != "" {
				text = applyTransform(text, field.Transform)
			}
			result[field.Name] = text
		}
		if len(result) > 0 {
			results = append(results, result)
		}
	}
	return results, nil
}

func lookupJSONPath(value interface{}, rawPath string) (interface{}, bool) {
	path := strings.Trim(strings.TrimSpace(rawPath), ".")
	if path == "" || path == "$" {
		return value, true
	}
	path = strings.TrimPrefix(path, "$.")
	parts := strings.Split(path, ".")
	return lookupJSONParts(value, parts)
}

func lookupJSONParts(value interface{}, parts []string) (interface{}, bool) {
	if len(parts) == 0 {
		return value, true
	}
	switch current := value.(type) {
	case map[string]interface{}:
		next, ok := current[parts[0]]
		if !ok {
			return nil, false
		}
		return lookupJSONParts(next, parts[1:])
	case []interface{}:
		if parts[0] != "*" {
			return nil, false
		}
		values := make([]interface{}, 0, len(current))
		for _, entry := range current {
			if found, ok := lookupJSONParts(entry, parts[1:]); ok {
				values = append(values, found)
			}
		}
		return values, true
	default:
		return nil, false
	}
}

func stringifyJSONValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []interface{}:
		parts := make([]string, 0, len(typed))
		for _, entry := range typed {
			if text := strings.TrimSpace(stringifyJSONValue(entry)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " / ")
	case map[string]interface{}:
		data, _ := json.Marshal(typed)
		return string(data)
	default:
		return fmt.Sprint(typed)
	}
}
