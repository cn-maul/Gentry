package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/fetcher"
	"github.com/cn-maul/Gentry/notify"
)

type Monitor struct {
	site        *database.Site
	siteLock    sync.RWMutex
	extractor   *Extractor
	fetcher     *fetcher.Fetcher
	stopCh      chan struct{}
	stopOnce    sync.Once
	checkGate   chan struct{}
	cancelLock  sync.Mutex
	checkCancel context.CancelFunc
	runLock     sync.Mutex
	runStarted  bool
	runDone     chan struct{}
	status      MonitorStatus
	statusLock  sync.RWMutex
}

type CheckOutcome struct {
	StrategyType    string          `json:"strategy_type"`
	Events          []ChangeEvent   `json:"events,omitempty"`
	Updates         []ExtractResult `json:"updates,omitempty"`
	IsFirstBaseline bool            `json:"is_first_baseline"`
}

func newMonitor(site *database.Site, fetcherOpts ...fetcher.Option) *Monitor {
	f := fetcher.New(fetcherOpts...)

	// 从 database.Site 构建选择器信息
	selectors := SiteSelectors{
		Container: site.Container,
		Item:      site.Item,
		Fields:    make([]FieldConfig, len(site.Fields)),
	}
	for i, f := range site.Fields {
		selectors.Fields[i] = FieldConfig{
			Name:      f.Name,
			Selector:  f.Selector,
			Type:      f.Type,
			Attr:      f.Attr,
			Transform: f.Transform,
		}
	}

	m := &Monitor{
		site:      site,
		extractor: NewExtractor(selectors),
		fetcher:   f,
		stopCh:    make(chan struct{}),
		checkGate: make(chan struct{}, 1),
		runDone:   make(chan struct{}),
		status: MonitorStatus{
			Name:           site.Name,
			URL:            site.URL,
			Group:          site.GroupName,
			IsRunning:      true,
			CheckInterval:  site.GetCheckInterval(),
			NextCheck:      time.Now().Add(site.GetCheckInterval()),
			StrategyType:   site.StrategyType,
			BaselineStatus: site.BaselineStatus,
		},
	}
	m.checkGate <- struct{}{}

	return m
}

func NewMonitor(site *database.Site, fetcherOpts ...fetcher.Option) *Monitor {
	m := newMonitor(site, fetcherOpts...)
	RegisterMonitor(m)
	return m
}

func Start(site *database.Site) {
	m := NewMonitor(site)
	m.Run()
}

// Run 运行监控循环。调用方必须先通过 NewMonitor 注册实例。
func (m *Monitor) Run() {
	m.runLock.Lock()
	if m.runStarted {
		m.runLock.Unlock()
		return
	}
	m.runStarted = true
	m.runLock.Unlock()
	defer close(m.runDone)
	select {
	case <-m.stopCh:
		m.updateStatus(func(s *MonitorStatus) { s.IsRunning = false })
		return
	default:
	}

	site := m.siteSnapshot()
	ticker := time.NewTicker(site.GetCheckInterval())
	defer ticker.Stop()

	m.updateStatus(func(s *MonitorStatus) {
		s.IsRunning = true
		s.NextCheck = time.Now().Add(site.GetCheckInterval())
	})

	log.Printf("[%s] 监控启动，检查间隔: %v", site.Name, site.GetCheckInterval())
	performCheck(m, true) // 首次检查

	for {
		select {
		case <-m.stopCh:
			m.updateStatus(func(s *MonitorStatus) {
				s.IsRunning = false
			})
			log.Printf("[%s] 监控停止", site.Name)
			return
		case <-ticker.C:
			performCheck(m, false)
		}
	}
}

func performCheck(m *Monitor, isFirst bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[%s] 检查异常: %v", m.siteName(), r)
			m.updateStatus(func(s *MonitorStatus) {
				s.LastError = fmt.Sprintf("panic: %v", r)
			})
		}
	}()

	startTime := time.Now()
	outcome, err := m.CheckNow(context.Background())
	duration := time.Since(startTime)
	if outcome.StrategyType == "field_transition" {
		logCheckResultFromEngine(m, outcome.Events, err, duration, isFirst)
		if err == nil && len(outcome.Events) > 0 {
			log.Printf("[%s] 产生 %d 个事件，等待投递队列处理", m.siteName(), len(outcome.Events))
		}
		return
	}
	logCheckResult(m, outcome.Updates, err, duration, isFirst)
}

