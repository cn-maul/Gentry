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
	Updates         []ExtractResult `json:"updates,omitempty"`
	IsFirstBaseline bool            `json:"is_first_baseline"`
}

func newMonitor(site *database.Site, fetcherOpts ...fetcher.Option) *Monitor {
	f := fetcher.New(fetcherOpts...)

	// 从 database.Site 构建选择器信息
	selectors := SiteSelectorsFromSite(site)

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
			CheckInterval:  durationSeconds(site.GetCheckInterval()),
			NextCheck:      time.Now().Add(site.GetCheckInterval()),
			BaselineStatus: site.BaselineStatus,
		},
	}
	m.checkGate <- struct{}{}

	return m
}

// Run 运行监控循环。调用方必须先通过 newMonitor/RegisterMonitor 注册实例。
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
	logCheckResult(m, outcome, err, duration, isFirst)
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
	outcome := CheckOutcome{}

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
	updateMonitorStatus(m, len(updates), checkErr, time.Since(startTime))
	if checkErr == nil {
		// 推送所有待推送记录：既包含本轮新发现，也包含历史遗留的推送失败记录。
		// 此前只在「发现新内容」时推送一次，一旦发送失败/被跳过，该记录已进入历史，
		// 下次检查不再算新内容，导致「待推送」永久挂起。改为每次检查统一补推，形成重试闭环。
		pending := m.loadPendingNotifications()
		if len(pending) > 0 {
			items := make([]ExtractResult, 0, len(pending))
			recordIDs := make([]uint, 0, len(pending))
			for _, p := range pending {
				items = append(items, p.Item)
				recordIDs = append(recordIDs, p.ID)
			}
			m.sendCombinedNotification(items, recordIDs)
		}
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

// updateMonitorStatus 统一更新监控器状态
// resultCount 表示检测到的新内容/事件数量
func updateMonitorStatus(m *Monitor, resultCount int, err error, duration time.Duration) {
	site := m.siteSnapshot()
	m.updateStatus(func(s *MonitorStatus) {
		s.LastCheck = time.Now()
		s.LastDuration = duration
		s.NextCheck = time.Now().Add(site.GetCheckInterval())

		if err != nil {
			s.LastError = err.Error()
		} else {
			s.LastError = ""
			if resultCount > 0 {
				s.LastUpdate = time.Now()
				s.UpdatesCount += resultCount
			}
		}
	})

	// 同步更新数据库中的 last_check_at
	if err := database.GetDB().Model(&database.Site{}).Where("id = ?", site.ID).Update("LastCheckAt", time.Now()).Error; err != nil {
		log.Printf("[%s] 更新 LastCheckAt 失败: %v", site.Name, err)
	}
}

// logCheckResult 统一记录检查结果日志
// outcome 包含检查结果的完整信息
func logCheckResult(m *Monitor, outcome CheckOutcome, err error, duration time.Duration, isFirst bool) {
	prefix := "检查"
	if isFirst {
		prefix = "首次检查"
	}

	name := m.siteName()
	if err != nil {
		log.Printf("[%s] %s失败 (耗时: %v): %v", name, prefix, duration, err)
		return
	}

	if len(outcome.Updates) > 0 {
		log.Printf("[%s] %s发现 %d 条更新 (耗时: %v)", name, prefix, len(outcome.Updates), duration)
		for _, item := range outcome.Updates {
			log.Printf(" - %s", item["title"])
		}
	} else {
		log.Printf("[%s] %s未发现新内容 (耗时: %v)", name, prefix, duration)
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
	isFirstBaseline := len(last) == 0
	if isFirstBaseline {
		newItems = nil
	}

	// saveResults 保存所有当前结果到数据库（含 title+url 去重），
	// 新条目会被记录为新 UpdateRecord，已存在的跳过。
	// 首次基线保存的条目直接标记为已推送，避免被计入"待推送"统计（它们不会也不应推送）。
	if err := m.saveResults(current, isFirstBaseline); err != nil {
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

// pendingNotification 待推送记录及其关联的 update_record ID，
// 推送成功后按 ID 直接标记 notified，避免依赖 title/url 匹配。
type pendingNotification struct {
	ID   uint
	Item ExtractResult
}

// loadPendingNotifications 返回该站点所有尚未推送成功的更新记录（notified=false），
// 用于在每次检查后统一补推。这样即使某次推送失败（网络异常、关键词未命中、
// 推送开关临时关闭等），记录也会在后续检查中持续重试，直到推送成功。
func (m *Monitor) loadPendingNotifications() []pendingNotification {
	var records []database.UpdateRecord
	if err := database.GetDB().
		Where("site_id = ? AND notified = ?", m.site.ID, false).
		Order("created_at asc, id asc").
		Find(&records).Error; err != nil {
		log.Printf("[%s] 加载待推送记录失败: %v", m.site.Name, err)
		return nil
	}
	if len(records) == 0 {
		return nil
	}
	items := make([]pendingNotification, 0, len(records))
	for _, r := range records {
		// 优先用保存的完整 JSON 还原原始字段，保证推送内容与抓取时一致
		item := ExtractResult{}
		if r.Content != "" {
			if err := json.Unmarshal([]byte(r.Content), &item); err == nil && len(item) > 0 {
				items = append(items, pendingNotification{ID: r.ID, Item: item})
				continue
			}
		}
		// 兜底：仅用 title/url 构造（老记录可能没有 Content）
		if r.Title != "" {
			item["title"] = r.Title
		}
		if r.URL != "" {
			item["url"] = r.URL
		}
		if len(item) > 0 {
			items = append(items, pendingNotification{ID: r.ID, Item: item})
		}
	}
	return items
}

func (m *Monitor) saveResults(results []ExtractResult, markNotified bool) error {
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
			existingSet[extractKey(ExtractResult{"title": k.Title, "url": k.URL})] = struct{}{}
		}
	}

	var firstErr error
	for _, item := range results {
		title := toString(item["title"])
		urlStr := toString(item["url"])
		key := extractKey(item)
		if key == "" {
			continue
		}
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
			// 首次基线保存的条目直接标记为已推送；后续新条目保持未推送等待通知
			Notified: markNotified,
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

// notifyTemplatePlaceholders 推送模板支持的占位符。
const (
	placeholderSiteName = "{site_name}" // 站点名称
	placeholderCount    = "{count}"     // 更新条数
	placeholderItems    = "{items}"     // 编号条目列表
	placeholderTitles   = "{titles}"    // 标题以「、」连接
)

// 程序内置的默认推送模板，未在「推送通知」页面自定义时使用。
const (
	defaultNotifyTitleTemplate   = "{site_name} 有 {count} 条更新"
	defaultNotifyContentTemplate = "最新更新内容：\n{items}"
)

// renderNotifyTemplate 渲染推送模板：替换占位符；模板为空时返回零值表示使用默认格式。
// 自定义模板渲染后为空（例如模板只含未匹配的占位符）同样视为未配置，回退默认格式。
func renderNotifyTemplate(tmpl string, siteName string, items []ExtractResult) string {
	if strings.TrimSpace(tmpl) == "" {
		return ""
	}
	titleList := make([]string, 0, len(items))
	var list strings.Builder
	for i, item := range items {
		if t := toString(item["title"]); t != "" {
			titleList = append(titleList, t)
		}
		fmt.Fprintf(&list, "%d. %s\n   %s\n", i+1, toString(item["title"]), toString(item["url"]))
	}
	out := tmpl
	out = strings.ReplaceAll(out, placeholderSiteName, siteName)
	out = strings.ReplaceAll(out, placeholderCount, fmt.Sprintf("%d", len(items)))
	out = strings.ReplaceAll(out, placeholderItems, strings.TrimRight(list.String(), "\n"))
	out = strings.ReplaceAll(out, placeholderTitles, strings.Join(titleList, "、"))
	if strings.TrimSpace(out) == "" {
		return ""
	}
	return out
}

// buildNotifyContent 生成推送标题与内容。模板为全局配置：从「推送通知」页保存的
// 模板列表中取选中的模板（push_templates + push_template_active），
// 未选中任何自定义模板时使用内置默认模板。
func buildNotifyContent(siteName string, items []ExtractResult) (string, string) {
	var titleTemplate, contentTemplate string
	activeName, _ := database.GetSetting("push_template_active")
	if activeName != "" {
		for _, t := range database.GetPushTemplates() {
			if t.Name == activeName {
				titleTemplate = t.TitleTemplate
				contentTemplate = t.ContentTemplate
				break
			}
		}
	}

	title := renderNotifyTemplate(titleTemplate, siteName, items)
	if title == "" {
		title = renderNotifyTemplate(defaultNotifyTitleTemplate, siteName, items)
	}

	content := renderNotifyTemplate(contentTemplate, siteName, items)
	if content == "" {
		content = renderNotifyTemplate(defaultNotifyContentTemplate, siteName, items)
	}
	// 与旧默认格式保持一致：去掉因 {items} 为空等原因残留的尾部换行
	content = strings.TrimRight(content, "\n")
	return title, content
}

func (m *Monitor) sendCombinedNotification(items []ExtractResult, recordIDs []uint) {
	site := m.siteSnapshot()

	// 收集条目标题，供推送记录时间轴展示
	titles := make([]string, 0, len(items))
	for _, item := range items {
		if t := toString(item["title"]); t != "" {
			titles = append(titles, t)
		}
	}

	if !notify.IsEnabled() {
		log.Printf("[%s] 推送已关闭，跳过 %d 条通知", site.Name, len(items))
		m.savePushLog(site, "skipped", "推送开关未开启", nil, titles, "", recordIDs)
		return
	}

	// 如果启用了关键词过滤，只推送命中关键词的更新
	if site.NotifyFilter == "keyword" && site.NotifyKeywords != "" {
		matched := filterByKeywords(items, site.NotifyKeywords)
		if len(matched) == 0 {
			log.Printf("[%s] 关键词过滤后无匹配项，跳过推送", site.Name)
			m.savePushLog(site, "skipped", "关键词过滤后无匹配项", nil, titles, "", recordIDs)
			return
		}
		items = matched
	}

	// 确定要推送的账户
	accountIDs := site.GetNotifyAccountIDs()
	if len(accountIDs) == 0 {
		log.Printf("[%s] 未配置推送账户，跳过推送", site.Name)
		m.savePushLog(site, "skipped", "未配置推送账户", nil, titles, "", recordIDs)
		return
	}

	title, content := buildNotifyContent(site.Name, items)

	var sentCount int
	var accountNames []string
	var failedDetail []string
	for _, accID := range accountIDs {
		var account database.NotificationAccount
		if err := database.GetDB().First(&account, accID).Error; err != nil {
			msg := fmt.Sprintf("推送账户 #%d 不存在", accID)
			log.Printf("[%s] %s，跳过", site.Name, msg)
			failedDetail = append(failedDetail, msg)
			continue
		}
		accountNames = append(accountNames, account.Name)
		if err := notify.SendToAccount(&account, title, content); err != nil {
			log.Printf("[%s] 推送账户「%s」(%s) 发送失败: %v", site.Name, account.Name, account.Service, err)
			failedDetail = append(failedDetail, fmt.Sprintf("「%s」(%s): %v", account.Name, account.Service, err))
			continue
		}
		sentCount++
	}

	// 全部失败时不标记
	if sentCount == 0 {
		log.Printf("[%s] 所有推送账户均发送失败", site.Name)
		m.savePushLog(site, "failed", "", accountNames, titles, strings.Join(failedDetail, "；"), recordIDs)
		return
	}

	// 部分失败时仅记录，不标记 notified，以便用户在 UI 中看到未推送状态；
	// 未成功的记录会在下一次检查时继续补推
	if sentCount < len(accountIDs) {
		log.Printf("[%s] 部分推送账户失败 (%d/%d 成功): %s",
			site.Name, sentCount, len(accountIDs), strings.Join(failedDetail, ", "))
		m.savePushLog(site, "partial",
			fmt.Sprintf("%d/%d 个账户推送成功", sentCount, len(accountIDs)),
			accountNames, titles, strings.Join(failedDetail, "；"), recordIDs)
		return
	}

	// 全部成功才标记已通知，避免部分账户失败时丢失推送
	now := time.Now()
	if err := database.GetDB().Model(&database.UpdateRecord{}).
		Where("id IN ? AND notified = ?", recordIDs, false).
		Updates(map[string]interface{}{
			"notified":    true,
			"notified_at": now,
		}).Error; err != nil {
		log.Printf("[%s] 标记通知记录失败: %v", site.Name, err)
	}
	log.Printf("[%s] 推送成功至 %d 个账户，已标记 %d 条记录", site.Name, sentCount, len(items))
	m.savePushLog(site, "success", "", accountNames, titles, fmt.Sprintf("推送至 %d 个账户", sentCount), recordIDs)
}

// savePushLog 写入一条推送记录（时间轴展示用）。记录失败不影响主流程，仅打日志。
func (m *Monitor) savePushLog(site database.Site, status, reason string, accountNames, titles []string, detail string, recordIDs []uint) {
	namesJSON, _ := json.Marshal(accountNames)
	if namesJSON == nil {
		namesJSON = []byte("[]")
	}
	titlesJSON, _ := json.Marshal(titles)
	if titlesJSON == nil {
		titlesJSON = []byte("[]")
	}
	idsJSON, _ := json.Marshal(recordIDs)
	if idsJSON == nil {
		idsJSON = []byte("[]")
	}
	entry := &database.PushLog{
		SiteID:       site.ID,
		SiteName:     site.Name,
		Status:       status,
		Reason:       reason,
		AccountNames: string(namesJSON),
		ItemCount:    len(titles),
		Titles:       string(titlesJSON),
		Detail:       detail,
		RecordIDs:    string(idsJSON),
	}
	if err := database.GetDB().Create(entry).Error; err != nil {
		log.Printf("[%s] 写入推送记录失败: %v", site.Name, err)
	}
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
