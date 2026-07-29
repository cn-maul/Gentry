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
	RegisterWithMetadata("bark", newBarkNotifier, &ProviderMetadata{
		Label:          "Bark",
		RequiredFields: []string{"key"},
		OptionalFields: []string{"server", "group", "sound", "icon"},
	})
}

func newBarkNotifier(config map[string]interface{}) (Notifier, error) {
	key, _ := config["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("缺少必需的 key 参数")
	}

	server, _ := config["server"].(string)
	if server == "" {
		server = "https://api.day.app"
	}
	group, _ := config["group"].(string)
	sound, _ := config["sound"].(string)
	icon, _ := config["icon"].(string)

	return &barkNotifier{
		key:    key,
		server: server,
		group:  group,
		sound:  sound,
		icon:   icon,
	}, nil
}

type barkResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	DeviceKey string `json:"device_key"`
}

type barkNotifier struct {
	key    string
	server string
	group  string
	sound  string
	icon   string
}

func (b *barkNotifier) Send(title, content string) error {
	payload := map[string]interface{}{
		"device_key": b.key,
		"title":      title,
		"body":       content,
	}
	if b.group != "" {
		payload["group"] = b.group
	}
	if b.sound != "" {
		payload["sound"] = b.sound
	}
	if b.icon != "" {
		payload["icon"] = b.icon
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	url := fmt.Sprintf("%s/%s", b.server, b.key)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json; charset=utf-8", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	var result barkResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败 (status=%d, body=%s)", resp.StatusCode, string(body))
	}

	if result.Code != 200 {
		return fmt.Errorf("Bark 返回错误: %s (code=%d)", result.Message, result.Code)
	}

	return nil
}

func (b *barkNotifier) Name() string {
	return "bark"
}
