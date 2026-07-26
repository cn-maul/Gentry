package web

import (
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebServer 核心服务器结构
type WebServer struct {
	engine     *gin.Engine
	frontendFS fs.FS
	version    string
	httpServer *http.Server
}

// APIResponse 标准API响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewWebServer 创建WebServer实例
func NewWebServer(frontendFS fs.FS, version string) *WebServer {
	return &WebServer{engine: gin.Default(), frontendFS: frontendFS, version: version}
}