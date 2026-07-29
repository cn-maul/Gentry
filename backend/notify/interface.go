package notify

// Notifier 通用推送接口
type Notifier interface {
	// Send 发送通知
	// title: 通知标题
	// content: 通知内容
	// 返回: 错误信息
	Send(title, content string) error

	// Name 返回服务名称
	Name() string
}
