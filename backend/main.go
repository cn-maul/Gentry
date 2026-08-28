package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/monitor"
	"github.com/cn-maul/Gentry/notify"
	"github.com/cn-maul/Gentry/web"
)

// Version 程序版本号，发布新版本时手动修改
const Version = "v1.1.2"

func main() {
	setupConsoleEncoding()
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("=== Gentry 网页变更监控系统 ===")
	log.Println("[模式] 单服务模式（前端嵌入二进制，API 与界面同端口）")

	// 1. 初始化数据库（DB_PATH 环境变量可覆盖，默认当前目录 gentry.db）
	dbPath := getDBPath()
	if err := database.Init(dbPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 加载推送开关状态
	if enabledVal, ok := database.GetSetting("notifications_enabled"); ok && enabledVal == "true" {
		notify.SetEnabled(true)
	}

	monitor.InitScanRules(os.Getenv("SCAN_RULES_FILE"))

	// 2. 从数据库加载并启动所有活跃的监控器
	monitor.StartAllFromDB()

	// 2.5 数据保留清理：启动 1 分钟后先跑一次，之后每 24 小时一次。
	// goroutine 随进程退出即可，清理是幂等操作，中途被终止不影响数据一致性。
	go func() {
		time.Sleep(1 * time.Minute)
		if err := database.RunRetention(); err != nil {
			log.Printf("[DB] 数据保留清理失败: %v", err)
		}
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := database.RunRetention(); err != nil {
				log.Printf("[DB] 数据保留清理失败: %v", err)
			}
		}
	}()

	// 3. 启动 Web 服务（API + 嵌入式前端界面）
	ws := web.NewWebServer(Version)
	go func() {
		addr := ":" + getPort()
		log.Printf("[Web] 服务启动: http://localhost%s (界面) | http://localhost%s/api/v1 (API)", addr, addr)
		if err := ws.Run(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Web 服务启动失败: %v", err)
		}
	}()

	// 4. 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("收到信号 %v，正在停止服务...", sig)

	// 先关 HTTP 拒绝新请求，再停监控器
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := ws.Shutdown(httpCtx); err != nil {
		log.Printf("关闭 Web 服务超时: %v", err)
	}
	httpCancel()

	monitor.StopAll()
	log.Println("Gentry 已安全退出")
}

// getPort 读取 PORT 环境变量，默认 8080
func getPort() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}

// getDBPath 读取数据库文件路径：
//  1. DB_PATH 环境变量（完整路径）优先；
//  2. 否则若设置了 DATA_DIR，则使用 $DATA_DIR/gentry.db（Docker 挂载数据卷场景）；
//  3. 默认当前目录 gentry.db（本地运行）。
func getDBPath() string {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p
	}
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return filepath.Join(dir, "gentry.db")
	}
	return "gentry.db"
}

// setupConsoleEncoding 设置 Windows 控制台为 UTF-8 编码，确保中文正常显示
func setupConsoleEncoding() {
	if isWindows() {
		runCmd("chcp", "65001")
	}
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}
