//go:build windows

package web

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func restartWindows(execPath, newFile string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = filepath.Dir(execPath)
	}
	scriptPath := filepath.Join(filepath.Dir(execPath), ".gentry-upgrade.ps1")
	readyPath := filepath.Join(filepath.Dir(execPath), ".gentry-upgrade-ready")
	_ = os.Remove(readyPath)
	script := windowsUpgradeScript(execPath, newFile, workingDir, readyPath, os.Getpid())
	content := append([]byte{0xef, 0xbb, 0xbf}, []byte(script)...)
	if err := os.WriteFile(scriptPath, content, 0600); err != nil {
		return fmt.Errorf("创建升级脚本失败: %w", err)
	}

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000,
		HideWindow:    true,
	}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("启动升级脚本失败: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			_ = os.Remove(scriptPath)
			return fmt.Errorf("升级辅助进程启动超时")
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Release()
	writeUpdateLog("info", "升级辅助进程已启动，主程序即将退出")
	os.Exit(0)
	return nil
}

func windowsUpgradeScript(execPath, newFile, workingDir, readyPath string, pid int) string {
	backupFile := execPath + ".bak"
	logPath := filepath.Join(filepath.Dir(execPath), ".gentry-update.log")
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$exe = %s
$newExe = %s
$backup = %s
$workingDir = %s
$logPath = %s
$readyPath = %s
$oldMoved = $false
$newProcess = $null

function Write-UpgradeLog([string]$message) {
    Add-Content -LiteralPath $logPath -Encoding UTF8 -Value ('[{0}] helper: {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $message)
}

Set-Content -LiteralPath $readyPath -Value 'ready' -Encoding ASCII
try {
    Write-UpgradeLog 'waiting for the old process to exit'
    Wait-Process -Id %d -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $backup) {
        Remove-Item -LiteralPath $backup -Force
    }
    Move-Item -LiteralPath $exe -Destination $backup -Force
    $oldMoved = $true
    Move-Item -LiteralPath $newExe -Destination $exe -Force
    Start-Sleep -Seconds 2
    $newProcess = Start-Process -FilePath $exe -WorkingDirectory $workingDir -PassThru
    Start-Sleep -Seconds 5
    if ($newProcess.HasExited) {
        throw ('new process exited immediately with code {0}' -f $newProcess.ExitCode)
    }
    Write-UpgradeLog ('new process started, PID {0}' -f $newProcess.Id)
} catch {
    Write-UpgradeLog ('upgrade failed: {0}' -f $_.Exception.Message)
    if ($newProcess -and -not $newProcess.HasExited) {
        Stop-Process -Id $newProcess.Id -Force -ErrorAction SilentlyContinue
    }
    if ($oldMoved -and (Test-Path -LiteralPath $backup)) {
        if (Test-Path -LiteralPath $exe) {
            Remove-Item -LiteralPath $exe -Force
        }
        Move-Item -LiteralPath $backup -Destination $exe -Force
        Start-Process -FilePath $exe -WorkingDirectory $workingDir
        Write-UpgradeLog 'restored and restarted the previous version'
    }
}

Remove-Item -LiteralPath $readyPath -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $PSCommandPath -Force -ErrorAction SilentlyContinue
`, psQuote(execPath), psQuote(newFile), psQuote(backupFile), psQuote(workingDir), psQuote(logPath), psQuote(readyPath), pid)
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
