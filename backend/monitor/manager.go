package monitor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cn-maul/Gentry/database"
)

var (
	monitors     = make(map[string]*Monitor)
	monitorsLock sync.RWMutex
)

type MonitorStatus struct {
	Name           string        `json:"name"`
	URL            string        `json:"url"`
	Group          string        `json:"group"`
	IsRunning      bool          `json:"is_running"`
	LastCheck      time.Time     `json:"last_check"`
	LastDuration   time.Duration `json:"last_duration"`
	LastError      string        `json:"last_error,omitempty"`
	LastUpdate     time.Time     `json:"last_update,omitempty"`
	UpdatesCount   int           `json:"updates_count"`
	NextCheck      time.Time     `json:"next_check"`
	CheckInterval  time.Duration `json:"check_interval"`
	StrategyType   string        `json:"strategy_type,omitempty"`
	BaselineStatus string        `json:"baseline_status,omitempty"`
	LastEventAt    *time.Time    `json:"last_event_at,omitempty"`
	SnapshotCount  int           `json:"snapshot_count,omitempty"`
}

// AtomicReplaceMonitor 原子式替换监控器：停止旧实例 → 注销旧名 → 创建新实例 → 启动/停止
// 整个流程在 monitorsLock 保护下完成，消除状态分裂窗口
func AtomicReplaceMonitor(site *database.Site, oldName string) (*Monitor, error) {
	monitorsLock.Lock()
	defer monitorsLock.Unlock()

	// 如果名字不同，停止并移除旧的
	if oldName != "" && oldName != site.Name {
		if old := monitors[oldName]; old != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			err := old.StopAndWait(ctx)
			cancel()
			if err != nil {
				return nil, fmt.Errorf("等待旧监控器停止失败: %w", err)
			}
			delete(monitors, oldName)
		}
	}

	// 如果同名的已存在（重名或重载），停止并移除
	if existing := monitors[site.Name]; existing != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := existing.StopAndWait(ctx)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("等待同名监控器停止失败: %w", err)
		}
		delete(monitors, site.Name)
	}

	m := newMonitor(site)
	if site.IsActive {
		if err := SetSiteActive(site.Name, true); err != nil {
			return nil, fmt.Errorf("更新站点活跃状态失败: %w", err)
		}
		registerMonitorLocked(m)
		go m.Run()
	} else {
		m.updateStatus(func(s *MonitorStatus) {
			s.IsRunning = false
		})
		registerMonitorLocked(m)
	}
	return m, nil
}

func RegisterMonitor(m *Monitor) {
	monitorsLock.Lock()
	defer monitorsLock.Unlock()
	registerMonitorLocked(m)
}

func NewMonitorUnlocked(site *database.Site) *Monitor {
	m := newMonitor(site)
	registerMonitorLocked(m)
	return m
}

// NewDetachedMonitor 创建不注册、不启动的监控器，供一次性手动检查使用。
func NewDetachedMonitor(site *database.Site) *Monitor {
	return newMonitor(site)
}

func registerMonitorLocked(m *Monitor) {
	monitors[m.site.Name] = m
	log.Printf("[%s] 监控器已注册", m.site.Name)
}

func UnregisterMonitor(name string) {
	monitorsLock.Lock()
	defer monitorsLock.Unlock()
	if _, exists := monitors[name]; exists {
		delete(monitors, name)
		log.Printf("[%s] 监控器已注销", name)
	}
}

func ReplaceStoppedSite(site *database.Site, oldName string) {
	monitorsLock.Lock()
	defer monitorsLock.Unlock()
	if oldName != "" && oldName != site.Name {
		delete(monitors, oldName)
	}
	delete(monitors, site.Name)
	m := newMonitor(site)
	m.updateStatus(func(s *MonitorStatus) {
		s.IsRunning = false
	})
	registerMonitorLocked(m)
}

func GetMonitor(name string) *Monitor {
	monitorsLock.RLock()
	defer monitorsLock.RUnlock()
	return monitors[name]
}

func GetAllMonitors() []MonitorStatus {
	monitorsLock.RLock()
	defer monitorsLock.RUnlock()
	var statusList []MonitorStatus
	for _, m := range monitors {
		statusList = append(statusList, m.GetStatus())
	}
	return statusList
}

func (m *Monitor) GetStatus() MonitorStatus {
	m.statusLock.RLock()
	defer m.statusLock.RUnlock()
	return m.status
}

func (m *Monitor) updateStatus(updater func(*MonitorStatus)) {
	m.statusLock.Lock()
	defer m.statusLock.Unlock()
	updater(&m.status)
}

