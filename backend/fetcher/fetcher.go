package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type Fetcher struct {
	config *config // 持有私有配置的不可变副本
}

// New 创建Fetcher实例（线程安全）
func New(opts ...Option) *Fetcher {
	cfg := newDefaultConfig() // 深拷贝默认配置
	for _, opt := range opts {
		opt(cfg) // 应用用户配置
	}
	return &Fetcher{config: cfg}
}

// Fetch 执行HTTP请求
func (f *Fetcher) Fetch(url string) (string, error) {
	return f.FetchContext(context.Background(), url)
}

// FetchContext 执行支持取消和超时传递的 HTTP 请求。
func (f *Fetcher) FetchContext(ctx context.Context, url string) (string, error) {
	return f.FetchContextWithHeaders(ctx, url, nil)
}

// FetchContextWithHeaders executes a request with a small caller-provided
// header set. Hop-by-hop and host headers are never accepted.
// 网络层错误（超时、连接重置、DNS 等）会自动重试 maxRetries 次；
// HTTP 状态码错误（如 404/500）不会重试，因为重试无意义且可能放大压力。
func (f *Fetcher) FetchContextWithHeaders(ctx context.Context, url string, headers map[string]string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var lastErr error
	for attempt := 0; attempt <= f.config.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(f.config.retryDelay):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		body, err := f.fetchOnce(ctx, url, headers)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isRetryableNetworkError(err) {
			return "", err
		}
	}
	return "", lastErr
}

// fetchOnce 执行单次 HTTP 请求。
func (f *Fetcher) fetchOnce(ctx context.Context, url string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置浏览器兼容的默认请求头；数据源只能覆盖受控白名单。
	req.Header.Set("User-Agent", f.config.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	for name, value := range headers {
		canonical := http.CanonicalHeaderKey(strings.TrimSpace(name))
		switch canonical {
		case "Accept", "Accept-Language", "Referer", "X-Requested-With":
			req.Header.Set(canonical, value)
		}
	}

	// 执行请求（所有网络行为委托给http.Client）
	resp, err := f.config.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 读取响应（限制10MB内存）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", fmt.Errorf("读取失败: %w", err)
	}

	return string(body), nil
}

// isRetryableNetworkError 判断错误是否属于可重试的网络层错误。
// context deadline exceeded（客户端超时）、连接重置、DNS 失败等均属于此类；
// 服务端明确返回的 HTTP 状态错误不可重试。
func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

/*

	// 基础用法
	f1 := fetcher.New()
	result, err := f1.Fetch("https://example.com")

	// 自定义配置
	f2 := fetcher.New(
		fetcher.WithTimeout(10 * time.Second),
		fetcher.WithUserAgent("MyBot/2.0"),
	)
	result, _ = f2.Fetch("https://api.example.com")

	// 完全自定义Client
	customClient := &http.Client{Timeout: 3 * time.Second}
	f3 := fetcher.New(fetcher.WithClient(customClient))
	result, _ = f3.Fetch("https://internal.example.com")

*/
