package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	RegisterWithMetadata("webhook", newWebhookNotifier, &ProviderMetadata{
		Label:          "Webhook",
		RequiredFields: []string{"url"},
		OptionalFields: []string{"method"},
	})
}

type webhookNotifier struct {
	url    string
	method string
}

func newWebhookNotifier(config map[string]interface{}) (Notifier, error) {
	url, _ := config["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("缺少必需的 url 参数")
	}

	method, _ := config["method"].(string)
	if method == "" {
		method = "POST"
	}

	return &webhookNotifier{
		url:    url,
		method: method,
	}, nil
}

func (w *webhookNotifier) Send(title, content string) error {
	payload := map[string]interface{}{
		"title":   title,
		"content": content,
		"time":    time.Now().Format("2006-01-02 15:04:05"),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	req, err := http.NewRequest(w.method, w.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("webhook 返回错误 status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

func (w *webhookNotifier) Name() string {
	return "webhook"
}
