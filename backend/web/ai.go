package web

import (
	"net/http"
	"strings"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/llm"
	"github.com/cn-maul/Gentry/monitor"
	"github.com/gin-gonic/gin"
)

func getLLMSettingValue(key string) string {
	value, _ := database.GetSetting(key)
	return value
}

// ===== AI 模型接入设置 =====

func (s *WebServer) getLLMSettings(c *gin.Context) {
	cfg := llm.Config{
		BaseURL: getLLMSettingValue(llm.SettingBaseURL),
		APIKey:  getLLMSettingValue(llm.SettingAPIKey),
		Model:   getLLMSettingValue(llm.SettingModel),
	}
	ok(c, map[string]interface{}{
		"base_url":   cfg.BaseURL,
		"api_key":    maskSecret(cfg.APIKey),
		"model":      cfg.Model,
		"configured": cfg.Configured(),
	})
}

func (s *WebServer) updateLLMSettings(c *gin.Context) {
	var req struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	}
	if !bindJSON(c, &req) {
		return
	}
	cfg := llm.Config{
		BaseURL: strings.TrimSpace(req.BaseURL),
		APIKey:  strings.TrimSpace(req.APIKey),
		Model:   strings.TrimSpace(req.Model),
	}
	// 前端回显的是脱敏值；未修改密钥时保留原值
	if cfg.APIKey != "" {
		if existing := getLLMSettingValue(llm.SettingAPIKey); cfg.APIKey == maskSecret(existing) {
			cfg.APIKey = existing
		}
	}
	if err := llm.SaveConfig(cfg); err != nil {
		fail(c, http.StatusBadRequest, "保存失败: "+err.Error())
		return
	}
	ok(c, nil)
}

func (s *WebServer) testLLMSettings(c *gin.Context) {
	cfg, err := llm.LoadConfig()
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	answer, err := llm.Chat(c.Request.Context(), cfg, "你是连通性测试助手。", "请只回复两个字：正常")
	if err != nil {
		fail(c, http.StatusBadRequest, "连接失败: "+err.Error())
		return
	}
	ok(c, map[string]interface{}{
		"ok":      true,
		"model":   cfg.Model,
		"message": "连接成功，模型已回复",
		"answer":  truncateForDisplay(answer, 50),
	})
}

func truncateForDisplay(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}

// ===== AI 规则提取 =====

func (s *WebServer) aiExtractScanRule(c *gin.Context) {
	var req struct {
		URL      string `json:"url" binding:"required"`
		Keywords string `json:"keywords"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		fail(c, http.StatusBadRequest, "URL 无效: "+err.Error())
		return
	}

	result, err := monitor.AIExtractRule(c.Request.Context(), req.URL, splitKeywords(req.Keywords))
	if err != nil {
		fail(c, http.StatusBadRequest, "AI 提取失败: "+err.Error())
		return
	}
	ok(c, result)
}

// splitKeywords 分割关键词（支持中英文逗号与空白）。
func splitKeywords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == '　' || r == ' '
	})
}
