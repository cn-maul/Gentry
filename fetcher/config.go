package fetcher

import (
	"net/http"
	"time"
)

// 默认配置（私有变量，外部不可修改）
var (
	defaultTimeout   = 10 * time.Second
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
)

type config struct {
	client    *http.Client // 所有网络操作委托给标准http.Client
	userAgent string       // User-Agent单独管理（高频使用字段）
}

// 深拷贝默认配置
func newDefaultConfig() *config {
	return &config{
		client: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				ForceAttemptHTTP2:   true,
			},
		},
		userAgent: defaultUserAgent,
	}
}
