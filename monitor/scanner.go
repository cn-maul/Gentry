package monitor

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/fetcher"
)

type scanContainerKey struct {
	tag      string
	selector string
}

type scanContainerEntry struct {
	key      scanContainerKey
	parent   *goquery.Selection
	items    []ExtractResult
	itemsSel []*goquery.Selection
	hits     int
}

type ScanResult struct {
	URL        string          `json:"url"`
	Containers []ContainerInfo `json:"containers"`
}

type ContainerInfo struct {
	Selector     string            `json:"selector"`
	ContainerTag string            `json:"container_tag"`
	ContainerCSS string            `json:"container_css"`
	ItemTag      string            `json:"item_tag"`
	ItemCSS      string            `json:"item_css"`
	ItemCount    int               `json:"item_count"`
	KeywordHits  int               `json:"keyword_hits"`
	SampleItems  []ExtractResult   `json:"sample_items"`
	Config       ScanMonitorConfig `json:"config"`
	Strategy     string            `json:"strategy,omitempty"`
	Confidence   int               `json:"confidence,omitempty"`
	Diagnostics  []string          `json:"diagnostics,omitempty"`
}

type ScanMonitorConfig struct {
	Container string            `json:"container"`
	Item      string            `json:"item"`
	Fields    []ScanFieldConfig `json:"fields"`
	Fetch     *FetchConfig      `json:"fetch_config,omitempty"`
}

type ScanFieldConfig struct {
	Name      string `json:"name"`
	Selector  string `json:"selector"`
	Type      string `json:"type"`
	Attr      string `json:"attr,omitempty"`
	Transform string `json:"transform,omitempty"`
}

type ScanSettings struct {
	URL          string   `json:"url"`
	Keywords     []string `json:"keywords"`
	StrategyType string   `json:"strategy_type,omitempty"`
}

type scanStrategyResult struct {
	name              string
	container         *goquery.Selection
	hits              int
	diagnostics       []string
	priority          int
	fields            []database.ScanRuleField
	containerSelector string
	itemSelector      string
}

type match struct {
	sel     *goquery.Selection
	keyword string
}

var lowValueLinkTexts = []string{
	"首页", "更多", "上一页", "下一页", "登录", "注册", "返回", "联系我们", "无障碍",
}

func buildDeepSelector(sel *goquery.Selection) string {
	parts := []string{}
	current := sel
	depth := 0
	for current != nil && depth < 3 {
		if current.Is("html") || current.Is("body") {
			break
		}
		tag := goquery.NodeName(current)
		s := tag
		if id, exists := current.Attr("id"); exists && id != "" {
			s = "#" + id
		} else if class, exists := current.Attr("class"); exists && class != "" {
			classes := strings.Fields(class)
			for _, c := range classes {
				if len(c) > 1 && !strings.HasPrefix(c, "ng-") && !strings.HasPrefix(c, "_") {
					s = tag + "." + c
					break
				}
			}
		}
		parts = append([]string{s}, parts...)
		current = current.Parent()
		depth++
	}
	return strings.Join(parts, " > ")
}

func isLikelyNoiseContainer(sel *goquery.Selection) bool {
	if sel == nil || sel.Length() == 0 {
		return false
	}
	attrs := []string{}
	if id, ok := sel.Attr("id"); ok {
		attrs = append(attrs, id)
	}
	if class, ok := sel.Attr("class"); ok {
		attrs = append(attrs, class)
	}
	for _, raw := range attrs {
		lower := strings.ToLower(raw)
		if strings.Contains(lower, "footer") || strings.Contains(lower, "header") ||
			strings.Contains(lower, "nav") || strings.Contains(lower, "sidebar") ||
			strings.Contains(lower, "menu") || strings.Contains(lower, "banner") ||
			strings.Contains(lower, "breadcrumb") || strings.Contains(lower, "crumb") {
			return true
		}
	}
	return false
}

func isLowValueLinkText(text string) bool {
	text = strings.TrimSpace(text)
	for _, keyword := range lowValueLinkTexts {
		if text == keyword {
			return true
		}
	}
	return false
}

func collectKeywordMatches(doc *goquery.Document, keywords []string) []match {
	var matches []match
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		kw = strings.TrimSpace(kw)
		doc.Find("body *").Each(func(_ int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text == "" {
				return
			}
			tag := goquery.NodeName(s)
			if tag == "script" || tag == "style" || tag == "noscript" {
				return
			}
			if strings.Contains(text, kw) {
				matches = append(matches, match{sel: s, keyword: kw})
			}
		})
	}
	return matches
}

func keywordAncestorStrategy(doc *goquery.Document, matches []match) []scanStrategyResult {
	var results []scanStrategyResult
	for _, m := range matches {
		container := findBestContainer(m.sel, doc)
		if container == nil {
			continue
		}
		results = append(results, scanStrategyResult{
			name:        "keyword_ancestor",
			container:   container,
			hits:        1,
			diagnostics: []string{"通过关键词命中向上定位候选容器"},
			priority:    40,
		})
	}
	return results
}

