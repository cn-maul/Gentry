package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cn-maul/Gentry/database"
)

func setupLLMTestDB(t *testing.T) {
	t.Helper()
	original := database.DB
	dir, err := os.MkdirTemp("", "gentry-llm-test-*")
	if err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := database.Init(filepath.Join(dir, "llm-test.db")); err != nil {
		t.Fatalf("init test database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		database.DB = original
		_ = os.RemoveAll(dir)
	})
}

func TestSaveAndLoadConfig(t *testing.T) {
	setupLLMTestDB(t)

	if _, err := LoadConfig(); err != ErrNotConfigured {
		t.Fatalf("empty config should return ErrNotConfigured, got %v", err)
	}

	if err := SaveConfig(Config{BaseURL: "ftp://example.com", Model: "gpt"}); err == nil {
		t.Fatal("non-http base url should be rejected")
	}
	if err := SaveConfig(Config{BaseURL: "https://api.example.com/v1"}); err == nil {
		t.Fatal("missing model should be rejected")
	}

	if err := SaveConfig(Config{BaseURL: " https://api.example.com/v1 ", APIKey: " sk-test ", Model: " deepseek-chat "}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.BaseURL != "https://api.example.com/v1" || loaded.APIKey != "sk-test" || loaded.Model != "deepseek-chat" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
	if !loaded.Configured() {
		t.Fatal("config should be configured")
	}
}

func TestChatCallsChatCompletions(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"正常"}}]}`))
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, APIKey: "sk-test", Model: "test-model"}
	answer, err := Chat(context.Background(), cfg, "system-prompt", "user-prompt")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if answer != "正常" {
		t.Fatalf("answer = %q", answer)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("authorization header = %q", gotAuth)
	}
	if !strings.HasSuffix(gotPath, "/chat/completions") {
		t.Fatalf("request path = %q", gotPath)
	}
	if gotBody.Model != "test-model" || gotBody.Temperature != 0 {
		t.Fatalf("unexpected request body: %+v", gotBody)
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Role != "system" || gotBody.Messages[1].Content != "user-prompt" {
		t.Fatalf("unexpected messages: %+v", gotBody.Messages)
	}
}

func TestChatSurfacesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, APIKey: "bad", Model: "m"}
	if _, err := Chat(context.Background(), cfg, "s", "u"); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got %v", err)
	}
}
