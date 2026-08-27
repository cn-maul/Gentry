// extractor_compat.go 承载「旧配置修复启发式」：兼容历史版本保存的过宽容器
// （ul/ol/table 等未收窄选择器）和误选元数据列表的条目选择器。
//
// 定位说明（见 docs/design/rule-centric-refactor.md §2.1 S2）：
// 这不是规则识别语义的一部分，而是存量配置的兼容层。它随每次 Extract 执行，
// 行为由 extractor_compat_test.go 的特征化测试锁定；未来若移除，
// 必须先把受影响的存量监控配置迁移为精确选择器。
package monitor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var dateLikePattern = regexp.MustCompile(`20\d{2}(?:[-/.]\d{1,2}[-/.]\d{1,2}|年\d{1,2}月\d{1,2}日?|\.\d{4})`)

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
