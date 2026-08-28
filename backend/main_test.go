package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDBPath(t *testing.T) {
	cases := []struct {
		name     string
		dbPath   string
		dataDir  string
		wantSub  string // 期望路径包含的子串（相对比较，避免依赖临时目录具体值）
		notEqual string // 期望不等于的值
	}{
		{"default uses current dir", "", "", "gentry.db", ""},
		{"DB_PATH wins over DATA_DIR", filepath.Join("custom", "x.db"), "/data", filepath.Join("custom", "x.db"), ""},
		{"DATA_DIR places db inside dir", "", filepath.Join("vol", "data"), filepath.Join("vol", "data", "gentry.db"), "gentry.db"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.dbPath != "" {
				t.Setenv("DB_PATH", tc.dbPath)
			} else {
				os.Unsetenv("DB_PATH")
			}
			if tc.dataDir != "" {
				t.Setenv("DATA_DIR", tc.dataDir)
			} else {
				os.Unsetenv("DATA_DIR")
			}

			got := getDBPath()
			if !filepath.IsAbs(got) && tc.wantSub != "" && got != tc.wantSub && !containsPath(got, tc.wantSub) {
				t.Fatalf("getDBPath() = %q, want containing %q", got, tc.wantSub)
			}
			if tc.notEqual != "" && got == tc.notEqual {
				t.Fatalf("getDBPath() = %q, should not be %q", got, tc.notEqual)
			}
		})
	}
}

func containsPath(path, sub string) bool {
	return path == sub || (len(path) > len(sub) && path[len(path)-len(sub):] == sub)
}

func TestGetDBPathCreatesDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DB_PATH", "")
	t.Setenv("DATA_DIR", filepath.Join(dir, "nested", "data"))
	got := getDBPath()
	if _, err := os.Stat(filepath.Dir(got)); err != nil {
		t.Fatalf("data dir should be auto-created: %v", err)
	}
}
