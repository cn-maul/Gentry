package monitor

import (
	"context"
	"fmt"

	"github.com/cn-maul/Gentry/database"
)

// ValidateExtraction 只读验证抓取与选择器配置：执行一次完整提取并返回样本，
// 不写入任何基线或历史状态。
func ValidateExtraction(ctx context.Context, site *database.Site) (*ExtractionValidationResult, error) {
	items, err := ExtractConfiguredSource(ctx, site)
	if err != nil {
		return nil, err
	}
	if err := ResolveExtractedURLs(site.URL, items); err != nil {
		return nil, fmt.Errorf("resolve URLs failed: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("提取结果为空，请检查选择器")
	}

	report := &ExtractionValidationResult{ExtractedItems: len(items)}
	limit := len(items)
	if limit > 5 {
		limit = 5
	}
	for _, item := range items[:limit] {
		key := extractKey(item)
		raw := toString(item["title"])
		if raw == "" {
			raw = key
		}
		report.Samples = append(report.Samples, ExtractionValidationSample{ItemKey: key, Raw: raw})
	}
	return report, nil
}