// CheckNow 串行执行一次检查；定时检查和手动检查必须复用此入口。
func (m *Monitor) CheckNow(ctx context.Context) (CheckOutcome, error) {
	checkCtx, release, err := m.acquireCheck(ctx)
	if err != nil {
		return CheckOutcome{}, err
	}
	defer release()

	startTime := time.Now()
	site := m.siteSnapshot()
	strategyType := site.StrategyType
	if strategyType == "" {
		strategyType = "presence"
	}
	outcome := CheckOutcome{StrategyType: strategyType}

	if strategyType == "field_transition" {
		engine, createErr := NewEngine(&site)
		if createErr != nil {
			updateMonitorStatusFromEngine(m, nil, createErr, time.Since(startTime))
			return outcome, fmt.Errorf("创建引擎失败: %w", createErr)
		}
		events, isFirstBaseline, checkErr := engine.CheckOnce(checkCtx)
		outcome.Events = events
		outcome.IsFirstBaseline = isFirstBaseline
		updateMonitorStatusFromEngine(m, events, checkErr, time.Since(startTime))
		if isFirstBaseline && checkErr == nil {
			m.SetBaselineStatus("ready")
		}
		return outcome, checkErr
	}

	updates, checkErr := m.checkForUpdatesContext(checkCtx, site)
	outcome.Updates = updates
	if checkErr == nil && site.BaselineStatus != "ready" {
		if err := database.GetDB().Model(&database.Site{}).Where("id = ?", site.ID).Update("baseline_status", "ready").Error; err != nil {
			checkErr = fmt.Errorf("更新基线状态失败: %w", err)
		} else {
			outcome.IsFirstBaseline = true
			m.SetBaselineStatus("ready")
		}
	}
	updateMonitorStatus(m, updates, checkErr, time.Since(startTime))
	if checkErr == nil && len(updates) > 0 {
		m.sendCombinedNotification(updates)
	}
	return outcome, checkErr
}

func (m *Monitor) acquireCheck(ctx context.Context) (context.Context, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-m.checkGate:
	}
	checkCtx, cancel := context.WithCancel(ctx)
	m.cancelLock.Lock()
	m.checkCancel = cancel
	m.cancelLock.Unlock()
	release := func() {
		cancel()
		m.cancelLock.Lock()
		m.checkCancel = nil
		m.cancelLock.Unlock()
		m.checkGate <- struct{}{}
	}
	return checkCtx, release, nil
}

func updateMonitorStatus(m *Monitor, updates []ExtractResult, err error, duration time.Duration) {
	site := m.siteSnapshot()
	m.updateStatus(func(s *MonitorStatus) {
		s.LastCheck = time.Now()
		s.LastDuration = duration
		s.NextCheck = time.Now().Add(site.GetCheckInterval())

		if err != nil {
			s.LastError = err.Error()
		} else {
			s.LastError = ""
			if len(updates) > 0 {
				s.LastUpdate = time.Now()
				s.UpdatesCount += len(updates)
			}
		}
	})

	// 同步更新数据库中的 last_check_at
	if err := database.GetDB().Model(&database.Site{}).Where("id = ?", site.ID).Update("LastCheckAt", time.Now()).Error; err != nil {
		log.Printf("[%s] 更新 LastCheckAt 失败: %v", site.Name, err)
	}
}

func updateMonitorStatusFromEngine(m *Monitor, events []ChangeEvent, err error, duration time.Duration) {
	site := m.siteSnapshot()
	m.updateStatus(func(s *MonitorStatus) {
		s.LastCheck = time.Now()
		s.LastDuration = duration
		s.NextCheck = time.Now().Add(site.GetCheckInterval())

		if err != nil {
			s.LastError = err.Error()
		} else {
			s.LastError = ""
			if len(events) > 0 {
				s.LastUpdate = time.Now()
				s.UpdatesCount += len(events)
			}
		}
	})

	if err := database.GetDB().Model(&database.Site{}).Where("id = ?", site.ID).Update("LastCheckAt", time.Now()).Error; err != nil {
		log.Printf("[%s] 更新 LastCheckAt 失败: %v", site.Name, err)
	}
}

