package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchRetriesOnTimeoutThenSucceeds(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			// 第一次直接挂起直至客户端超时
			select {
			case <-time.After(3 * time.Second):
			case <-r.Context().Done():
			}
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	// 短超时 + 1 次重试，确保测试快速完成
	f := New(WithTimeout(200*time.Millisecond), WithMaxRetries(1), WithRetryDelay(10*time.Millisecond))
	body, err := f.Fetch(server.URL)
	if err != nil {
		t.Fatalf("fetch should succeed after retry: %v", err)
	}
	if body != "ok" {
		t.Fatalf("unexpected body: %q", body)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestFetchDoesNotRetryHTTPStatusError(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	f := New(WithMaxRetries(3), WithRetryDelay(10*time.Millisecond))
	_, err := f.Fetch(server.URL)
	if err == nil {
		t.Fatal("expected HTTP 404 error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("HTTP errors must not be retried, attempts = %d", attempts.Load())
	}
}

func TestFetchRetriesOnConnectionError(t *testing.T) {
	// 先开一个服务器拿到端口，再关闭它，模拟连接失败
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	f := New(WithMaxRetries(1), WithRetryDelay(10*time.Millisecond))
	_, err := f.Fetch(url)
	if err == nil {
		t.Fatal("expected connection error to surface after retries")
	}
}

func TestFetchRetriesUntilDeadline(t *testing.T) {
	// 带截止时间的 context 超时后应返回 ctx.Err，而不是继续重试
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer server.Close()

	f := New(WithTimeout(5*time.Second), WithMaxRetries(5), WithRetryDelay(500*time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := f.FetchContext(ctx, server.URL)
	if err == nil {
		t.Fatal("expected deadline error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("retries must respect context deadline, took %v", elapsed)
	}
}
