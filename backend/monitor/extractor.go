package monitor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// isLikelyDetailHref 判断链接是否像详情页（用于旧配置修复的启发式打分）。
func isLikelyDetailHref(href string) bool {
	path := strings.ToLower(strings.SplitN(strings.TrimSpace(href), "?", 2)[0])
	return strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".htm") || strings.HasSuffix(path, ".jhtml")
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

func isDateLike(text string) bool {
	return dateLikePattern.MatchString(strings.TrimSpace(text))
}

var dateLikePattern = regexp.MustCompile(`20\d{2}(?:[-/.]\d{1,2}[-/.]\d{1,2}|年\d{1,2}月\d{1,2}日?|\.\d{4})`)

// ExtractResult 表示从网页中提取的单个结果项
type ExtractResult map[string]interface{}

// SiteSelectors 提取器选择器配置
type SiteSelectors struct {
	Container string
	Item      string
	Fields    []FieldConfig
}

// FieldConfig 提取字段配置
type FieldConfig struct {
	Name      string
	Selector  string
	Type      string
	Attr      string
	Transform string
}

type Extractor struct {
	containerSelector string
	itemSelector      string
	fields            []FieldConfig
}

func NewExtractor(selectors SiteSelectors) *Extractor {
	return &Extractor{
		containerSelector: selectors.Container,
		itemSelector:      selectors.Item,
		fields:            selectors.Fields,
	}
}

func (e *Extractor) Extract(html string) ([]ExtractResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var results []ExtractResult

	containers := doc.Find(e.containerSelector)
	containers = narrowLegacyBroadContainers(containers, e.containerSelector, e.itemSelector)
	items := containers
	recoveredLegacyAnchors := false
	if strings.TrimSpace(e.itemSelector) != "" {
		if recovered := recoverLegacyMetadataItemSelector(containers, e.itemSelector); recovered != nil {
			items = recovered
			recoveredLegacyAnchors = true
		} else {
			items = items.Find(e.itemSelector)
		}
	}
	items.Each(func(_ int, s *goquery.Selection) {
		result := make(ExtractResult)
		for _, field := range e.fields {
			if value := e.extractField(s, field); value != nil {
				result[field.Name] = value
			}
		}
		if recoveredLegacyAnchors {
			if _, exists := result["url"]; !exists {
				if href, exists := s.Attr("href"); exists && strings.TrimSpace(href) != "" {
					result["url"] = href
				}
			}
		}
		if len(result) > 0 {
			results = append(results, result)
		}
	})

	return results, nil
}

// recoverLegacyMetadataItemSelector 兼容旧扫描器把公告旁边的元数据 ul
// 误识别成条目，而真正公告是容器直接子级链接的配置。
func recoverLegacyMetadataItemSelector(containers *goquery.Selection, itemSelector string) *goquery.Selection {
	selector := strings.ToLower(strings.TrimSpace(itemSelector))
	if !strings.HasPrefix(selector, "ul") && !strings.HasPrefix(selector, "ol") {
		return nil
	}
	metadataItems := containers.Find(itemSelector)
	if metadataItems.Length() < 2 || metadataItems.Find("a[href]").Length() > 0 {
		return nil
	}
	directAnchors := containers.ChildrenFiltered("a[href]")
	if directAnchors.Length() < 2 {
		return nil
	}
	meaningful := 0
	detailLinks := 0
	directAnchors.Each(func(_ int, anchor *goquery.Selection) {
		if strings.TrimSpace(anchor.Text()) == "" {
			return
		}
		meaningful++
		if href, exists := anchor.Attr("href"); exists && isLikelyDetailHref(href) {
			detailLinks++
		}
	})
	if meaningful < 2 || detailLinks*2 < meaningful {
		return nil
	}
	return directAnchors
}

