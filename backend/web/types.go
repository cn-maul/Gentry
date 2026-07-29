package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebServer 核心服务器结构
type WebServer struct {
	engine     *gin.Engine
	version    string
	httpServer *http.Server
}

// APIResponse 标准API响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewWebServer 创建WebServer实例（纯 API 模式）
func NewWebServer(version string) *WebServer {
	return &WebServer{engine: gin.Default(), version: version}
}
