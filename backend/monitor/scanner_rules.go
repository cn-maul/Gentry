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
	name       string
	container  string
	item       string
	priority   int
	fields     []database.ScanRuleField
	matchesURL func(string) bool
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
	if rule.ScopeType == ScanRuleScopeGlobal {
		return true
	}
	if rule.ScopeType == ScanRuleScopeExact || rule.ScopeType == ScanRuleScopeRoute || rule.ScopeType == ScanRuleScopeSection {
		normalized, err := normalizeScanRuleURL(rawURL)
		if err != nil {
			return false
		}
		if rule.MatchHost == "" || !strings.EqualFold(rule.MatchHost, normalized.host) {
			return false
		}
		if rule.ScopeType == ScanRuleScopeExact {
			return rule.MatchPath == normalized.path && rule.MatchQuery == normalized.query
		}
		basePath := strings.TrimSuffix(rule.MatchPath, "/")
		pathMatches := normalized.path == rule.MatchPath
		if rule.MatchPath != "/" {
			pathMatches = pathMatches || strings.HasPrefix(normalized.path, basePath+"/")
		}
		queryMatches := rule.MatchQuery == "" || rule.MatchQuery == normalized.query
		return pathMatches && queryMatches
	}
	if rule.ScopeType != "" {
		return false
	}
	needle := strings.TrimSpace(rule.URLContains)
	return needle != "" && strings.Contains(strings.ToLower(rawURL), strings.ToLower(needle))
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
			name:       "template_" + template.Name,
			container:  template.Container,
			item:       template.Item,
			priority:   max(70, template.Priority),
			fields:     template.Fields,
			matchesURL: func(rawURL string) bool { return ScanRuleMatchesURL(template, rawURL) },
		})
	}
	return result
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
		needle := strings.ToLower(rule.URLContains)
		rules = append(rules, htmlScanRule{
			name:       "rule_" + rule.Name,
			container:  rule.ContainerSelector,
			item:       rule.ItemSelector,
			priority:   priority,
			matchesURL: func(rawURL string) bool { return strings.Contains(strings.ToLower(rawURL), needle) },
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
		if rule.matchesURL != nil && rule.matchesURL(pageURL) {
			matched = append(matched, rule)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].priority > matched[j].priority })
	return matched
}
