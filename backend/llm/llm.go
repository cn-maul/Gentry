// Package llm 提供 OpenAI 兼容 Chat Completions 客户端与接入配置存取。
// 通过 Base URL 可对接 OpenAI、DeepSeek、Moonshot、通义千问、Ollama 等服务。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cn-maul/Gentry/database"
)

// SystemSetting 中保存接入配置的键。
const (
	SettingBaseURL = "llm_base_url"
	SettingAPIKey  = "llm_api_key"
	SettingModel   = "llm_model"
)

// Config 是一次 LLM 调用所需的全部接入信息。
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

// ErrNotConfigured 表示尚未完成 AI 模型接入配置。
var ErrNotConfigured = fmt.Errorf("尚未配置 AI 模型，请先在「设置」页填写接入地址和模型名称")

// LoadConfig 从系统设置读取接入配置。
func LoadConfig() (Config, error) {
	cfg := Config{
		BaseURL: getSetting(SettingBaseURL),
		APIKey:  getSetting(SettingAPIKey),
		Model:   getSetting(SettingModel),
	}
	if !cfg.Configured() {
		return Config{}, ErrNotConfigured
	}
	return cfg, nil
}

func getSetting(key string) string {
	value, _ := database.GetSetting(key)
	return value
}

// SaveConfig 保存接入配置。空 API Key 允许保存（本地 Ollama 等无需鉴权）。
func SaveConfig(cfg Config) error {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("接入地址必须是有效的 http(s) URL")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("模型名称不能为空")
	}
	if err := database.SetSetting(SettingBaseURL, baseURL); err != nil {
		return err
	}
	if err := database.SetSetting(SettingModel, strings.TrimSpace(cfg.Model)); err != nil {
		return err
	}
	return database.SetSetting(SettingAPIKey, strings.TrimSpace(cfg.APIKey))
}

func (c Config) Configured() bool {
	return strings.TrimSpace(c.BaseURL) != "" && strings.TrimSpace(c.Model) != ""
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 以 system + user 两条消息调用一次补全，返回模型输出的文本。
func Chat(ctx context.Context, cfg Config, systemPrompt, userPrompt string) (string, error) {
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	payload, err := json.Marshal(chatRequest{
		Model: cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0,
		Stream:      false,
	})
	if err != nil {
		return "", err
	}

	requestCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 AI 服务失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("读取 AI 响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI 服务返回 %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("解析 AI 响应失败: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("AI 服务错误: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("AI 未返回任何内容")
	}
	return parsed.Choices[0].Message.Content, nil
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}
