package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCompleteRetriesTransientHTTPError(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("upstream hiccup"))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`))
	}))
	defer server.Close()

	resp, err := defaultProvider.Complete(context.Background(), Config{BaseURL: server.URL, Model: "m"}, CompleteRequest{
		SystemPrompt: "s", UserPrompt: "u",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("usage not parsed: %+v", resp.Usage)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", got)
	}
}

func TestCompleteDoesNotRetryAuthError(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer server.Close()

	_, err := defaultProvider.Complete(context.Background(), Config{BaseURL: server.URL, Model: "m"}, CompleteRequest{UserPrompt: "u"})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !IsFatal(err) {
		t.Fatalf("auth error should be fatal: %v", err)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected status in message: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("auth errors must not retry, got %d calls", got)
	}
}

func TestIsFatalFalseForNetworkError(t *testing.T) {
	perr := &ProviderError{Kind: KindNetwork, Wrapped: fmt.Errorf("dial timeout")}
	var target *ProviderError
	if !errorsAs(perr, &target) || IsFatal(perr) {
		t.Fatalf("network error should not be fatal")
	}
	if !strings.Contains(perr.Error(), "请求 AI 服务失败") {
		t.Fatalf("legacy message format broken: %v", perr)
	}
}

func TestExtractJSONObjectHandlesFencesAndProse(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "fenced with trailing prose",
			in:   "```json\n{\"container\":\"ul\",\"item\":\"li\"}\n```\n补充说明",
			want: `{"container":"ul","item":"li"}`,
		},
		{
			name: "prose braces before real object",
			in:   `说明文字 {示例} 正文 {"container":"ul","item":"li","fields":[]}`,
			want: `{"container":"ul","item":"li","fields":[]}`,
		},
		{
			name: "braces inside string values",
			in:   `{"a":"x {y} z","b":{"c":1}}`,
			want: `{"a":"x {y} z","b":{"c":1}}`,
		},
		{
			name: "escaped quotes inside strings",
			in:   `前缀 {"a":"他说：\"} 快跑\"","b":2} 后缀`,
			want: `{"a":"他说：\"} 快跑\"","b":2}`,
		},
		{
			name: "nested objects",
			in:   `结果 {"outer":{"inner":[1,2]},"tail":"}"}`,
			want: `{"outer":{"inner":[1,2]},"tail":"}"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractJSONObject(tc.in)
			if err != nil {
				t.Fatalf("ExtractJSONObject(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
		})
	}

	if _, err := ExtractJSONObject("模型拒绝分析该页面"); err == nil {
		t.Fatal("expected error for non-json answer")
	}
	unbalanced := `{"container":"ul","item":`
	if _, err := ExtractJSONObject(unbalanced); err == nil {
		t.Fatal("expected error for unbalanced object")
	}
}

// TestChatUsesProviderShape 保持 /chat/completions 请求体与历史版本一致，
// 防止兼容封装意外破坏协议字段。
func TestChatKeepsLegacyRequestShape(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		body = string(buf)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, APIKey: "sk-test", Model: "test-model"}
	answer, err := Chat(context.Background(), cfg, "sys", "usr")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if answer != "pong" {
		t.Fatalf("unexpected answer: %q", answer)
	}
	for _, want := range []string{`"model":"test-model"`, `"temperature":0`, `"stream":false`, `"role":"system"`, `"role":"user"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("request body missing %s: %s", want, body)
		}
	}
	if strings.Contains(body, "response_format") {
		t.Fatalf("JSON mode must be opt-in, body: %s", body)
	}
}