func repeatedListStrategy(doc *goquery.Document) []scanStrategyResult {
	var results []scanStrategyResult
	doc.Find("ul, ol, table, tbody, dl, section, div, main").Each(func(_ int, sel *goquery.Selection) {
		if isLikelyNoiseContainer(sel) {
			return
		}
		itemTag, itemCSS := detectItemPattern(sel)
		if itemTag == "" && itemCSS == "" {
			return
		}
		count := 0
		if itemCSS != "" {
			count = sel.ChildrenFiltered(itemCSS).Length()
		}
		if count == 0 && itemTag != "" {
			count = sel.ChildrenFiltered(itemTag).Length()
		}
		if count < 3 {
			return
		}
		results = append(results, scanStrategyResult{
			name:        "repeated_list",
			container:   sel,
			hits:        count,
			diagnostics: []string{fmt.Sprintf("检测到 %d 个重复子项", count)},
			priority:    30,
		})
	})
	return results
}

func linkClusterStrategy(doc *goquery.Document) []scanStrategyResult {
	var results []scanStrategyResult
	doc.Find("div, section, main, aside").Each(func(_ int, sel *goquery.Selection) {
		if isLikelyNoiseContainer(sel) {
			return
		}
		links := sel.Find("a")
		if links.Length() < 3 {
			return
		}
		directLinks := sel.ChildrenFiltered("a")
		if directLinks.Length() < 3 {
			return
		}
		lowValue := 0
		nonEmpty := 0
		directLinks.Each(func(_ int, a *goquery.Selection) {
			text := strings.TrimSpace(a.Text())
			if text == "" {
				return
			}
			nonEmpty++
			if isLowValueLinkText(text) {
				lowValue++
			}
		})
		if nonEmpty < 3 || lowValue*2 >= nonEmpty {
			return
		}
		results = append(results, scanStrategyResult{
			name:        "link_cluster",
			container:   sel,
			hits:        directLinks.Length(),
			diagnostics: []string{fmt.Sprintf("检测到 %d 个直系链接子项", directLinks.Length())},
			priority:    20,
		})
	})
	return results
}

func tableStrategy(doc *goquery.Document) []scanStrategyResult {
	var results []scanStrategyResult
	doc.Find("table, tbody").Each(func(_ int, sel *goquery.Selection) {
		if isLikelyNoiseContainer(sel) {
			return
		}
		rows := sel.Find("tr")
		if rows.Length() < 2 {
			return
		}
		linkRows := 0
		rows.Each(func(_ int, row *goquery.Selection) {
			if row.Find("a").Length() > 0 {
				linkRows++
			}
		})
		if linkRows < 2 {
			return
		}
		results = append(results, scanStrategyResult{
			name:        "table_rows",
			container:   sel,
			hits:        linkRows,
			diagnostics: []string{fmt.Sprintf("检测到 %d 行含链接表格数据", linkRows)},
			priority:    35,
		})
	})
	return results
}

func primaryStrategy(current string, currentPriority int, incoming string, incomingPriority int) string {
	if current == "" || incomingPriority > currentPriority {
		return incoming
	}
	return current
}

func scanRuleStrategies(doc *goquery.Document, settings *ScanSettings) []scanStrategyResult {
	var results []scanStrategyResult
	settingsURL := ""
	if settings != nil {
		settingsURL = settings.URL
	}
	for _, rule := range append(buildUserTemplateRules(), CurrentScanRules()...) {
		if rule.matchesURL != nil && !rule.matchesURL(settingsURL) {
			continue
		}
		if rule.matchesURL == nil && rule.urlPattern != "" && !strings.Contains(strings.ToLower(settingsURL), strings.ToLower(rule.urlPattern)) {
			continue
		}
		results = append(results, rule.build(doc, settings)...)
	}
	return results
}

