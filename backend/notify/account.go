package notify

import (
	"encoding/json"
	"fmt"

	"github.com/cn-maul/Gentry/database"
)

// SendToAccount 使用指定推送账户发送通知
func SendToAccount(account *database.NotificationAccount, title, content string) error {
	creator, ok := providers[account.Service]
	if !ok {
		return fmt.Errorf("未注册的推送服务: %s", account.Service)
	}

	var config map[string]interface{}
	if account.ConfigJSON != "" {
		if err := json.Unmarshal([]byte(account.ConfigJSON), &config); err != nil {
			return fmt.Errorf("解析账户配置失败: %w", err)
		}
	}

	notifier, err := creator(config)
	if err != nil {
		return fmt.Errorf("创建推送实例失败: %w", err)
	}

	return notifier.Send(title, content)
}