func logCheckResult(m *Monitor, updates []ExtractResult, err error, duration time.Duration, isFirst bool) {
	prefix := "检查"
	if isFirst {
		prefix = "首次检查"
	}

	name := m.siteName()
	if err != nil {
		log.Printf("[%s] %s失败 (耗时: %v): %v", name, prefix, duration, err)
		return
	}

	if len(updates) > 0 {
		log.Printf("[%s] %s发现 %d 条更新 (耗时: %v)", name, prefix, len(updates), duration)
		for _, item := range updates {
			log.Printf(" - %s", item["title"])
		}
	} else {
		log.Printf("[%s] %s未发现新内容 (耗时: %v)", name, prefix, duration)
	}
}

func logCheckResultFromEngine(m *Monitor, events []ChangeEvent, err error, duration time.Duration, isFirst bool) {
	prefix := "检查"
	if isFirst {
		prefix = "首次检查"
	}

	name := m.siteName()
	if err != nil {
		log.Printf("[%s] %s失败 (耗时: %v): %v", name, prefix, duration, err)
		return
	}

	if len(events) > 0 {
		log.Printf("[%s] %s发现 %d 个事件 (耗时: %v)", name, prefix, len(events), duration)
		for _, event := range events {
			log.Printf(" - [%s] %s: %s", event.EventType, event.Title, event.NewValue)
		}
	} else {
		log.Printf("[%s] %s未发现变化 (耗时: %v)", name, prefix, duration)
	}
}

func (m *Monitor) CheckForUpdates() ([]ExtractResult, error) {
	return m.checkForUpdatesContext(context.Background(), m.siteSnapshot())
}

func (m *Monitor) checkForUpdatesContext(ctx context.Context, site database.Site) ([]ExtractResult, error) {
	current, err := extractSiteResults(ctx, &site, m.fetcher, m.extractor)
	if err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}
	if err := ResolveExtractedURLs(site.URL, current); err != nil {
		return nil, fmt.Errorf("resolve extracted URLs failed: %w", err)
	}

	last, err := m.loadLastResults()
	if err != nil {
		return nil, fmt.Errorf("load history failed: %w", err)
	}

	newItems := compareResults(last, current)
	// 第一次成功抓取只建立基线，不把页面现有内容当作新增内容通知。
	if len(last) == 0 {
		newItems = nil
	}

	// saveResults 保存所有当前结果到数据库（含 title+url 去重），
	// 新条目会被记录为新 UpdateRecord，已存在的跳过
	if err := m.saveResults(current); err != nil {
		return nil, fmt.Errorf("save failed: %w", err)
	}

	return newItems, nil
}

// ResolveExtractedURLs 将提取结果中的相对链接转换为监控源站的绝对链接。
func ResolveExtractedURLs(baseURL string, results []ExtractResult) error {
	base, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	for _, item := range results {
		href := toString(item["url"])
		if href == "" {
			continue
		}
		ref, err := url.Parse(href)
		if err != nil {
			return err
		}
		if !ref.IsAbs() {
			item["url"] = base.ResolveReference(ref).String()
		}
	}
	return nil
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// Stop 安全停止监控器（可多次调用，不会 panic）
func (m *Monitor) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.cancelLock.Lock()
	if m.checkCancel != nil {
		m.checkCancel()
	}
	m.cancelLock.Unlock()
	m.updateStatus(func(s *MonitorStatus) { s.IsRunning = false })
}

