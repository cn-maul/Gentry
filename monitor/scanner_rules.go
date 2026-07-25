package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/cn-maul/Gentry/database"
)

type scanSiteRule struct {
	name       string
	urlPattern string
	matchesURL func(string) bool
	build      func(doc *goquery.Document, settings *ScanSettings) []scanStrategyResult
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

// ApplyExactScanRuleScope is kept as a convenience for callers that only need
// a page-isolated rule.
func ApplyExactScanRuleScope(rule *database.ScanRuleTemplate, rawURL string) error {
	return ApplyScanRuleScope(rule, rawURL, ScanRuleScopeExact)
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

type externalScanRuleFile struct {
	Version int                `json:"version"`
	Rules   []externalScanRule `json:"rules"`
}

type externalScanRule struct {
	Name              string   `json:"name"`
	URLContains       string   `json:"url_contains"`
	ContainerSelector string   `json:"container_selector"`
	ItemSelector      string   `json:"item_selector"`
	Priority          int      `json:"priority"`
	Diagnostics       []string `json:"diagnostics"`
}

var runtimeScanRules []scanSiteRule

func matchCount(items []ExtractResult, keywords []string) int {
	count := 0
	for _, item := range items {
		if matchKeywords(item, keywords) {
			count++
		}
	}
	return count
}

func buildRuleStrategyResult(name string, container *goquery.Selection, items []ExtractResult, keywords []string, diagnostics ...string) []scanStrategyResult {
	if container == nil || container.Length() == 0 {
		return nil
	}
	// 即使关键词无命中，也保留策略结果，让扫描规则模板和结构化策略能继续参与候选生成
	hits := matchCount(items, keywords)
	return []scanStrategyResult{{
		name:        name,
		container:   container,
		hits:        max(1, hits),
		diagnostics: diagnostics,
		priority:    60,
	}}
}

func buildSelectorRuleStrategyResult(sourceLabel, name, urlPattern, containerSelector, itemSelector string, priority int, diagnostics []string, fields []database.ScanRuleField) scanSiteRule {
	if priority <= 0 {
		priority = 60
	}
	return scanSiteRule{
		name:       name,
		urlPattern: urlPattern,
		build: func(doc *goquery.Document, settings *ScanSettings) []scanStrategyResult {
			container := doc.Find(containerSelector).First()
			if container.Length() == 0 {
				return nil
			}
			var items []ExtractResult
			container.Find(itemSelector).Each(func(_ int, item *goquery.Selection) {
				text := strings.TrimSpace(item.Text())
				if text == "" {
					return
				}
				entry := ExtractResult{"title": text}
				firstLink := item.Find("a[href]").First()
				if firstLink.Length() > 0 {
					if href, exists := firstLink.Attr("href"); exists {
						entry["url"] = href
					}
				}
				items = append(items, entry)
			})
			results := buildRuleStrategyResult(name, container, items, settings.Keywords, append([]string{sourceLabel}, diagnostics...)...)
			if len(results) > 0 {
				results[0].priority = priority
				results[0].containerSelector = containerSelector
				results[0].itemSelector = itemSelector
				// 即使模板没有定义额外字段，也记录 selector，让后续逻辑能应用模板的 selector
				results[0].fields = fields
			}
			return results
		},
	}
}

func buildUserTemplateRules() []scanSiteRule {
	db := database.GetDB()
	if db == nil {
		return nil
	}
	var templates []database.ScanRuleTemplate
	if err := db.Preload("Fields").Where("enabled = ?", true).Order("priority desc, id asc").Find(&templates).Error; err != nil {
		log.Printf("[ScannerRules] 加载数据库扫描规则模板失败: %v", err)
		return nil
	}
	result := make([]scanSiteRule, 0, len(templates))
	for _, tpl := range templates {
		diagnostics := []string{"命中用户扫描规则模板"}
		if tpl.Description != "" {
			diagnostics = append(diagnostics, tpl.Description)
		}
		rule := buildSelectorRuleStrategyResult("命中用户扫描规则模板", "template_"+tpl.Name, tpl.URLContains, tpl.Container, tpl.Item, max(70, tpl.Priority), diagnostics, tpl.Fields)
		template := tpl
		rule.matchesURL = func(rawURL string) bool {
			return ScanRuleMatchesURL(template, rawURL)
		}
		result = append(result, rule)
	}
	return result
}

func loadExternalScanRules(path string) []scanSiteRule {
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
	var rules []scanSiteRule
	for _, rule := range file.Rules {
		if rule.Name == "" || rule.URLContains == "" || rule.ContainerSelector == "" || rule.ItemSelector == "" {
			log.Printf("[ScannerRules] 跳过无效规则: %+v", rule)
			continue
		}
		rules = append(rules, buildSelectorRuleStrategyResult("命中外部规则", rule.Name, rule.URLContains, rule.ContainerSelector, rule.ItemSelector, rule.Priority, rule.Diagnostics, nil))
	}
	log.Printf("[ScannerRules] 已从 %s 加载 %d 条外部规则", path, len(rules))
	return rules
}

func InitScanRules(externalPath string) {
	runtimeScanRules = loadExternalScanRules(externalPath)
	log.Printf("[ScannerRules] 已加载 %d 条外部扫描规则", len(runtimeScanRules))
}

func CurrentScanRules() []scanSiteRule {
	return runtimeScanRules
}
