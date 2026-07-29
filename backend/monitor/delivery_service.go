package monitor

import (
	"context"
	"log"
	"sync"
	"time"
)

// DeliveryService 封装投递队列的生命周期管理
type DeliveryService struct {
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

// NewDeliveryService 创建投递服务
func NewDeliveryService() *DeliveryService {
	return &DeliveryService{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start 启动投递工作者（每 30 秒检查一次）
func (s *DeliveryService) Start() {
	log.Println("[DeliveryService] 启动投递队列工作者")
	go func() {
		ReconcileEventDeliveryStatuses()
		// 在后台立即执行一次，避免历史积压阻塞 Web 服务启动。
		DeliveryWorker()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		defer close(s.doneCh)
		for {
			select {
			case <-s.stopCh:
				log.Println("[DeliveryService] 投递工作者已停止")
				return
			case <-ticker.C:
				DeliveryWorker()
			}
		}
	}()
}

// Stop 安全停止投递服务
func (s *DeliveryService) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.stopCh) })
	select {
	case <-s.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
