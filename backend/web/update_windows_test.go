//go:build windows

package web

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	windowsUpdateChildEnv = "GENTRY_WINDOWS_UPDATE_TEST_CHILD"
	windowsUpdateNewEnv   = "GENTRY_WINDOWS_UPDATE_TEST_NEW"
)

func TestMain(m *testing.M) {
	if os.Getenv(windowsUpdateNewEnv) == "1" {
		marker := filepath.Join(filepath.Dir(mustExecutable()), "new-process.txt")
		_ = os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0600)
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestWindowsUpgradeScriptWaitsAndRollsBack(t *testing.T) {
	script := windowsUpgradeScript(`C:\app\gentry.exe`, `C:\app\.gentry-new.exe`, `C:\app`, `C:\app\.ready`, 1234)
	for _, expected := range []string{
		"Wait-Process -Id 1234",
		"Start-Sleep -Seconds 2",
		"if ($newProcess.HasExited)",
		"Move-Item -LiteralPath $backup -Destination $exe -Force",
		"restored and restarted the previous version",
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("script missing %q", expected)
		}
	}
}

func TestWindowsUpgradeEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "gentry-windows-amd64-v1.1.0.exe")
	newPath := filepath.Join(tempDir, ".gentry-new.exe")
	copyTestExecutable(t, mustExecutable(), oldPath)
	copyTestExecutable(t, mustExecutable(), newPath)

	child := exec.Command(oldPath, "-test.run=^TestWindowsUpgradeChild$")
	child.Dir = tempDir
	child.Env = append(os.Environ(), windowsUpdateChildEnv+"=1", windowsUpdateNewEnv+"=")
	output, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("updater child failed: %v\n%s", err, output)
	}

	marker := filepath.Join(tempDir, "new-process.txt")
	deadline := time.Now().Add(20 * time.Second)
	for !fileExists(marker) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if !fileExists(marker) {
		logContents, _ := os.ReadFile(filepath.Join(tempDir, ".gentry-update.log"))
		scriptContents, _ := os.ReadFile(filepath.Join(tempDir, ".gentry-upgrade.ps1"))
		t.Fatalf("new process did not start; helper log:\n%s\nhelper script:\n%s", logContents, scriptContents)
	}

	scriptPath := filepath.Join(tempDir, ".gentry-upgrade.ps1")
	for fileExists(scriptPath) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if fileExists(scriptPath) {
		t.Fatal("upgrade helper did not finish")
	}

	pidBytes, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if process, findErr := os.FindProcess(pid); findErr == nil {
		_ = process.Kill()
		_, _ = process.Wait()
	}

	if !fileExists(oldPath + ".bak") {
		t.Fatal("previous executable backup was not retained")
	}
	if !fileExists(oldPath) {
		t.Fatal("new executable was not moved to the original path")
	}
	if fileExists(newPath) {
		t.Fatal("temporary new executable still exists")
	}
}

func TestWindowsUpgradeRollsBackWhenNewExecutableFails(t *testing.T) {
	tempDir := t.TempDir()
	oldPath := filepath.Join(tempDir, "gentry-windows-amd64-v1.1.0.exe")
	newPath := filepath.Join(tempDir, ".gentry-new.exe")
	copyTestExecutable(t, mustExecutable(), oldPath)
	if err := os.WriteFile(newPath, []byte("not an executable"), 0700); err != nil {
		t.Fatal(err)
	}

	child := exec.Command(oldPath, "-test.run=^TestWindowsUpgradeChild$")
	child.Dir = tempDir
	child.Env = append(os.Environ(), windowsUpdateChildEnv+"=1", windowsUpdateNewEnv+"=")
	output, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("updater child failed: %v\n%s", err, output)
	}

	marker := filepath.Join(tempDir, "new-process.txt")
	deadline := time.Now().Add(20 * time.Second)
	for !fileExists(marker) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if !fileExists(marker) {
		logContents, _ := os.ReadFile(filepath.Join(tempDir, ".gentry-update.log"))
		t.Fatalf("restored process did not start; helper log:\n%s", logContents)
	}

	scriptPath := filepath.Join(tempDir, ".gentry-upgrade.ps1")
	for fileExists(scriptPath) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	stopMarkedProcess(t, marker)

	if !fileExists(oldPath) {
		t.Fatal("previous executable was not restored")
	}
	if fileExists(oldPath + ".bak") {
		t.Fatal("backup still exists after rollback")
	}
	logContents, err := os.ReadFile(filepath.Join(tempDir, ".gentry-update.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logContents), "restored and restarted the previous version") {
		t.Fatalf("rollback was not logged:\n%s", logContents)
	}
}

func TestWindowsUpgradeChild(t *testing.T) {
	if os.Getenv(windowsUpdateChildEnv) != "1" {
		t.Skip("helper process only")
	}
	if err := os.Setenv(windowsUpdateNewEnv, "1"); err != nil {
		t.Fatal(err)
	}
	execPath := mustExecutable()
	if err := restartWindows(execPath, filepath.Join(filepath.Dir(execPath), ".gentry-new.exe")); err != nil {
		t.Fatal(err)
	}
}

func copyTestExecutable(t *testing.T, source, dest string) {
	t.Helper()
	in, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func mustExecutable() string {
	path, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("os.Executable: %v", err))
	}
	path, err = filepath.Abs(path)
	if err != nil {
		panic(fmt.Sprintf("filepath.Abs: %v", err))
	}
	return path
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func stopMarkedProcess(t *testing.T, marker string) {
	t.Helper()
	pidBytes, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if process, findErr := os.FindProcess(pid); findErr == nil {
		_ = process.Kill()
		_, _ = process.Wait()
	}
}
