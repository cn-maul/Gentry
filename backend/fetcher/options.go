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