// StopAndWait 停止监控循环，并等待已经开始的循环退出。
func (m *Monitor) StopAndWait(ctx context.Context) error {
	m.Stop()
	if ctx == nil {
		ctx = context.Background()
	}
	m.runLock.Lock()
	started := m.runStarted
	m.runLock.Unlock()
	if !started {
		return nil
	}
	select {
	case <-m.runDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// UpdateSiteNotifyAccounts 更新运行中监控器的推送账户（无需重启）
func (m *Monitor) UpdateSiteNotifyAccounts(ids string) {
	m.siteLock.Lock()
	defer m.siteLock.Unlock()
	m.site.NotifyAccountIDs = ids
}

// SetBaselineStatus 同步更新数据库和内存中的基线状态
func (m *Monitor) SetBaselineStatus(status string) {
	m.siteLock.Lock()
	m.site.BaselineStatus = status
	m.siteLock.Unlock()
	m.updateStatus(func(s *MonitorStatus) {
		s.BaselineStatus = status
	})
}

// ResetBaseline 与检查互斥地重置数据库和内存基线状态。
func (m *Monitor) ResetBaseline(ctx context.Context) error {
	_, release, err := m.acquireCheck(ctx)
	if err != nil {
		return err
	}
	defer release()
	site := m.siteSnapshot()
	newVersion, err := database.ResetMonitorBaseline(site.ID)
	if err != nil {
		return err
	}
	m.siteLock.Lock()
	m.site.ConfigVersion = newVersion
	m.site.BaselineStatus = "needs_baseline"
	m.siteLock.Unlock()
	m.updateStatus(func(s *MonitorStatus) { s.BaselineStatus = "needs_baseline" })
	return nil
}

func (m *Monitor) siteSnapshot() database.Site {
	m.siteLock.RLock()
	defer m.siteLock.RUnlock()
	copySite := *m.site
	copySite.Fields = append([]database.SiteField(nil), m.site.Fields...)
	return copySite
}

func (m *Monitor) siteName() string {
	m.siteLock.RLock()
	defer m.siteLock.RUnlock()
	return m.site.Name
}

func (m *Monitor) loadLastResults() ([]ExtractResult, error) {
	// 查询所有 distinct (title, url) 用于去重，比加载全量 Content 更高效
	type keyPair struct {
		Title string
		URL   string
	}
	var keys []keyPair
	if err := database.GetDB().Model(&database.UpdateRecord{}).
		Select("DISTINCT title, url").
		Where("site_id = ?", m.site.ID).
		Find(&keys).Error; err != nil {
		log.Printf("[%s] 加载历史结果失败: %v", m.site.Name, err)
		return nil, fmt.Errorf("load history failed: %w", err)
	}

	if len(keys) == 0 {
		return nil, nil
	}

	var results []ExtractResult
	for _, k := range keys {
		if k.Title != "" || k.URL != "" {
			results = append(results, ExtractResult{"title": k.Title, "url": k.URL})
		}
	}
	return results, nil
}

func (m *Monitor) saveResults(results []ExtractResult) error {
	if len(results) == 0 {
		return nil
	}
	// 同一次抓取的记录使用相同时间戳。这样历史列表可以先按抓取批次倒序，
	// 再按 ID 正序保留网页本身从上到下的顺序（通常是新内容在前）。
	batchCreatedAt := database.Now()

	// 一次性加载已有的 (title, url) 对，避免 N+1 查询
	type keyPair struct {
		Title string
		URL   string
	}
	var existing []keyPair
	if err := database.GetDB().Model(&database.UpdateRecord{}).
		Select("DISTINCT title, url").
		Where("site_id = ?", m.site.ID).
		Find(&existing).Error; err != nil {
		log.Printf("[%s] 加载已有记录失败: %v", m.site.Name, err)
		return fmt.Errorf("load existing records failed: %w", err)
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, k := range existing {
		if k.Title != "" || k.URL != "" {
			existingSet[k.Title+"|"+k.URL] = struct{}{}
		}
	}

	var firstErr error
	for _, item := range results {
		title := toString(item["title"])
		urlStr := toString(item["url"])
		key := title + "|" + urlStr
		if _, exists := existingSet[key]; exists {
			continue
		}

		data, marshalErr := json.Marshal(item)
		if marshalErr != nil {
			log.Printf("[%s] 序列化提取结果失败: %v", m.site.Name, marshalErr)
			if firstErr == nil {
				firstErr = marshalErr
			}
			continue
		}
		record := &database.UpdateRecord{
			CreatedAt: batchCreatedAt,
			SiteID:    m.site.ID,
			Title:     title,
			URL:       urlStr,
			Content:   string(data),
		}
		if err := database.GetDB().Create(record).Error; err != nil {
			log.Printf("[%s] 创建更新记录失败: %v", m.site.Name, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func compareResults(last, current []ExtractResult) []ExtractResult {
	lastKeys := make(map[string]struct{})
	for _, item := range last {
		if key := extractKey(item); key != "" {
			lastKeys[key] = struct{}{}
		}
	}

	var newItems []ExtractResult
	for _, item := range current {
		if key := extractKey(item); key != "" {
			if _, exists := lastKeys[key]; !exists {
				newItems = append(newItems, item)
			}
		}
	}
	return newItems
}

// matchKeywords 检查更新项的标题或URL是否命中任一关键词（大小写不敏感）
func matchKeywords(item ExtractResult, keywordList []string) bool {
	if len(keywordList) == 0 {
		return true
	}
	title, _ := item["title"].(string)
	urlStr, _ := item["url"].(string)
	text := strings.ToLower(title + " " + urlStr)
	if text == "" {
		return false
	}
	for _, kw := range keywordList {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// filterByKeywords 根据关键词过滤更新项，仅返回命中任一关键词的项
func filterByKeywords(items []ExtractResult, keywords string) []ExtractResult {
	if keywords == "" {
		return items
	}
	kwList := strings.Split(keywords, ",")
	var matched []ExtractResult
	for _, item := range items {
		if matchKeywords(item, kwList) {
			matched = append(matched, item)
		}
	}
	return matched
}

func buildNotifyContent(siteName string, items []ExtractResult) (string, string) {
	// 推送给前端但前端不需要 content，保持原有格式
	title := fmt.Sprintf("%s 有 %d 条更新", siteName, len(items))
	var content strings.Builder
	content.WriteString("最新更新内容：\n")
	for i, item := range items {
		fmt.Fprintf(&content, "%d. %s\n   %s\n", i+1, item["title"], item["url"])
	}
	return title, content.String()
}

func (m *Monitor) sendCombinedNotification(items []ExtractResult) {
	site := m.siteSnapshot()
	if !notify.IsEnabled() {
		log.Printf("[%s] 推送已关闭，跳过 %d 条通知", site.Name, len(items))
		return
	}

	// 如果启用了关键词过滤，只推送命中关键词的更新
	if site.NotifyFilter == "keyword" && site.NotifyKeywords != "" {
		matched := filterByKeywords(items, site.NotifyKeywords)
		if len(matched) == 0 {
			log.Printf("[%s] 关键词过滤后无匹配项，跳过推送", site.Name)
			return
		}
		items = matched
	}

	// 确定要推送的账户
	accountIDs := site.GetNotifyAccountIDs()
	if len(accountIDs) == 0 {
		log.Printf("[%s] 未配置推送账户，跳过推送", site.Name)
		return
	}

	title, content := buildNotifyContent(site.Name, items)

	var sentCount int
	var failedAccounts []string
	for _, accID := range accountIDs {
		var account database.NotificationAccount
		if err := database.GetDB().First(&account, accID).Error; err != nil {
			log.Printf("[%s] 推送账户 #%d 不存在，跳过", site.Name, accID)
			failedAccounts = append(failedAccounts, fmt.Sprintf("#%d", accID))
			continue
		}
		if err := notify.SendToAccount(&account, title, content); err != nil {
			log.Printf("[%s] 推送账户「%s」(%s) 发送失败: %v", site.Name, account.Name, account.Service, err)
			failedAccounts = append(failedAccounts, account.Name)
			continue
		}
		sentCount++
	}

	// 全部失败时不标记
	if sentCount == 0 {
		log.Printf("[%s] 所有推送账户均发送失败", site.Name)
		return
	}

	// 部分失败时仅记录，不标记 notified，以便用户在 UI 中看到未推送状态
	if len(failedAccounts) > 0 {
		log.Printf("[%s] 部分推送账户失败 (%d/%d 成功): %s",
			site.Name, sentCount, len(accountIDs), strings.Join(failedAccounts, ", "))
	}

	// 全部成功才标记已通知，避免部分账户失败时丢失推送
	if sentCount < len(accountIDs) {
		log.Printf("[%s] 存在失败账户，不标记 notified，等待下次重试", site.Name)
		return
	}

	// 推送成功后标记数据库记录为已通知
	now := time.Now()
	for _, item := range items {
		itemTitle := toString(item["title"])
		urlStr := toString(item["url"])
		if err := database.GetDB().Model(&database.UpdateRecord{}).
			Where("site_id = ? AND title = ? AND url = ? AND notified = ?", site.ID, itemTitle, urlStr, false).
			Updates(map[string]interface{}{
				"notified":    true,
				"notified_at": now,
			}).Error; err != nil {
			log.Printf("[%s] 标记通知记录失败 (title=%s, url=%s): %v", site.Name, itemTitle, urlStr, err)
		}
	}
	log.Printf("[%s] 推送成功至 %d 个账户，已标记 %d 条记录", site.Name, sentCount, len(items))
}

func extractKey(item ExtractResult) string {
	title, _ := item["title"].(string)
	urlStr, _ := item["url"].(string)
	switch {
	case title != "" && urlStr != "":
		return title + "|" + urlStr
	case title != "":
		return title
	case urlStr != "":
		return urlStr
	default:
		data, err := json.Marshal(item)
		if err == nil {
			return fmt.Sprintf("%x", data)
		}
		return ""
	}
}
