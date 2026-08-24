package monitor

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/cn-maul/Gentry/database"
)

// NormalizeAndValidateSiteDefinition 规范化并校验监控定义。
// 创建、更新和检查必须复用此入口，避免前后端校验语义漂移。
func NormalizeAndValidateSiteDefinition(site *database.Site) error {
	if site == nil {
		return fmt.Errorf("site is required")
	}
	parsedURL, err := url.ParseRequestURI(strings.TrimSpace(site.URL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("URL 必须是有效的绝对地址")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL 仅支持 http 或 https")
	}
	canonicalFetchConfig, err := CanonicalFetchConfig(site.FetchConfig, site.URL)
	if err != nil {
		return err
	}
	site.FetchConfig = canonicalFetchConfig
	if strings.TrimSpace(site.Container) == "" {
		return fmt.Errorf("容器选择器不能为空")
	}

	fieldNames := make(map[string]struct{}, len(site.Fields))
	for i := range site.Fields {
		if site.Fields[i].Type == "" {
			site.Fields[i].Type = "text"
		}
		field := site.Fields[i]
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return fmt.Errorf("字段名称不能为空")
		}
		if _, exists := fieldNames[name]; exists {
			return fmt.Errorf("字段名称重复: %s", name)
		}
		if field.Type != "text" && field.Type != "attr" {
			return fmt.Errorf("字段 %s 使用了不支持的提取类型: %s", name, field.Type)
		}
		site.Fields[i].Name = name
		fieldNames[name] = struct{}{}
	}
	if len(fieldNames) == 0 {
		return fmt.Errorf("至少需要配置一个提取字段")
	}
	return nil
}