func smartScanHTMLWithSettings(html string, settings *ScanSettings) (*ScanResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	matches := collectKeywordMatches(doc, settings.Keywords)

	containerMap := make(map[scanContainerKey]*scanContainerEntry)
	var containerOrder []scanContainerKey
	strategyByKey := make(map[scanContainerKey]string)
	strategyPriorityByKey := make(map[scanContainerKey]int)
	diagnosticsByKey := make(map[scanContainerKey][]string)
	fieldsByKey := make(map[scanContainerKey][]database.ScanRuleField)
	containerSelByKey := make(map[scanContainerKey]string)
	itemSelByKey := make(map[scanContainerKey]string)

	// 先运行扫描规则（用户模板 + 内置规则），它们不依赖关键词
	strategyResults := scanRuleStrategies(doc, settings)
	// 运行不依赖关键词的启发式策略
	strategyResults = append(strategyResults, repeatedListStrategy(doc)...)
	strategyResults = append(strategyResults, linkClusterStrategy(doc)...)
	strategyResults = append(strategyResults, tableStrategy(doc)...)
	// 关键词依赖的策略仅在有关键词命中时运行
	if len(matches) > 0 {
		strategyResults = append(strategyResults, keywordAncestorStrategy(doc, matches)...)
	}

	if len(strategyResults) == 0 {
		return &ScanResult{URL: settings.URL}, nil
	}
	for _, sr := range strategyResults {
		container := sr.container
		if container == nil {
			continue
		}
		tag := goquery.NodeName(container)
		sel := buildDeepSelector(container)
		if sel == "" {
			continue
		}
		key := scanContainerKey{tag: tag, selector: sel}
		if _, exists := containerMap[key]; !exists {
			containerMap[key] = &scanContainerEntry{key: key, parent: container}
			containerOrder = append(containerOrder, key)
			strategyByKey[key] = sr.name
			strategyPriorityByKey[key] = sr.priority
			diagnosticsByKey[key] = append([]string{}, sr.diagnostics...)
			if len(sr.fields) > 0 {
				fieldsByKey[key] = sr.fields
			}
			if sr.containerSelector != "" {
				containerSelByKey[key] = sr.containerSelector
			}
			if sr.itemSelector != "" {
				itemSelByKey[key] = sr.itemSelector
			}
		} else {
			if strategyByKey[key] != sr.name {
				diagnosticsByKey[key] = append(diagnosticsByKey[key], fmt.Sprintf("同时命中策略 %s", sr.name))
			}
			strategyByKey[key] = primaryStrategy(strategyByKey[key], strategyPriorityByKey[key], sr.name, sr.priority)
			if sr.priority > strategyPriorityByKey[key] {
				strategyPriorityByKey[key] = sr.priority
			}
		}
		containerMap[key].hits += sr.hits
	}

	for _, key := range containerOrder {
		entry := containerMap[key]
		extractContainerItems(entry)
	}

	sortScanContainers(containerOrder, containerMap, strategyByKey)
	maxContainers := 3
	if len(containerOrder) > maxContainers {
		containerOrder = containerOrder[:maxContainers]
	}

	result := &ScanResult{URL: settings.URL}
	for _, key := range containerOrder {
		entry := containerMap[key]
		css := buildShortSelector(entry.parent, entry.key.tag)
		itemTag, itemCSS := detectItemPattern(entry.parent)
		var config ScanMonitorConfig
		if strategyFields, ok := fieldsByKey[key]; ok && len(strategyFields) > 0 {
			cfgContainer := css
			cfgItem := itemCSS
			if cs, ok := containerSelByKey[key]; ok && cs != "" {
				cfgContainer = cs
			}
			if is, ok := itemSelByKey[key]; ok && is != "" {
				cfgItem = is
			}
			config = ScanMonitorConfig{
				Container: cfgContainer,
				Item:      cfgItem,
				Fields:    scanRuleFieldsToConfigFields(strategyFields),
			}
		} else {
			// 即使没有自定义字段（仅有 title），也要优先使用模板的容器/item selector
			cfgContainer := css
			cfgItem := itemCSS
			if cs, ok := containerSelByKey[key]; ok && cs != "" {
				cfgContainer = cs
			}
			if is, ok := itemSelByKey[key]; ok && is != "" {
				cfgItem = is
			}
			config = ScanMonitorConfig{
				Container: cfgContainer,
				Item:      cfgItem,
				Fields:    buildScanConfig(entry.parent, cfgItem, entry.items).Fields,
			}
		}
		strategy := strategyByKey[key]
		if strategy == "" {
			strategy = "keyword_ancestor"
		}
		configItems, configErr := NewExtractor(ScanConfigToSelectors(config)).Extract(html)
		if configErr != nil {
			diagnosticsByKey[key] = append(diagnosticsByKey[key], fmt.Sprintf("保存配置验证失败: %v", configErr))
		}
		if settings.URL != "" {
			if err := ResolveExtractedURLs(settings.URL, configItems); err != nil {
				diagnosticsByKey[key] = append(diagnosticsByKey[key], fmt.Sprintf("链接解析失败: %v", err))
			}
		}
		displayItems := configItems
		if len(displayItems) == 0 {
			diagnosticsByKey[key] = append(diagnosticsByKey[key], "保存配置未提取到样本项")
		}

		info := ContainerInfo{
			Selector:     entry.key.selector,
			ContainerTag: entry.key.tag,
			ContainerCSS: css,
			ItemTag:      itemTag,
			ItemCSS:      itemCSS,
			ItemCount:    len(displayItems),
			KeywordHits:  entry.hits,
			Config:       config,
			Strategy:     strategy,
			Confidence:   candidateConfidence(entry, strategy),
			Diagnostics:  append([]string{}, diagnosticsByKey[key]...),
		}
		info.Diagnostics = append(info.Diagnostics, fmt.Sprintf("提取到 %d 个样本项", len(displayItems)))
		if len(displayItems) > 10 {
			info.SampleItems = displayItems[:10]
		} else if len(displayItems) > 0 {
			info.SampleItems = displayItems
		} else {
			info.SampleItems = []ExtractResult{}
		}
		result.Containers = append(result.Containers, info)
	}
	return result, nil
}

func smartScanHTML(html string, keywords []string) (*ScanResult, error) {
	return smartScanHTMLWithSettings(html, &ScanSettings{Keywords: keywords})
}

