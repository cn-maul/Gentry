package fetcher

import (
	"net/http"
	"time"
)

type Option func(*config)

// WithTimeout 设置请求超时
func WithTimeout(d time.Duration) Option {
	return func(c *config) {
		if d <= 0 {
			panic("timeout must be positive")
		}
		c.client.Timeout = d
	}
}

// WithUserAgent 设置User-Agent
func WithUserAgent(ua string) Option {
	return func(c *config) {
		if ua == "" {
			panic("user agent cannot be empty")
		}
		c.userAgent = ua
	}
}

// WithClient 完全自定义http.Client
func WithClient(cli *http.Client) Option {
	return func(c *config) {
		if cli == nil {
			panic("http.Client cannot be nil")
		}
		c.client = cli // 注意：调用方需自行保证线程安全
	}
}

// WithMaxRetries 设置网络层错误（超时/连接重置等）的最大重试次数。
// 默认 1 次；设为 0 可关闭重试。
func WithMaxRetries(n int) Option {
	return func(c *config) {
		if n < 0 {
			panic("max retries cannot be negative")
		}
		c.maxRetries = n
	}
}

// WithRetryDelay 设置相邻两次重试之间的等待时间。默认 500ms。
func WithRetryDelay(d time.Duration) Option {
	return func(c *config) {
		if d < 0 {
			panic("retry delay cannot be negative")
		}
		c.retryDelay = d
	}
}