// narrowLegacyBroadContainers 修复旧版扫描器保存的 ul/ol/table 等过宽配置。
// 只有最佳候选明显优于其他候选时才收窄，避免影响确实需要合并多个列表的配置。
func narrowLegacyBroadContainers(containers *goquery.Selection, containerSelector, itemSelector string) *goquery.Selection {
	selector := strings.ToLower(strings.TrimSpace(containerSelector))
	if containers.Length() <= 1 || strings.TrimSpace(itemSelector) == "" {
		return containers
	}
	switch selector {
	case "ul", "ol", "table", "tbody", "dl":
	default:
		return containers
	}

	var best *goquery.Selection
	bestScore := -1 << 30
	secondScore := -1 << 30
	containers.Each(func(_ int, container *goquery.Selection) {
		score := contentContainerScore(container, itemSelector)
		if score > bestScore {
			secondScore = bestScore
			bestScore = score
			best = container
		} else if score > secondScore {
			secondScore = score
		}
	})

	if best == nil || bestScore-secondScore < 25 {
		return containers
	}
	return best
}

func contentContainerScore(container *goquery.Selection, itemSelector string) int {
	items := container.Find(itemSelector)
	if items.Length() == 0 {
		return -1000
	}

	score := items.Length()
	if isLikelyNoiseRegion(container) {
		score -= 200
	}
	items.Each(func(_ int, item *goquery.Selection) {
		text := strings.TrimSpace(item.Text())
		if len([]rune(text)) >= 8 {
			score += 3
		}
		if isDateLike(text) {
			score += 8
		}
		item.Find("a[href]").EachWithBreak(func(_ int, link *goquery.Selection) bool {
			href, _ := link.Attr("href")
			if isLikelyDetailHref(href) {
				score += 10
				return false
			}
			return true
		})
	})
	return score
}

func isLikelyNoiseRegion(container *goquery.Selection) bool {
	if isLikelyNoiseContainer(container) {
		return true
	}
	noise := false
	container.Parents().EachWithBreak(func(index int, parent *goquery.Selection) bool {
		if index >= 4 {
			return false
		}
		if isLikelyNoiseContainer(parent) {
			noise = true
			return false
		}
		return true
	})
	return noise
}

func (e *Extractor) extractField(s *goquery.Selection, field FieldConfig) interface{} {
	sel := s
	if field.Selector != "" {
		sel = s.Find(field.Selector)
	}
	if sel.Length() == 0 {
		if field.Type == "text" && field.Name == "title" {
			sel = s
		} else if field.Type == "attr" {
			// attr 类型的字段（如 URL），selector 为空或查不到时尝试从当前项自身提取属性
			// 这是处理 item 本身就是 <a> 标签时提取自身 href 的关键路径
			sel = s
		} else {
			return nil
		}
	}

	var value string
	switch field.Type {
	case "attr":
		attr := field.Attr
		if attr == "" {
			attr = "href"
		}
		value, _ = sel.Attr(attr)
	case "text":
		value = strings.TrimSpace(sel.Text())
	default:
		return nil
	}

	if field.Transform != "" {
		value = applyTransform(value, field.Transform)
	}

	return value
}

// applyTransform 应用转换规则
// 支持格式:
//
//	trim(chars)    — 去除两端指定字符
//	prefix(text)   — 添加前缀
//	suffix(text)   — 添加后缀
//	regexp(pat,repl) — 正则替换
func applyTransform(value, transform string) string {
	if value == "" || transform == "" {
		return value
	}

	// 解析 transform: funcName(args)
	idx := strings.Index(transform, "(")
	if idx < 0 || !strings.HasSuffix(transform, ")") {
		return value
	}

	name := transform[:idx]
	args := transform[idx+1 : len(transform)-1]

	switch name {
	case "trim":
		return strings.Trim(value, args)
	case "prefix":
		return args + value
	case "suffix":
		return value + args
	case "regexp":
		parts := strings.SplitN(args, ",", 2)
		if len(parts) == 2 {
			pattern := strings.TrimSpace(parts[0])
			replacement := strings.TrimSpace(parts[1])
			// 去掉可能的引号
			pattern = strings.Trim(pattern, `"'`)
			replacement = strings.Trim(replacement, `"'`)
			re, err := regexp.Compile(pattern)
			if err != nil {
				// 编译失败则返回原值
				return value
			}
			return re.ReplaceAllString(value, replacement)
		}
		return value
	default:
		return value
	}
}