func SmartScan(settings *ScanSettings) (*ScanResult, error) {
	if settings == nil {
		return nil, fmt.Errorf("扫描配置不能为空")
	}
	f := fetcher.New()
	apiCandidates := scanJSONTemplateCandidates(settings, f)
	// Explicit API rules are deterministic and do not need the monitored page
	// itself to be fetchable. This also lets API-backed presence monitors reuse
	// exactly the same scan-rule library.
	if len(apiCandidates) > 0 {
		return &ScanResult{URL: settings.URL, Containers: apiCandidates}, nil
	}
	html, err := f.Fetch(settings.URL)
	if err != nil {
		return nil, fmt.Errorf("抓取页面失败: %w", err)
	}

	htmlResult, err := smartScanHTMLWithSettings(html, settings)
	if err != nil {
		return nil, err
	}
	if settings.StrategyType == "field_transition" {
		var ruleCandidates []ContainerInfo
		for _, candidate := range htmlResult.Containers {
			if strings.HasPrefix(candidate.Strategy, "template_") || strings.HasPrefix(candidate.Strategy, "rule_") {
				ruleCandidates = append(ruleCandidates, candidate)
			}
		}
		if len(ruleCandidates) > 0 {
			htmlResult.Containers = ruleCandidates
		}
	}
	return htmlResult, nil
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
		candidate, err := buildJSONScanCandidate("template_"+template.Name, "命中用户扫描规则模板", config, settings.URL, template.Fields, client)
		if err != nil {
			log.Printf("[ScannerRules] JSON API 规则「%s」扫描失败: %v", template.Name, err)
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func buildJSONScanCandidate(strategy, diagnostic string, config FetchConfig, sourceURL string, fields []database.ScanRuleField, client *fetcher.Fetcher) (ContainerInfo, error) {
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
		Selector: config.ItemsPath, ContainerTag: "JSON", ContainerCSS: config.ItemsPath,
		ItemTag: "OBJECT", ItemCSS: "*", ItemCount: len(items), KeywordHits: 1,
		SampleItems: items, Strategy: strategy, Confidence: 100,
		Diagnostics: []string{diagnostic, fmt.Sprintf("JSON API 提取到 %d 个条目", len(items))},
		Config:      ScanMonitorConfig{Container: config.ItemsPath, Item: "*", Fields: fieldConfigs, Fetch: &config},
	}, nil
}

func buildScanConfig(parent *goquery.Selection, itemCSS string, samples []ExtractResult) ScanMonitorConfig {
	container := buildElementSelector(parent)
	if container == "" {
		container = buildDeepSelector(parent)
	}
	item := itemCSS
	if item == "" {
		item = "a"
	}
	fields := inferScanFields(parent, item, samples)
	return ScanMonitorConfig{Container: container, Item: item, Fields: fields}
}

func inferScanFields(parent *goquery.Selection, itemCSS string, samples []ExtractResult) []ScanFieldConfig {
	titleSelector := inferTitleSelector(parent, itemCSS, samples)
	fields := []ScanFieldConfig{{Name: "title", Selector: titleSelector, Type: "text"}}
	item := parent.Find(itemCSS).First()
	if item.Is("a") && item.AttrOr("href", "") != "" {
		fields = append(fields, ScanFieldConfig{Name: "url", Selector: "", Type: "attr", Attr: "href"})
	} else if selector := inferURLSelector(parent, itemCSS, titleSelector); selector != "" {
		fields = append(fields, ScanFieldConfig{Name: "url", Selector: selector, Type: "attr", Attr: "href"})
	}
	if selector := inferDateSelector(parent, itemCSS, samples); selector != "" {
		fields = append(fields, ScanFieldConfig{Name: "date", Selector: selector, Type: "text"})
	}
	if selector := inferSummarySelector(parent, itemCSS, titleSelector); selector != "" {
		fields = append(fields, ScanFieldConfig{Name: "summary", Selector: selector, Type: "text", Transform: `regexp("\\s+", " ")`})
	}
	return fields
}

func inferTitleSelector(parent *goquery.Selection, itemCSS string, samples []ExtractResult) string {
	itemLower := strings.ToLower(itemCSS)
	if strings.HasPrefix(itemLower, "li") || strings.HasPrefix(itemLower, "tr") || strings.HasPrefix(itemLower, "dd") || strings.HasPrefix(itemLower, "dt") {
		if parent.Find(itemCSS).First().Find("a").Length() == 0 {
			return ""
		}
	}
	item := parent.Find(itemCSS).First()
	if item.Length() == 0 {
		return "a"
	}
	selectors := []string{
		".article__card__title a, .article-card__title a, .post-card__title a, .title a",
		"h1 a, h2 a, h3 a, h4 a",
		".article__card__title, .article-card__title, .post-card__title, .title",
		"h1, h2, h3, h4",
		"a",
	}
	for _, selector := range selectors {
		if hasNonEmptyText(item.Find(selector)) {
			return selector
		}
	}
	if len(samples) > 0 && getTitle(samples[0]) != "" {
		return ""
	}
	return "a"
}

func inferURLSelector(parent *goquery.Selection, itemCSS, titleSelector string) string {
	item := parent.Find(itemCSS).First()
	if item.Length() == 0 {
		return "a"
	}
	// item 自身就是链接项
	if item.Is("a") && item.AttrOr("href", "") != "" {
		return ""
	}
	if titleSelector != "" && item.Find(titleSelector).Filter("a").Length() > 0 {
		return titleSelector
	}
	if titleSelector != "" && item.Find(titleSelector).Find("a").Length() > 0 {
		return titleSelector + " a"
	}
	if item.Find("a[href]").Length() > 0 {
		return "a[href]"
	}
	return ""
}

func inferDateSelector(parent *goquery.Selection, itemCSS string, samples []ExtractResult) string {
	item := parent.Find(itemCSS).First()
	if item.Length() == 0 {
		return ""
	}
	selectors := []string{"time", ".date", ".time", ".meta", ".publish", "span", "small"}
	for _, selector := range selectors {
		found := false
		item.Find(selector).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if isDateLike(strings.TrimSpace(s.Text())) {
				found = true
				return false
			}
			return true
		})
		if found {
			return selector
		}
	}
	return ""
}