func StartLoadedSite(site *database.Site) error {
	monitorsLock.Lock()
	if existing := monitors[site.Name]; existing != nil && existing.GetStatus().IsRunning {
		monitorsLock.Unlock()
		return nil
	}
	m := NewMonitorUnlocked(site)
	monitorsLock.Unlock()

	if err := SetSiteActive(site.Name, true); err != nil {
		m.Stop()
		m.updateStatus(func(s *MonitorStatus) { s.IsRunning = false })
		UnregisterMonitor(site.Name)
		return fmt.Errorf("更新站点活跃状态失败: %w", err)
	}
	go m.Run()
	return nil
}

func StartSite(name string) error {
	var site database.Site
	if err := database.GetDB().Preload("Fields").Where("name = ?", name).First(&site).Error; err != nil {
		return fmt.Errorf("加载站点「%s」失败: %w", name, err)
	}
	return StartLoadedSite(&site)
}

func StopSite(name string) error {
	monitorsLock.RLock()
	m := monitors[name]
	monitorsLock.RUnlock()
	if m == nil {
		return fmt.Errorf("监控器「%s」不存在", name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err := m.StopAndWait(ctx)
	cancel()
	if err != nil {
		return fmt.Errorf("等待监控器停止失败: %w", err)
	}
	if err := SetSiteActive(name, false); err != nil {
		return fmt.Errorf("更新站点活跃状态失败: %w", err)
	}
	return nil
}

func RestartSite(site *database.Site) error {
	monitorsLock.Lock()
	if existing := monitors[site.Name]; existing != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := existing.StopAndWait(ctx)
		cancel()
		if err != nil {
			monitorsLock.Unlock()
			return fmt.Errorf("等待监控器停止失败: %w", err)
		}
		delete(monitors, site.Name)
	}
	m := NewMonitorUnlocked(site)
	monitorsLock.Unlock()
	go m.Run()
	return nil
}

func RegisterStoppedSite(site *database.Site) {
	ReplaceStoppedSite(site, "")
}

func SetSiteActive(name string, active bool) error {
	return database.GetDB().Model(&database.Site{}).Where("name = ?", name).Update("is_active", active).Error
}

func StartAllFromDB() {
	var sites []database.Site
	if err := database.GetDB().Preload("Fields").Find(&sites).Error; err != nil {
		log.Printf("[Monitor] 从数据库加载站点失败: %v", err)
		return
	}
	activeCount := 0
	for i := range sites {
		if sites[i].IsActive {
			if err := StartLoadedSite(&sites[i]); err != nil {
				log.Printf("[%s] 启动失败: %v", sites[i].Name, err)
				continue
			}
			activeCount++
		} else {
			RegisterStoppedSite(&sites[i])
		}
	}
	log.Printf("[Monitor] 已启动 %d 个监控器（共 %d 个站点）", activeCount, len(sites))
}

func StopAll() {
	monitorsLock.RLock()
	names := make([]string, 0, len(monitors))
	for name := range monitors {
		names = append(names, name)
	}
	monitorsLock.RUnlock()
	for _, name := range names {
		monitorsLock.RLock()
		m, exists := monitors[name]
		monitorsLock.RUnlock()
		if exists {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			if err := m.StopAndWait(ctx); err != nil {
				log.Printf("[%s] 等待监控器停止失败: %v", name, err)
			}
			cancel()
			log.Printf("[%s] 监控器已停止", name)
		}
	}
	log.Println("[Monitor] 所有监控器已停止")
}

// QuiesceMonitor 停止并等待指定监控器退出，但保留 registry 条目供后续原子替换。
func QuiesceMonitor(name string, ctx context.Context) error {
	monitorsLock.RLock()
	m := monitors[name]
	monitorsLock.RUnlock()
	if m == nil {
		return nil
	}
	return m.StopAndWait(ctx)
}

func StopMonitor(name string) bool {
	monitorsLock.RLock()
	m, exists := monitors[name]
	monitorsLock.RUnlock()
	if !exists {
		return false
	}
	m.Stop()
	log.Printf("[%s] 监控器已停止", name)
	return true
}

func Exists(name string) bool {
	monitorsLock.RLock()
	defer monitorsLock.RUnlock()
	_, ok := monitors[name]
	return ok
}

func MarkRead(name string) {
	monitorsLock.RLock()
	m, exists := monitors[name]
	monitorsLock.RUnlock()
	if !exists {
		return
	}
	m.updateStatus(func(s *MonitorStatus) {
		s.UpdatesCount = 0
	})
}
