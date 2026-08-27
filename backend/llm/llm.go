// Package llm 提供 OpenAI 兼容 Chat Completions 客户端与接入配置存取。
// 通过 Base URL 可对接 OpenAI、DeepSeek、Moonshot、通义千问、Ollama 等服务。
//
// 调用方应依赖 Chat（便捷封装）或 Provider 接口（需要 usage/JSON 模式时），
// 不要直接构造 HTTP 请求。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
		return fmt.Errorf("模型名称不能是空")
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

// ===== Provider 抽象 =====

// CompleteRequest 是一次补全调用的输入。
type CompleteRequest struct {
	SystemPrompt string
	UserPrompt   string
	Temperature  float64
	// JSONMode 为 true 时向服务端透传 response_format={"type":"json_object"}；
	// 服务端不支持该字段时应忽略而非报错（多数 OpenAI 兼容实现兼容）。
	JSONMode bool
}

// Usage 是服务端返回的 token 用量统计（缺失时为零值）。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Response 是一次补全调用的输出。
type Response struct {
	Content string
	Model   string
	Usage   Usage
}

// ErrorKind 对 Provider 错误做粗粒度分类，供调用方决定是否值得重试。
type ErrorKind string

const (
	KindNetwork  ErrorKind = "network"  // 连接失败、超时等瞬态错误
	KindHTTP     ErrorKind = "http"     // 非 200 响应
	KindAuth     ErrorKind = "auth"     // 401/403 鉴权失败
	KindBusiness ErrorKind = "business" // 服务端业务错误（error.message）
	KindEmpty    ErrorKind = "empty"    // 响应缺少内容
)

// ProviderError 携带错误分类；Error() 文案与历史版本保持一致。
type ProviderError struct {
	Kind    ErrorKind
	Status  int
	Message string
	Wrapped error
}

func (e *ProviderError) Error() string {
	switch e.Kind {
	case KindNetwork:
		return fmt.Sprintf("请求 AI 服务失败: %v", e.Wrapped)
	case KindHTTP, KindAuth:
		return fmt.Sprintf("AI 服务返回 %d: %s", e.Status, e.Message)
	case KindBusiness:
		return fmt.Sprintf("AI 服务错误: %s", e.Message)
	case KindEmpty:
		return "AI 未返回任何内容"
	default:
		return e.Message
	}
}

func (e *ProviderError) Unwrap() error { return e.Wrapped }

// IsFatal 判断该错误是否重试也无意义（鉴权失败等）：
// 管线遇到 fatal 错误应立即中止，不再消耗反馈重试机会。
func IsFatal(err error) bool {
	var perr *ProviderError
	if ok := errorsAs(err, &perr); ok {
		return perr.Kind == KindAuth
	}
	return false
}