func inferSummarySelector(parent *goquery.Selection, itemCSS, titleSelector string) string {
	item := parent.Find(itemCSS).First()
	if item.Length() == 0 {
		return ""
	}
	selectors := []string{"p", ".summary", ".desc", ".content", ".excerpt"}
	for _, selector := range selectors {
		sel := item.Find(selector).First()
		if sel.Length() == 0 {
			continue
		}
		text := strings.TrimSpace(sel.Text())
		if text == "" {
			continue
		}
		if titleSelector != "" {
			titleText := strings.TrimSpace(item.Find(titleSelector).First().Text())
			if titleText != "" && text == titleText {
				continue
			}
		}
		if len([]rune(text)) >= 12 {
			return selector
		}
	}
	return ""
}

func hasNonEmptyText(sel *goquery.Selection) bool {
	ok := false
	sel.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if strings.TrimSpace(s.Text()) != "" {
			ok = true
			return false
		}
		return true
	})
	return ok
}

func ScanConfigToSelectors(config ScanMonitorConfig) SiteSelectors {
	fields := make([]FieldConfig, 0, len(config.Fields))
	for _, f := range config.Fields {
		fields = append(fields, FieldConfig{Name: f.Name, Selector: f.Selector, Type: f.Type, Attr: f.Attr, Transform: f.Transform})
	}
	return SiteSelectors{Container: config.Container, Item: config.Item, Fields: fields}
}

