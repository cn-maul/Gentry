package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/monitor"
	"github.com/cn-maul/Gentry/notify"
	"github.com/cn-maul/Gentry/web"
)

//go:embed frontend/dist
var frontendDist embed.FS

// Version 程序版本号，发布新版本时手动修改
const Version = "v1.1.2"

func main() {
	setupConsoleEncoding()
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("=== Gentry 网页变更监控系统 ===")

	// 1. 初始化数据库
	dbPath := "gentry.db"
	if err := database.Init(dbPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 加载推送开关状态
	if enabledVal, ok := database.GetSetting("notifications_enabled"); ok && enabledVal == "true" {
		notify.SetEnabled(true)
	}

	monitor.InitScanRules(os.Getenv("SCAN_RULES_FILE"))
	if upgraded, err := monitor.UpgradeKnownReusablePriceRules(); err != nil {
		log.Printf("[ScannerRules] 升级旧版价格规则失败: %v", err)
	} else if upgraded > 0 {
		log.Printf("[ScannerRules] 已将 %d 条旧版价格规则升级为跨商品规则", upgraded)
	}

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

	// 3. 载入前端（如果已构建）
	var frontendFS fs.FS
	if _, err := fs.ReadDir(frontendDist, "frontend/dist"); err == nil {
		sub, err := fs.Sub(frontendDist, "frontend/dist")
		if err == nil {
			frontendFS = sub
			log.Println("[Web] 前端已嵌入，管理界面可用")
		}
	} else {
		log.Println("[Web] 前端未构建，仅 API 模式运行（cd frontend && npm run dev）")
	}

	// 4. 启动投递服务
	deliverySvc := monitor.NewDeliveryService()
	deliverySvc.Start()

	// 5. 启动 Web 服务
	ws := web.NewWebServer(frontendFS, Version)
	go func() {
		addr := ":" + getPort()
		log.Printf("[Web] 服务启动: http://localhost%s", addr)
		if err := ws.Run(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Web 服务启动失败: %v", err)
		}
	}()

	// 6. 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("收到信号 %v，正在停止服务...", sig)

	// 先关 HTTP 拒绝新请求，再停监控器和投递服务
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := ws.Shutdown(httpCtx); err != nil {
		log.Printf("关闭 Web 服务超时: %v", err)
	}
	httpCancel()

	monitor.StopAll()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := deliverySvc.Stop(shutdownCtx); err != nil {
		log.Printf("停止投递服务超时: %v", err)
	}
	shutdownCancel()
	log.Println("Gentry 已安全退出")
}

// getPort 读取 PORT 环境变量，默认 8080
func getPort() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
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
