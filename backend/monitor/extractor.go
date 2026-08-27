package monitor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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

// Extract 按选择器配置从 HTML 提取条目。
//
// 注意：当前实现仍会调用 extractor_compat.go 中的「旧配置修复启发式」
// （narrowLegacyBroadContainers / recoverLegacyMetadataItemSelector），
// 用于兼容历史版本保存的过宽容器或误选元数据列表的配置。
// 这属于存量兼容层，不属于规则识别语义；行为由 extractor_compat_test.go
// 的特征化测试锁定，未来移除前必须先迁移受影响的存量配置。
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