func scanConfigToSelectors(config ScanMonitorConfig) SiteSelectors {
	return ScanConfigToSelectors(config)
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

func getTitle(item ExtractResult) string {
	if v, ok := item["title"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func setTitle(item ExtractResult, title string) {
	item["title"] = title
}

func findBestContainer(sel *goquery.Selection, doc *goquery.Document) *goquery.Selection {
	type scoredContainer struct {
		sel   *goquery.Selection
		score int
	}
	var best scoredContainer

	// Parents() is used instead of repeatedly calling Parent() on the current
	// selection.  The latter can retain a selection with an unexpected node
	// set when the match was produced by a goquery traversal, which caused the
	// real list element to be skipped in favour of <main>/<section>.
	candidates := sel.AddSelection(sel.Parents())
	candidates.Each(func(index int, current *goquery.Selection) {
		tag := goquery.NodeName(current)
		if tag == "html" || tag == "body" {
			return
		}
		class, _ := current.Attr("class")

		if isContainerTag(tag) || hasArticleClass(class) {
			children := current.Children()
			childCount := 0
			linkCount := 0
			articleCardCount := 0
			textLen := 0

			children.Each(func(_ int, c *goquery.Selection) {
				ctag := goquery.NodeName(c)
				cclass, _ := c.Attr("class")

				if isListItemTag(ctag) || hasArticleItemClass(cclass) {
					childCount++
				}
				if c.Find("a").Length() > 0 {
					linkCount++
				}
				if hasArticleItemClass(cclass) {
					articleCardCount++
				}
				firstA := c.Find("a").First()
				if firstA.Length() > 0 {
					textLen += len(strings.TrimSpace(firstA.Text()))
				}
			})

			score := 0
			score += articleCardCount * 100
			score += childCount * 10
			score += linkCount * 5
			if childCount > 0 && textLen/childCount > 5 {
				score += 15
			}
			if childCount >= 2 && linkCount >= childCount/2 {
				score += 50
			}
			score += index * 2

			lowerClass := strings.ToLower(class)
			if strings.Contains(lowerClass, "footer") || strings.Contains(lowerClass, "header") ||
				strings.Contains(lowerClass, "nav") || strings.Contains(lowerClass, "sidebar") ||
				strings.Contains(lowerClass, "menu") || strings.Contains(lowerClass, "banner") ||
				strings.Contains(lowerClass, "breadcrumb") || strings.Contains(lowerClass, "crumb") {
				score = 0
			}

			if score > best.score {
				best = scoredContainer{sel: current, score: score}
			}
		}
	})

	if best.score >= 20 {
		return best.sel
	}

	nearestList := sel.Closest("ul, ol")
	if nearestList.Length() > 0 {
		liCount := nearestList.Find("li").Length()
		if liCount >= 2 {
			return nearestList
		}
	}

	return nil
}

func isContainerTag(tag string) bool {
	containers := map[string]bool{
		"div": true, "section": true, "ul": true, "ol": true,
		"table": true, "tbody": true, "dl": true,
		"article": true, "main": true,
	}
	return containers[tag]
}

func hasArticleClass(class string) bool {
	if class == "" {
		return false
	}
	lower := strings.ToLower(class)
	markers := []string{
		"article-list", "articlelist", "post-list", "postlist",
		"news-list", "newslist", "content-list", "contentlist",
		"feed", "feed__main", "feed_main",
		"list-view", "card-list", "cardlist",
		"section__wrapper", "section_wrapper",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func hasArticleItemClass(class string) bool {
	if class == "" {
		return false
	}
	lower := strings.ToLower(class)
	markers := []string{
		"article-card", "articlecard", "article__card", "post-card", "postcard",
		"news-item", "newsitem", "content-item", "contentitem",
		"feed-item", "feeditem", "card-item", "carditem",
		"list-item", "listitem",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func isListItemTag(tag string) bool {
	items := map[string]bool{
		"li": true, "tr": true, "dd": true, "dt": true,
		"div": true, "p": true, "article": true,
	}
	return items[tag]
}

func buildElementSelector(sel *goquery.Selection) string {
	if sel.Length() == 0 {
		return ""
	}
	if id, exists := sel.Attr("id"); exists && id != "" {
		return "#" + id
	}
	tag := goquery.NodeName(sel)
	if class, exists := sel.Attr("class"); exists && class != "" {
		classes := strings.Fields(class)
		for _, c := range classes {
			if len(c) > 1 && !strings.HasPrefix(c, "ng-") && !strings.HasPrefix(c, "_") {
				return tag + "." + c
			}
		}
		return tag + "." + classes[0]
	}
	return tag
}

// childAnalysis 子元素分析结果
type childAnalysis struct {
	tagStats         map[string]*tagCount
	articleCardCount int
	articleCardTag   string
	articleCardClass string
}

type tagCount struct {
	tag   string
	count int
	class string
}

// analyzeChildren 分析父元素的所有子元素，返回标签统计和文章卡片识别结果
func analyzeChildren(sel *goquery.Selection) *childAnalysis {
	result := &childAnalysis{
		tagStats: make(map[string]*tagCount),
	}

	sel.Children().Each(func(_ int, c *goquery.Selection) {
		ctag := goquery.NodeName(c)
		cclass, _ := c.Attr("class")

		if _, exists := result.tagStats[ctag]; !exists {
			result.tagStats[ctag] = &tagCount{tag: ctag, count: 0}
		}
		result.tagStats[ctag].count++
		if result.tagStats[ctag].class == "" {
			classes := strings.Fields(cclass)
			if len(classes) > 0 {
				result.tagStats[ctag].class = classes[0]
			}
		}

		if hasArticleItemClass(cclass) {
			result.articleCardCount++
			if result.articleCardTag == "" {
				result.articleCardTag = ctag
				classes := strings.Fields(cclass)
				for _, cls := range classes {
					if hasArticleItemClass(cls) {
						result.articleCardClass = cls
						break
					}
				}
			}
		}
	})

	return result
}

func buildShortSelector(sel *goquery.Selection, tag string) string {
	base := buildElementSelector(sel)
	if base == "" {
		base = tag
	}
	// 裸标签或重复 class 往往会同时命中导航、页脚和正文列表。
	// 当短选择器不唯一时，保留父级路径来限定到实际扫描到的容器。
	root := sel.ParentsFiltered("html")
	if root.Length() > 0 && root.Find(base).Length() > 1 {
		if deep := buildDeepSelector(sel); deep != "" {
			return deep
		}
	}
	return base
}

func detectItemPattern(sel *goquery.Selection) (tag, css string) {
	if tag, css, ok := detectDirectArticleAnchors(sel); ok {
		return tag, css
	}

	analysis := analyzeChildren(sel)

	if analysis.articleCardCount >= 2 && analysis.articleCardTag != "" && analysis.articleCardClass != "" {
		return analysis.articleCardTag, analysis.articleCardTag + "." + analysis.articleCardClass
	}
	if analysis.articleCardCount >= 2 && analysis.articleCardTag != "" {
		return analysis.articleCardTag, analysis.articleCardTag
	}

	var bestTag string
	var bestClass string
	bestCount := 0
	for t, info := range analysis.tagStats {
		if info.count > bestCount {
			bestCount = info.count
			bestTag = t
			bestClass = info.class
		}
	}
	if bestCount >= 2 && bestTag != "" {
		if bestClass != "" {
			return bestTag, bestTag + "." + bestClass
		}
		return bestTag, bestTag
	}
	return "", ""
}

func detectDirectArticleAnchors(sel *goquery.Selection) (tag, css string, ok bool) {
	anchors := sel.ChildrenFiltered("a[href]")
	if anchors.Length() < 2 {
		return "", "", false
	}

	meaningful := 0
	detailLinks := 0
	classCounts := make(map[string]int)
	anchors.Each(func(_ int, anchor *goquery.Selection) {
		if strings.TrimSpace(anchor.Text()) == "" {
			return
		}
		meaningful++
		if href, exists := anchor.Attr("href"); exists && isLikelyDetailHref(href) {
			detailLinks++
		}
		if class, exists := anchor.Attr("class"); exists {
			for _, name := range strings.Fields(class) {
				if len(name) > 1 && !strings.HasPrefix(name, "ng-") && !strings.HasPrefix(name, "_") {
					classCounts[name]++
				}
			}
		}
	})
	if meaningful < 2 || detailLinks*2 < meaningful {
		return "", "", false
	}

	bestClass := ""
	bestCount := 0
	for name, count := range classCounts {
		if count > bestCount {
			bestClass = name
			bestCount = count
		}
	}
	if bestClass != "" && bestCount*2 >= meaningful {
		return "a", "a." + bestClass, true
	}
	return "a", "a", true
}

func isLikelyDetailHref(href string) bool {
	path := strings.ToLower(strings.SplitN(strings.TrimSpace(href), "?", 2)[0])
	return strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".htm") || strings.HasSuffix(path, ".jhtml")
}

func extractContainerItems(entry *scanContainerEntry) {
	if entry.parent.ChildrenFiltered("a").Length() >= 2 {
		entry.parent.ChildrenFiltered("a").Each(func(_ int, a *goquery.Selection) {
			text := strings.TrimSpace(a.Text())
			if text == "" {
				return
			}
			item := ExtractResult{"title": text}
			if href, exists := a.Attr("href"); exists {
				item["url"] = href
			}
			entry.items = append(entry.items, item)
			entry.itemsSel = append(entry.itemsSel, a)
		})
		if len(entry.items) > 0 {
			return
		}
	}

	entry.parent.Children().Each(func(_ int, c *goquery.Selection) {
		ctag := goquery.NodeName(c)
		cclass, _ := c.Attr("class")

		if !isListItemTag(ctag) && !hasArticleItemClass(cclass) {
			return
		}

		item := make(ExtractResult)

		// 策略1: 标题类名元素
		titleClasses := []string{
			"article__card__title", "article-card__title", "post-card__title",
			"card__title", "item__title", "news__title", "title",
			"article-title", "post-title", "entry-title",
		}
		for _, cls := range titleClasses {
			titleEl := c.Find("." + strings.ReplaceAll(cls, " ", "."))
			if titleEl.Length() > 0 {
				text := strings.TrimSpace(titleEl.Text())
				if text != "" {
					setTitle(item, text)
					link := titleEl.Find("a").First()
					if link.Length() > 0 {
						if href, exists := link.Attr("href"); exists {
							item["url"] = href
						}
					}
					break
				}
			}
		}

		// 策略2: 从链接中取标题（需用辅助函数安全判断 nil）
		if getTitle(item) == "" {
			var bestTitle, bestURL string
			c.Find("a").Each(func(_ int, a *goquery.Selection) {
				href, _ := a.Attr("href")
				text := strings.TrimSpace(a.Text())
				if text == "" {
					return
				}
				isBetter := false
				if strings.Contains(href, "/post/") || strings.Contains(href, "/article/") || strings.Contains(href, ".html") || strings.Contains(href, ".htm") || strings.Contains(href, "/item/") {
					isBetter = true
				} else if bestTitle == "" || len(text) > len(bestTitle) {
					isBetter = true
				}
				if isBetter {
					bestTitle, bestURL = text, href
				}
			})
			if bestTitle != "" {
				setTitle(item, bestTitle)
				if bestURL != "" {
					item["url"] = bestURL
				}
			}
		}

		// 策略3: 回退到子项文本
		if getTitle(item) == "" {
			text := strings.TrimSpace(c.Text())
			runes := []rune(text)
			if len(runes) > 3 {
				s := text
				if len([]byte(s)) > 80 {
					s = string(runes[:min(80, len(runes))]) + "..."
				}
				setTitle(item, s)
			}
		}

		// 提取日期
		c.Find("span, time, small, .date, .time, .meta, .publish").Each(func(_ int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if isDateLike(text) && item["date"] == "" {
				item["date"] = text
			}
		})

		if getTitle(item) != "" {
			entry.items = append(entry.items, item)
			entry.itemsSel = append(entry.itemsSel, c)
		}
	})
}

func isDateLike(text string) bool {
	return dateLikePattern.MatchString(strings.TrimSpace(text))
}

var dateLikePattern = regexp.MustCompile(`20\d{2}(?:[-/.]\d{1,2}[-/.]\d{1,2}|年\d{1,2}月\d{1,2}日?|\.\d{4})`)

func candidateScore(entry *scanContainerEntry, strategy string) int {
	validTitles := 0
	urlCount := 0
	dateCount := 0
	for _, item := range entry.items {
		if getTitle(item) != "" {
			validTitles++
		}
		if url, ok := item["url"].(string); ok && url != "" {
			urlCount++
		}
		if date, ok := item["date"].(string); ok && date != "" {
			dateCount++
		}
	}
	score := min(entry.hits, 10)*12 + validTitles*20 + urlCount*15 + dateCount*8 + len(entry.items)*6
	switch strategy {
	case "keyword_ancestor":
		score += 18
	case "table_rows":
		score += 15
	case "repeated_list":
		score += 12
	case "link_cluster":
		score += 8
	}
	if strings.HasPrefix(strategy, "template_") || strings.HasPrefix(strategy, "rule_") {
		score += 1000
	}
	if countArticleCards(entry.parent) >= 2 {
		score += 18
	}
	if len(entry.items) > 0 && validTitles*2 < len(entry.items) {
		score -= 10
	}
	if len(entry.items) > 0 && urlCount == 0 {
		// 公告/文章列表通常应带详情链接。元数据块即使命中关键词，
		// 也不能压过同时包含标题和详情链接的真实内容列表。
		score -= 80
	} else if validTitles > 0 && urlCount*2 < validTitles {
		score -= 25
	}
	if isLikelyNoiseContainer(entry.parent) {
		score -= 80
	}
	return score
}

func sortScanContainers(keys []scanContainerKey, m map[scanContainerKey]*scanContainerEntry, strategies map[scanContainerKey]string) {
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			ei := m[keys[i]]
			ej := m[keys[j]]
			scoreI := candidateScore(ei, strategies[keys[i]])
			scoreJ := candidateScore(ej, strategies[keys[j]])
			if scoreJ > scoreI {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
}

func countArticleCards(sel *goquery.Selection) int {
	return analyzeChildren(sel).articleCardCount
}

func candidateConfidence(entry *scanContainerEntry, strategy string) int {
	base := 35 + entry.hits*2 + len(entry.items)*3
	if strategy == "keyword_ancestor" {
		base += 10
	}
	if strategy == "repeated_list" {
		base += 8
	}
	if strategy == "table_rows" {
		base += 6
	}
	if strategy == "link_cluster" {
		base += 4
	}
	if countArticleCards(entry.parent) >= 2 {
		base += 10
	}
	if len(entry.items) == 0 {
		base -= 20
	}
	if isLikelyNoiseContainer(entry.parent) {
		base -= 30
	}
	return min(100, max(1, base))
}

func MonitorFromScan(name, url, containerCSS string) (*Monitor, error) {
	containerSel := containerCSS
	itemSel := ""

	if idx := strings.LastIndex(containerCSS, " > "); idx > 0 {
		containerSel = containerCSS[:idx]
		itemSel = containerCSS[idx+3:]
	}

	config := buildLegacyScanConfig(containerSel, itemSel)
	return MonitorFromScanConfig(name, url, config)
}

func buildLegacyScanConfig(containerSel, itemSel string) ScanMonitorConfig {
	if itemSel == "" {
		itemSel = "a"
	}
	titleSelector := "a"
	urlSelector := "a"
	itemLower := strings.ToLower(itemSel)

	if strings.Contains(itemLower, "article__card") || strings.Contains(itemLower, "article-card") {
		titleSelector = ".article__card__title, .article-card__title, h3"
		urlSelector = ".article__card__link, .article-card__link"
	} else if strings.Contains(itemLower, "article") || itemLower == "article" {
		titleSelector = "h3, h2, .title, a"
	} else if strings.HasPrefix(itemLower, "div.") || strings.HasPrefix(itemLower, "li.") {
		titleSelector = "a"
	}
	if strings.HasPrefix(itemLower, "li.") || itemLower == "li" {
		titleSelector = ""
	}

	fields := []ScanFieldConfig{{Name: "title", Selector: titleSelector, Type: "text"}}
	if urlSelector != "" {
		fields = append(fields, ScanFieldConfig{Name: "url", Selector: urlSelector, Type: "attr", Attr: "href"})
	}
	return ScanMonitorConfig{Container: containerSel, Item: itemSel, Fields: fields}
}

func MonitorFromScanConfig(name, url string, config ScanMonitorConfig) (*Monitor, error) {
	site := &database.Site{
		Name:          name,
		URL:           url,
		Container:     config.Container,
		Item:          config.Item,
		GroupName:     "默认",
		CheckInterval: 3600,
		IsActive:      true,
		Fields:        ScanFieldsToSiteFields(config.Fields),
	}

	if err := database.CreateSiteWithFields(site); err != nil {
		return nil, fmt.Errorf("保存站点失败: %w", err)
	}

	var savedSite database.Site
	if err := database.GetDB().Preload("Fields").First(&savedSite, site.ID).Error; err != nil {
		return nil, fmt.Errorf("加载已保存站点失败: %w", err)
	}

	if err := StartSite(savedSite.Name); err != nil {
		log.Printf("[智能创建] 启动监控器「%s」失败: %v", name, err)
		// 回滚数据库状态
		if dbErr := database.GetDB().Model(&savedSite).Update("is_active", false).Error; dbErr != nil {
			log.Printf("[智能创建] 回滚监控器「%s」活跃状态失败: %v", name, dbErr)
		}
		return nil, fmt.Errorf("启动监控器失败: %w", err)
	}

	log.Printf("[智能创建] 监控器「%s」已创建并启动", name)
	return GetMonitor(savedSite.Name), nil
}
