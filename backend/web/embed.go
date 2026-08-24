package web

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// 前端构建产物在发布构建时由 Makefile / Dockerfile / build.bat
// 从 frontend/dist 拷贝到 web/dist。仓库中提交了占位 index.html，
// 保证未构建前端时 go build 也能通过。
//
//go:embed all:dist
var distFS embed.FS

var spaFS, _ = fs.Sub(distFS, "dist")

const (
	spaIndexCacheControl  = "no-cache, no-store, must-revalidate"
	spaAssetsCacheControl = "public, max-age=31536000, immutable"
	spaNoRouteAPIResponse = "route not found"
)

// setupSPA 注册 SPA 静态资源服务（通过 NoRoute 兜底）：
//   - /api 前缀的未匹配路由保持 JSON 404（API 语义不变）
//   - 其余 GET/HEAD 请求命中静态文件则返回（assets/* 一年 immutable 缓存）
//   - 未命中回退 index.html（支持前端 history 路由）
func (s *WebServer) setupSPA() {
	s.engine.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/api" || strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, NewErrorResponse(404, spaNoRouteAPIResponse))
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.JSON(http.StatusMethodNotAllowed, NewErrorResponse(405, "method not allowed"))
			return
		}
		serveSPAFile(c)
	})
}

func serveSPAFile(c *gin.Context) {
	name := strings.TrimPrefix(path.Clean("/"+c.Request.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}

	cacheControl := spaIndexCacheControl
	if strings.HasPrefix(name, "assets/") {
		cacheControl = spaAssetsCacheControl
	}

	if name != "index.html" && serveEmbeddedFile(c, name, cacheControl) {
		return
	}
	// SPA 回退：未命中的路径一律返回 index.html
	serveEmbeddedFile(c, "index.html", spaIndexCacheControl)
}

func serveEmbeddedFile(c *gin.Context, name, cacheControl string) bool {
	data, err := fs.ReadFile(spaFS, name)
	if err != nil {
		return false
	}
	c.Header("Cache-Control", cacheControl)
	c.Data(http.StatusOK, spaContentType(name), data)
	return true
}

func spaContentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json", ".map":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
			return ct
		}
		return "application/octet-stream"
	}
}
