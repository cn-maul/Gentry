package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseProxyURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantNil bool
		wantErr bool
	}{
		{name: "empty", value: "", wantNil: true},
		{name: "http", value: "http://127.0.0.1:7897"},
		{name: "socks", value: "socks5://127.0.0.1:1080"},
		{name: "missing scheme", value: "127.0.0.1:7897", wantErr: true},
		{name: "unsupported", value: "ftp://127.0.0.1:21", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseProxyURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseProxyURL() error = %v, wantErr %v", err, test.wantErr)
			}
			if (got == nil) != test.wantNil && !test.wantErr {
				t.Fatalf("parseProxyURL() = %v, wantNil %v", got, test.wantNil)
			}
		})
	}
}

func TestDirectHTTPClientDoesNotInheritEnvironmentProxy(t *testing.T) {
	client := newHTTPClient(time.Second, nil)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("direct client unexpectedly has a proxy function")
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{current: "v1.1.0", latest: "v1.1.1", want: true},
		{current: "v1.1.1", latest: "v1.1.1", want: false},
		{current: "v1.2.0", latest: "v1.1.9", want: false},
		{current: "v1.1", latest: "v1.1.1", want: true},
		{current: "dev", latest: "v9.0.0", want: false},
	}
	for _, test := range tests {
		if got := isNewerVersion(test.current, test.latest); got != test.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", test.current, test.latest, got, test.want)
		}
	}
}

func TestDoFetchReleaseDisablesCaching(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != updateUserAgent {
			t.Errorf("User-Agent = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.1.1","assets":[]}`))
	}))
	defer server.Close()

	release, err := doFetchRelease(server.URL, newHTTPClient(2*time.Second, nil))
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v1.1.1" {
		t.Fatalf("tag = %q", release.TagName)
	}
}

func TestDoDownloadValidatesSizeAndExecutable(t *testing.T) {
	header := []byte{'M', 'Z', 0, 0}
	if runtime.GOOS != "windows" {
		header = []byte{'\x7f', 'E', 'L', 'F'}
	}
	payload := append(header, []byte("test executable")...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "update.bin")
	client := newHTTPClient(2*time.Second, nil)
	if err := doDownload(server.URL, dest, int64(len(payload)), client); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	if err := doDownload(server.URL, dest, int64(len(payload)+1), client); err == nil || !strings.Contains(err.Error(), "文件大小不匹配") {
		t.Fatalf("expected size mismatch, got %v", err)
	}
}

func TestDoDownloadResumesPartialFile(t *testing.T) {
	header := []byte{'M', 'Z', 0, 0}
	if runtime.GOOS != "windows" {
		header = []byte{'\x7f', 'E', 'L', 'F'}
	}
	payload := append(header, []byte("resume payload")...)
	requestedRange := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedRange = r.Header.Get("Range")
		if requestedRange == "bytes=5-" {
			w.Header().Set("Content-Range", "bytes 5-"+strconv.Itoa(len(payload)-1)+"/"+strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(payload[5:])
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "resume.bin")
	if err := os.WriteFile(dest, payload[:5], 0600); err != nil {
		t.Fatal(err)
	}
	if err := doDownload(server.URL, dest, int64(len(payload)), newHTTPClient(2*time.Second, nil)); err != nil {
		t.Fatal(err)
	}
	if requestedRange != "bytes=5-" {
		t.Fatalf("Range = %q", requestedRange)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("payload = %q", got)
	}
}
