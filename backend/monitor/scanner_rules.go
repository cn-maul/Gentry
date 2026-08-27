package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/cn-maul/Gentry/database"
)

// htmlScanRule 是一条可对 HTML 页面应用的扫描规则（数据库模板或外部规则文件）。
type htmlScanRule struct {
	name      string
	address   string
	container string
	item      string
	priority  int
	fields    []database.ScanRuleField
	matcher   ScopeMatcher
}

// ScopeMatcher 判断一条扫描规则是否命中给定 URL。
// 四种适用范围各自对应一个无状态实现，由 NewScopeMatcher 按规则构造。
type ScopeMatcher interface {
	MatchesURL(rawURL string) bool
}

// NewScopeMatcher 依据规则的适用范围返回对应的匹配器；
// ScopeType 为空的遗留规则退化为 URLContains 子串匹配。
func NewScopeMatcher(rule database.ScanRuleTemplate) ScopeMatcher {
	switch rule.ScopeType {
	case ScanRuleScopeGlobal:
		return globalMatcher{}
	case ScanRuleScopeExact, ScanRuleScopeRoute, ScanRuleScopeSection:
		return scopedMatcher{
			scopeType: rule.ScopeType,
			host:      rule.MatchHost,
			matchPath: rule.MatchPath,
			query:     rule.MatchQuery,
		}
	default:
		if rule.ScopeType != "" {
			return neverMatcher{}
		}
		return legacyContainsMatcher{needle: strings.TrimSpace(rule.URLContains)}
	}
}

// globalMatcher 通用结构规则：不限制 URL，只有页面实际存在选择器时才生成候选。
type globalMatcher struct{}

func (globalMatcher) MatchesURL(string) bool { return true }

// neverMatcher 兜底未知范围：永不命中，避免误扩大匹配面。
type neverMatcher struct{}

func (neverMatcher) MatchesURL(string) bool { return false }

// scopedMatcher 覆盖 exact/route/section 三种基于规范化 URL 的精确匹配：
// 要求同主机；exact 全等 path+query，route/section 为路径段前缀且 query 为空或全等。
type scopedMatcher struct {
	scopeType string
	host      string
	matchPath string
	query     string
}

func (m scopedMatcher) MatchesURL(rawURL string) bool {
	normalized, err := normalizeScanRuleURL(rawURL)
	if err != nil {
		return false
	}
	if m.host == "" || !strings.EqualFold(m.host, normalized.host) {
		return false
	}
	if m.scopeType == ScanRuleScopeExact {
		return m.matchPath == normalized.path && m.query == normalized.query
	}
	basePath := strings.TrimSuffix(m.matchPath, "/")
	pathMatches := normalized.path == m.matchPath
	if m.matchPath != "/" {
		pathMatches = pathMatches || strings.HasPrefix(normalized.path, basePath+"/")
	}
	queryMatches := m.query == "" || m.query == normalized.query
	return pathMatches && queryMatches
}

// legacyContainsMatcher 遗留 URLContains 规则：忽略大小写子串匹配。
type legacyContainsMatcher struct {
	needle string
}

func (m legacyContainsMatcher) MatchesURL(rawURL string) bool {
	if m.needle == "" {
		return false
	}
	return strings.Contains(strings.ToLower(rawURL), strings.ToLower(m.needle))
}

const (
	ScanRuleScopeExact   = "exact"
	ScanRuleScopeRoute   = "route"
	ScanRuleScopeSection = "section"
	ScanRuleScopeGlobal  = "global"
)

type normalizedScanRuleURL struct {
	source string
	host   string
	path   string
	query  string
}

func normalizeScanRuleURL(raw string) (normalizedScanRuleURL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return normalizedScanRuleURL{}, fmt.Errorf("URL 必须包含协议和主机")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return normalizedScanRuleURL{}, fmt.Errorf("仅支持 HTTP 或 HTTPS URL")
	}

	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "" {
		return normalizedScanRuleURL{}, fmt.Errorf("URL 主机无效")
	}
	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	matchHost := hostname
	if port != "" {
		matchHost = net.JoinHostPort(hostname, port)
	}

	matchPath := parsed.Path
	if matchPath == "" {
		matchPath = "/"
	} else {
		matchPath = path.Clean("/" + strings.TrimPrefix(matchPath, "/"))
	}
	matchQuery := parsed.Query().Encode()

	parsed.Scheme = scheme
	parsed.User = nil
	parsed.Host = matchHost
	if strings.Contains(hostname, ":") && port == "" {
		parsed.Host = "[" + hostname + "]"
	}
	parsed.Path = matchPath
	parsed.RawPath = ""
	parsed.RawQuery = matchQuery
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""

	return normalizedScanRuleURL{
		source: parsed.String(),
		host:   matchHost,
		path:   matchPath,
		query:  matchQuery,
	}, nil
}

