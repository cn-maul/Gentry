package web

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cn-maul/Gentry/monitor"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// getAllowedOrigins 获取允许的 CORS 来源
// 支持通过环境变量 ALLOWED_ORIGINS 配置，多个来源用逗号分隔
// 默认允许所有来源（适用于 API 模式）
func getAllowedOrigins() []string {
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		return []string{"*"}
	}
	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (s *WebServer) setupRoutes() {
	// CORS 配置
	s.engine.Use(cors.New(cors.Config{
		AllowOrigins:     getAllowedOrigins(),
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API v1 路由组
	v1 := s.engine.Group("/api/v1")
	{
		// 需要认证的路由
		authenticated := v1.Group("")
		authenticated.Use(requireAuth())
		{
			// 系统
			authenticated.GET("/health", s.healthCheck)
			authenticated.GET("/stats", s.getStats)
			authenticated.GET("/groups", s.listGroups)
			authenticated.GET("/settings/notifications", s.getNotificationSettings)
			authenticated.PUT("/settings/notifications", s.updateNotificationSettings)

			// 监控器管理
			monitors := authenticated.Group("/monitors")
			{
				monitors.GET("/", s.listMonitors)
				monitors.GET("/:name", s.getMonitor)
				monitors.POST("/", s.addMonitor)
				monitors.PUT("/:name", s.updateMonitor)
				monitors.DELETE("/:name", s.removeMonitor)
				monitors.POST("/:name/start", s.startMonitor)
				monitors.POST("/:name/stop", s.stopMonitor)
				monitors.GET("/:name/updates", s.getUpdates)
				monitors.GET("/:name/config", s.getMonitorConfig)
				monitors.PUT("/:name/mark-all-notified", s.markAllNotified)
				monitors.POST("/:name/mark-read", s.markRead)
				monitors.PUT("/:name/notify-accounts", s.updateNotifyAccounts)

				// 新引擎 API
				monitors.GET("/:name/events", s.getMonitorEvents)
				monitors.GET("/:name/snapshots", s.getMonitorSnapshots)
				monitors.POST("/:name/baseline", s.resetBaseline)
				monitors.POST("/:name/check", s.manualCheck)
				monitors.POST("/validate", s.validateMonitorConfig)
			}

			// 智能扫描（在 monitors 组之外，避免 :name 通配符冲突）
			authenticated.POST("/monitors/preview", s.previewScan)
			authenticated.POST("/monitors/smart-create", s.smartCreate)

			// 推送账户 CRUD
			authenticated.GET("/settings/notification-accounts", s.listAccounts)
			authenticated.POST("/settings/notification-accounts", s.createAccount)
			authenticated.PUT("/settings/notification-accounts/:id", s.updateAccount)
			authenticated.DELETE("/settings/notification-accounts/:id", s.deleteAccount)

			// 扫描规则模板 CRUD
			authenticated.GET("/settings/scan-rules", s.listScanRules)
			authenticated.GET("/settings/scan-rules/export", s.exportScanRules)
			authenticated.POST("/settings/scan-rules/import", s.importScanRules)
			authenticated.POST("/settings/scan-rules/quick", s.quickCreateScanRule)
			authenticated.POST("/settings/scan-rules", s.createScanRule)
			authenticated.PUT("/settings/scan-rules/:id", s.updateScanRule)
			authenticated.DELETE("/settings/scan-rules/:id", s.deleteScanRule)
			authenticated.POST("/settings/scan-rules/:id/test", s.testScanRule)

			// 推送服务供应商元数据
			authenticated.GET("/settings/notification-providers", s.listNotificationProviders)

			// 更新相关
			authenticated.GET("/update/status", s.getUpdateStatus)
			authenticated.POST("/update/apply", s.applyUpdate)
			authenticated.GET("/update/proxy", s.getUpdateProxy)
			authenticated.PUT("/update/proxy", s.setUpdateProxy)
		}

		// 公开接口（不需要认证）
		v1.GET("/version", s.getVersion)
		v1.GET("/update/check", s.checkUpdate)
	}
}

func (s *WebServer) listMonitors(c *gin.Context) {
	monitors := monitor.GetAllMonitors()
	c.JSON(http.StatusOK, NewSuccessResponse(monitors))
}

func (s *WebServer) getMonitor(c *gin.Context) {
	name := c.Param("name")
	m := monitor.GetMonitor(name)
	if m == nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}
	c.JSON(http.StatusOK, NewSuccessResponse(m.GetStatus()))
}

func NewSuccessResponse(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func NewErrorResponse(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}
