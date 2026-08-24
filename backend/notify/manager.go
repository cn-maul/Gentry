package notify

import (
	"sync"
)

var (
	notificationsOn bool = false // 默认关闭推送
	settingsLock    sync.RWMutex
)

// SetEnabled 设置推送开关
func SetEnabled(enabled bool) {
	settingsLock.Lock()
	defer settingsLock.Unlock()
	notificationsOn = enabled
}

// IsEnabled 推送是否开启
func IsEnabled() bool {
	settingsLock.RLock()
	defer settingsLock.RUnlock()
	return notificationsOn
}
