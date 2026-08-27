package web

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// getAllowedOrigins 获取允许的 CORS 来源。
// 支持通过环境变量 ALLOWED_ORIGINS 配置，多个来源用逗号分隔；默认允许所有来源。
func getAllowedOrigins() []string {
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		return []string{"*"}
	}
	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// setupRoutes 注册 CORS、API v1 路由与 SPA 静态资源。
func (s *WebServer) setupRoutes() {
	s.engine.Use(cors.New(cors.Config{
		AllowOrigins:     getAllowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	v1 := s.engine.Group("/api/v1")
	authenticated := v1.Group("", requireAuth())
	s.registerSystemRoutes(authenticated)
	s.registerMonitorRoutes(authenticated)
	s.registerAccountRoutes(authenticated)
	s.registerScanRuleRoutes(authenticated)
	s.registerUpdateRoutes(authenticated)

	// 公开接口（不需要认证）
	v1.GET("/version", s.getVersion)
	v1.GET("/update/check", s.checkUpdate)

	s.setupSPA()
}

func (s *WebServer) registerSystemRoutes(group *gin.RouterGroup) {
	group.GET("/health", healthCheck)
	group.GET("/stats", getStats)
	group.GET("/groups", listGroups)
	group.GET("/settings/notifications", getNotificationSettings)
	group.PUT("/settings/notifications", updateNotificationSettings)
	group.GET("/settings/categories", listCategories)
	group.POST("/settings/categories", createCategory)
	group.PUT("/settings/categories/:id", renameCategory)
	group.DELETE("/settings/categories/:id", deleteCategory)
}

func (s *WebServer) registerMonitorRoutes(group *gin.RouterGroup) {
	monitors := group.Group("/monitors")
	monitors.GET("/", listMonitors)
	monitors.GET("/:name", getMonitor)
	monitors.POST("/", addMonitor)
	monitors.PUT("/:name", updateMonitor)
	monitors.DELETE("/:name", removeMonitor)
	monitors.POST("/:name/start", startMonitor)
	monitors.POST("/:name/stop", stopMonitor)
	monitors.GET("/:name/updates", getUpdates)
	monitors.GET("/:name/config", getMonitorConfig)
	monitors.PUT("/:name/mark-all-notified", markAllNotified)
	monitors.POST("/:name/mark-read", markRead)
	monitors.PUT("/:name/notify-accounts", updateNotifyAccounts)
	monitors.POST("/:name/check", manualCheck)
	monitors.POST("/validate", validateMonitorConfig)
	// 智能扫描（在 :name 之外，避免通配符冲突）
	monitors.POST("/preview", previewScan)
}

func (s *WebServer) registerAccountRoutes(group *gin.RouterGroup) {
	accounts := group.Group("/settings/notification-accounts")
	accounts.GET("", listAccounts)
	accounts.POST("", createAccount)
	accounts.PUT("/:id", updateAccount)
	accounts.DELETE("/:id", deleteAccount)

	group.GET("/settings/notification-providers", listNotificationProviders)
}

func (s *WebServer) registerScanRuleRoutes(group *gin.RouterGroup) {
	rules := group.Group("/settings/scan-rules")
	rules.GET("", listScanRules)
	rules.GET("/export", exportScanRules)
	rules.POST("/import", importScanRules)
	rules.POST("/quick", quickCreateScanRule)
	rules.POST("/ai-extract", s.aiExtractScanRule)
	rules.POST("", createScanRule)
	rules.PUT("/:id", updateScanRule)
	rules.DELETE("/:id", deleteScanRule)
	rules.POST("/:id/test", testScanRule)
	rules.POST("/capture", captureScanRule)
	rules.POST("/test-draft", testDraftMonitor)
}

func (s *WebServer) registerUpdateRoutes(group *gin.RouterGroup) {
	group.GET("/update/status", s.getUpdateStatus)
	group.POST("/update/apply", s.applyUpdate)
	group.GET("/update/proxy", s.getUpdateProxy)
	group.PUT("/update/proxy", s.setUpdateProxy)

	// AI 模型接入
	group.GET("/settings/llm", s.getLLMSettings)
	group.PUT("/settings/llm", s.updateLLMSettings)
	group.POST("/settings/llm/test", s.testLLMSettings)
}