// errorsAs 是 errors.As 的间接引用，便于在不引入额外 import 冲突的测试中使用。
func errorsAs(err error, target **ProviderError) bool {
	for err != nil {
		if perr, ok := err.(*ProviderError); ok {
			*target = perr
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

// Provider 是一次补全调用的抽象；业务代码依赖本接口而不是具体 SDK。
type Provider interface {
	Complete(ctx context.Context, cfg Config, req CompleteRequest) (Response, error)
}

// defaultProvider 是当前唯一的 OpenAI 兼容实现，带瞬态错误重试。
var defaultProvider Provider = &openAIProvider{}

// SetProvider 替换默认 Provider（测试注入用）。
func SetProvider(p Provider) {
	if p != nil {
		defaultProvider = p
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	Stream         bool            `json:"stream"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// openAIProvider 实现 OpenAI 兼容 /chat/completions 协议。
// 网络错误与 5xx/429 会以固定间隔重试一次；鉴权等其他错误不重试。
type openAIProvider struct{}

const (
	completeTimeout = 90 * time.Second
	maxResponseBody = 4 << 20
	retryBackoff    = 400 * time.Millisecond
)

// retryableStatuses 是值得原样重试一次的瞬态 HTTP 状态码。
var retryableStatuses = map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true}

func (p *openAIProvider) Complete(ctx context.Context, cfg Config, req CompleteRequest) (Response, error) {
	started := time.Now()
	resp, err := p.completeOnce(ctx, cfg, req)
	if err != nil {
		var perr *ProviderError
		if errorsAs(err, &perr) && (perr.Kind == KindNetwork || (perr.Kind == KindHTTP && retryableStatuses[perr.Status])) {
			time.Sleep(retryBackoff)
			log.Printf("[LLM] 瞬态错误(%s)将重试一次: %v", perr.Kind, perr.Error())
			resp, err = p.completeOnce(ctx, cfg, req)
		}
	}
	if err != nil {
		var perr *ProviderError
		if errorsAs(err, &perr) {
			log.Printf("[LLM] 调用失败 kind=%s 耗时=%s: %s", perr.Kind, time.Since(started).Round(time.Millisecond), perr.Error())
		}
		return Response{}, err
	}
	log.Printf("[LLM] 调用成功 model=%s 耗时=%s tokens=%d/%d/%d",
		resp.Model, time.Since(started).Round(time.Millisecond),
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	return resp, nil
}

func (p *openAIProvider) completeOnce(ctx context.Context, cfg Config, req CompleteRequest) (Response, error) {
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	payload, err := json.Marshal(chatRequest{
		Model: cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserPrompt},
		},
		Temperature:    req.Temperature,
		Stream:         false,
		ResponseFormat: jsonModeFormat(req),
	})
	if err != nil {
		return Response{}, fmt.Errorf("构造请求失败: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, completeTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("构造请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Response{}, &ProviderError{Kind: KindNetwork, Wrapped: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return Response{}, &ProviderError{Kind: KindNetwork, Wrapped: fmt.Errorf("读取 AI 响应失败: %w", err)}
	}
	if resp.StatusCode != http.StatusOK {
		kind := KindHTTP
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			kind = KindAuth
		}
		return Response{}, &ProviderError{Kind: kind, Status: resp.StatusCode, Message: truncate(string(body), 300)}
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Response{}, &ProviderError{Kind: KindHTTP, Status: resp.StatusCode, Message: fmt.Sprintf("解析 AI 响应失败: %v", err)}
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return Response{}, &ProviderError{Kind: KindBusiness, Message: parsed.Error.Message}
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return Response{}, &ProviderError{Kind: KindEmpty}
	}
	out := Response{Content: parsed.Choices[0].Message.Content, Model: cfg.Model}
	if parsed.Usage != nil {
		out.Usage = *parsed.Usage
	}
	return out, nil
}

func jsonModeFormat(req CompleteRequest) *responseFormat {
	if !req.JSONMode {
		return nil
	}
	return &responseFormat{Type: "json_object"}
}

// Chat 以 system + user 两条消息调用一次补全，返回模型输出的文本。
// 内部走默认 Provider（含瞬态重试与调用日志）。
func Chat(ctx context.Context, cfg Config, systemPrompt, userPrompt string) (string, error) {
	resp, err := defaultProvider.Complete(ctx, cfg, CompleteRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ExtractJSONObject 从模型输出中提取第一个完整且合法的 JSON 对象文本。
// 容忍 markdown 代码围栏、前后杂文，以及杂文中出现的孤立花括号：
// 通过平衡括号扫描（正确处理字符串内的花括号与转义）逐个候选验证。
func ExtractJSONObject(answer string) (string, error) {
	content := strings.TrimSpace(answer)
	content = strings.ReplaceAll(content, "```json", "```")
	if idx := strings.Index(content, "```"); idx >= 0 {
		if end := strings.Index(content[idx+3:], "```"); end >= 0 {
			content = content[idx+3 : idx+3+end]
		}
	}
	content = strings.TrimSpace(content)

	for offset := 0; offset < len(content); offset++ {
		start := strings.IndexByte(content[offset:], '{')
		if start < 0 {
			break
		}
		start += offset
		end, ok := balancedObjectEnd(content, start)
		if !ok {
			offset = start
			continue
		}
		candidate := content[start : end+1]
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
		offset = start
	}
	return "", fmt.Errorf("输出中未找到 JSON 对象")
}

// balancedObjectEnd 返回从 start（必须指向 '{'）开始的平衡对象结尾下标。
// 扫描跳过字符串字面量（含转义），保证对象内文本的花括号不干扰计数。
func balancedObjectEnd(s string, start int) (int, bool) {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}