// ApplyScanRuleScope derives matching fields from a trusted source URL. Route
// scopes are path-segment prefixes and keep query constraints when present.
func ApplyScanRuleScope(rule *database.ScanRuleTemplate, rawURL, scopeType string) error {
	if scopeType == "" {
		scopeType = ScanRuleScopeExact
	}
	if scopeType == ScanRuleScopeGlobal {
		rule.ScopeType = ScanRuleScopeGlobal
		rule.SourceURL = strings.TrimSpace(rawURL)
		rule.MatchHost = ""
		rule.MatchPath = ""
		rule.MatchQuery = ""
		return nil
	}
	if scopeType != ScanRuleScopeExact && scopeType != ScanRuleScopeRoute && scopeType != ScanRuleScopeSection {
		return fmt.Errorf("不支持的规则适用范围 %q", scopeType)
	}
	normalized, err := normalizeScanRuleURL(rawURL)
	if err != nil {
		return err
	}
	if scopeType == ScanRuleScopeRoute && normalized.path == "/" && normalized.query == "" {
		return fmt.Errorf("根路径不能创建路由规则，请选择当前页面或通用结构")
	}
	if scopeType == ScanRuleScopeSection {
		sectionPath := path.Dir(normalized.path)
		if sectionPath == "." || sectionPath == "/" {
			return fmt.Errorf("当前页面没有可复用的同站目录")
		}
		normalized.path = sectionPath
		normalized.query = ""
	}
	rule.SourceURL = normalized.source
	rule.ScopeType = scopeType
	rule.MatchHost = normalized.host
	rule.MatchPath = normalized.path
	rule.MatchQuery = normalized.query
	return nil
}

// ScanRuleMatchesURL keeps legacy URLContains rules working while scoped rules
// use normalized host, path and query equality.
func ScanRuleMatchesURL(rule database.ScanRuleTemplate, rawURL string) bool {
	return NewScopeMatcher(rule).MatchesURL(rawURL)
}

// userTemplateRules 加载数据库中启用的扫描规则模板。
func userTemplateRules() []htmlScanRule {
	db := database.GetDB()
	if db == nil {
		return nil
	}
	var templates []database.ScanRuleTemplate
	if err := db.Preload("Fields").Where("enabled = ?", true).Order("priority desc, id asc").Find(&templates).Error; err != nil {
		log.Printf("[ScannerRules] 加载数据库扫描规则模板失败: %v", err)
		return nil
	}
	result := make([]htmlScanRule, 0, len(templates))
	for _, tpl := range templates {
		template := tpl
		result = append(result, htmlScanRule{
			name:      "template_" + template.Name,
			address:   templateDisplayAddress(template),
			container: template.Container,
			item:      template.Item,
			priority:  max(70, template.Priority),
			fields:    template.Fields,
			matcher:   NewScopeMatcher(template),
		})
	}
	return result
}

// templateDisplayAddress 规则「适用地址」的展示文案：通用结构规则展示为
// 全局说明；带来源地址的展示来源地址；否则退化为 URLContains 文案。
func templateDisplayAddress(template database.ScanRuleTemplate) string {
	if template.ScopeType == ScanRuleScopeGlobal {
		return "所有网站中结构相同的页面"
	}
	if source := strings.TrimSpace(template.SourceURL); source != "" {
		return source
	}
	if contains := strings.TrimSpace(template.URLContains); contains != "" {
		return "URL 包含 " + contains
	}
	return ""
}

type externalScanRuleFile struct {
	Version int                `json:"version"`
	Rules   []externalScanRule `json:"rules"`
}

type externalScanRule struct {
	Name              string `json:"name"`
	URLContains       string `json:"url_contains"`
	ContainerSelector string `json:"container_selector"`
	ItemSelector      string `json:"item_selector"`
	Priority          int    `json:"priority"`
}

var runtimeScanRules []htmlScanRule

func loadExternalScanRules(path string) []htmlScanRule {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[ScannerRules] 读取外部规则失败: %v", err)
		return nil
	}
	var file externalScanRuleFile
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("[ScannerRules] 解析外部规则失败: %v", err)
		return nil
	}
	var rules []htmlScanRule
	for _, rule := range file.Rules {
		if rule.Name == "" || rule.URLContains == "" || rule.ContainerSelector == "" || rule.ItemSelector == "" {
			log.Printf("[ScannerRules] 跳过无效规则: %+v", rule)
			continue
		}
		priority := rule.Priority
		if priority <= 0 {
			priority = 60
		}
		rules = append(rules, htmlScanRule{
			name:      "rule_" + rule.Name,
			address:   "URL 包含 " + rule.URLContains,
			container: rule.ContainerSelector,
			item:      rule.ItemSelector,
			priority:  priority,
			matcher:   legacyContainsMatcher{needle: strings.ToLower(rule.URLContains)},
		})
	}
	log.Printf("[ScannerRules] 已从 %s 加载 %d 条外部规则", path, len(rules))
	return rules
}

func InitScanRules(externalPath string) {
	runtimeScanRules = loadExternalScanRules(externalPath)
	log.Printf("[ScannerRules] 已加载 %d 条外部扫描规则", len(runtimeScanRules))
}

// matchingHTMLScanRules 汇总数据库模板与外部规则，过滤出命中 URL 的规则，
// 并按优先级降序返回（稳定排序保持加载顺序）。
func matchingHTMLScanRules(pageURL string) []htmlScanRule {
	all := append(userTemplateRules(), runtimeScanRules...)
	matched := make([]htmlScanRule, 0, len(all))
	for _, rule := range all {
		if rule.matcher != nil && rule.matcher.MatchesURL(pageURL) {
			matched = append(matched, rule)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].priority > matched[j].priority })
	return matched
}
