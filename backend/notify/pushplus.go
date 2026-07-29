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
	RegisterWithMetadata("pushplus", newPushPlusNotifier, &ProviderMetadata{
		Label:          "PushPlus",
		RequiredFields: []string{"token"},
		OptionalFields: []string{"channel", "template"},
	})
}

func newPushPlusNotifier(config map[string]interface{}) (Notifier, error) {
	token, _ := config["token"].(string)
	if token == "" {
		return nil, fmt.Errorf("缺少必需的token参数")
	}

	channel, _ := config["channel"].(string)
	tmpl, _ := config["template"].(string)
	return &pushPlusNotifier{
		token:    token,
		channel:  channel,
		template: tmpl,
	}, nil
}

type pushPlusResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

type pushPlusNotifier struct {
	token    string
	channel  string
	template string
}

func (p *pushPlusNotifier) Send(title, content string) error {
	payload := map[string]interface{}{
		"token":     p.token,
		"title":     title,
		"content":   content + "\n",
		"timestamp": time.Now().UnixMilli(),
	}
	if p.channel != "" {
		payload["channel"] = p.channel
	}
	if p.template != "" {
		payload["template"] = p.template
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		"https://www.pushplus.plus/send",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	var result pushPlusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败 (status=%d, body=%s)", resp.StatusCode, string(body))
	}

	switch result.Code {
	case 200:
		return nil
	case 900:
		return fmt.Errorf("pushplus 账号受限，今日不再重试: %s", result.Msg)
	case 903:
		return fmt.Errorf("pushplus token 无效: %s", result.Msg)
	case 888:
		return fmt.Errorf("pushplus 积分不足: %s", result.Msg)
	case 905:
		return fmt.Errorf("pushplus 账户未实名认证: %s", result.Msg)
	default:
		return fmt.Errorf("pushplus 返回错误 code=%d: %s", result.Code, result.Msg)
	}
}

func (p *pushPlusNotifier) Name() string {
	return "pushplus"
}
