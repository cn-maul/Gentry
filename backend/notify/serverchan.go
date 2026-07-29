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
	RegisterWithMetadata("serverchan", newServerChanNotifier, &ProviderMetadata{
		Label:          "Server酱",
		RequiredFields: []string{"sendkey"},
		OptionalFields: []string{"channel"},
	})
}

func newServerChanNotifier(config map[string]interface{}) (Notifier, error) {
	sendkey, _ := config["sendkey"].(string)
	if sendkey == "" {
		return nil, fmt.Errorf("缺少必需的 sendkey 参数")
	}

	channel, _ := config["channel"].(string)
	return &serverChanNotifier{
		sendkey: sendkey,
		channel: channel,
	}, nil
}

type serverChanResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    *struct {
		PushID  string `json:"pushid"`
		ReadKey string `json:"readkey"`
		Error   string `json:"error"`
		ErrNo   int    `json:"errno"`
	} `json:"data"`
}

type serverChanNotifier struct {
	sendkey string
	channel string
}

func (s *serverChanNotifier) Send(title, content string) error {
	payload := map[string]interface{}{
		"title": title,
		"desp":  content,
	}
	if s.channel != "" {
		payload["channel"] = s.channel
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	url := fmt.Sprintf("https://sctapi.ftqq.com/%s.send", s.sendkey)
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

	var result serverChanResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败 (status=%d, body=%s)", resp.StatusCode, string(body))
	}

	if result.Code != 0 {
		return fmt.Errorf("Server酱返回错误: %s (code=%d)", result.Message, result.Code)
	}

	return nil
}

func (s *serverChanNotifier) Name() string {
	return "serverchan"
}
