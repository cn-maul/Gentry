package web

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/cn-maul/Gentry/monitor"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func (s *WebServer) setupRoutes() {
	s.engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authenticated := s.engine.Group("/api")
	authenticated.Use(requireAuth())
	{
		authenticated.GET("/health", s.healthCheck)
		authenticated.GET("/stats", s.getStats)
		authenticated.GET("/groups", s.listGroups)
		authenticated.GET("/settings/notifications", s.getNotificationSettings)
		authenticated.PUT("/settings/notifications", s.updateNotificationSettings)
	}

	api := authenticated.Group("/v1/monitors")
	{
		api.GET("/", s.listMonitors)
		api.GET("/:name", s.getMonitor)
		api.POST("/", s.addMonitor)
		api.PUT("/:name", s.updateMonitor)
		api.DELETE("/:name", s.removeMonitor)
		api.POST("/:name/start", s.startMonitor)
		api.POST("/:name/stop", s.stopMonitor)
		api.GET("/:name/updates", s.getUpdates)
		api.GET("/:name/config", s.getMonitorConfig)
		api.PUT("/:name/mark-all-notified", s.markAllNotified)
		api.POST("/:name/mark-read", s.markRead)
		api.PUT("/:name/notify-accounts", s.updateNotifyAccounts)

		// 新引擎 API
		api.GET("/:name/events", s.getMonitorEvents)
		api.GET("/:name/snapshots", s.getMonitorSnapshots)
		api.POST("/:name/baseline", s.resetBaseline)
		api.POST("/:name/check", s.manualCheck)
		api.POST("/validate", s.validateMonitorConfig)
	}

	// 智能扫描（在 api 组之外，避免 :name 通配符冲突）
	authenticated.POST("/v1/monitors/preview", s.previewScan)
	authenticated.POST("/v1/monitors/smart-create", s.smartCreate)

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

	// 推送服务供应商元数据（供前端展示字段标签和校验）
	authenticated.GET("/settings/notification-providers", s.listNotificationProviders)
	authenticated.GET("/update/status", s.getUpdateStatus)
	authenticated.POST("/update/apply", s.applyUpdate)
	authenticated.GET("/update/proxy", s.getUpdateProxy)
	authenticated.PUT("/update/proxy", s.setUpdateProxy)

	// 版本信息和检查不修改本地状态，可以公开访问。
	s.engine.GET("/api/version", s.getVersion)
	s.engine.GET("/api/update/check", s.checkUpdate)

	if s.frontendFS != nil {
		assets, err := fs.Sub(s.frontendFS, "assets")
		if err == nil {
			s.engine.StaticFS("/assets", http.FS(assets))
		}
		s.engine.NoRoute(func(c *gin.Context) {
			indexHTML, err := fs.ReadFile(s.frontendFS, "index.html")
			if err != nil {
				c.String(http.StatusNotFound, "not found")
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
		})
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
