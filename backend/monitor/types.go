package monitor

import (
	"github.com/cn-maul/Gentry/database"
)

// ExtractionValidationSample 是配置验证时返回给前端的只读样本。
type ExtractionValidationSample struct {
	ItemKey string `json:"item_key"`
	Raw     string `json:"raw"`
}

// ExtractionValidationResult 汇总一次只读提取验证的结果。
type ExtractionValidationResult struct {
	ExtractedItems int                          `json:"extracted_items"`
	Samples        []ExtractionValidationSample `json:"samples"`
}

// SiteSelectorsFromSite 将 database.Site 转换为 SiteSelectors
// 用于统一 Site 到选择器配置的转换逻辑
func SiteSelectorsFromSite(site *database.Site) SiteSelectors {
	selectors := SiteSelectors{
		Container: site.Container,
		Item:      site.Item,
		Fields:    make([]FieldConfig, len(site.Fields)),
	}
	for i, f := range site.Fields {
		selectors.Fields[i] = FieldConfig{
			Name:      f.Name,
			Selector:  f.Selector,
			Type:      f.Type,
			Attr:      f.Attr,
			Transform: f.Transform,
		}
	}
	return selectors
}
