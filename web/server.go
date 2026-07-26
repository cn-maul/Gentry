package web

import (
	"context"
	"net/http"
)

// Run 启动 HTTP 服务并阻塞。正常关闭时返回 http.ErrServerClosed。
func (s *WebServer) Run(addr string) error {
	s.setupRoutes()
	s.httpServer = &http.Server{Addr: addr, Handler: s.engine}
	return s.httpServer.ListenAndServe()
}

// Shutdown 优雅关闭 HTTP 服务，等待在途请求完成或 ctx 超时。
func (s *WebServer) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
