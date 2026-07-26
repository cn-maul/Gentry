package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/fetcher"
	"github.com/cn-maul/Gentry/notify"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrStaleDefinition = errors.New("monitor definition changed during check")

// Engine 监控引擎，编排一次检查的完整流程
type Engine struct {
	site      *database.Site
	extractor *Extractor
	fetcher   *fetcher.Fetcher
	detector  Detector
	rule      *DetectionRule
}

// NewEngine 创建新引擎，返回错误而不是在非法配置下默默运行
func NewEngine(site *database.Site) (*Engine, error) {
	if site == nil {
		return nil, fmt.Errorf("site is required")
	}
	normalizedSite := *site
	normalizedSite.Fields = append([]database.SiteField(nil), site.Fields...)
	if err := NormalizeAndValidateSiteDefinition(&normalizedSite); err != nil {
		return nil, fmt.Errorf("invalid monitor definition: %w", err)
	}
	site = &normalizedSite

	f := fetcher.New()
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

	rule, err := ParseDetectionRule(site.StrategyConfig)
	if err != nil {
		return nil, fmt.Errorf("parse detection rule failed: %w", err)
	}

	// 未知策略类型返回错误
	if rule.Type != "presence" && rule.Type != "field_transition" {
		return nil, fmt.Errorf("unknown strategy type: %s", rule.Type)
	}

	detector := NewDetector(rule.Type, *rule)

	return &Engine{
		site:      site,
		extractor: NewExtractor(selectors),
		fetcher:   f,
		detector:  detector,
		rule:      rule,
	}, nil
}

// CheckOnce 执行一次完整检查，返回事件和是否建立基线
func (e *Engine) CheckOnce(ctx context.Context) ([]ChangeEvent, bool, error) {
	site := e.site
	observations, err := e.observe(ctx)
	if err != nil {
		return nil, false, err
	}

	// 1. 加载快照
	snapshots, err := LoadSnapshots(site.ID, site.ConfigVersion)
	if err != nil {
		return nil, false, fmt.Errorf("load snapshots failed: %w", err)
	}

	// 2. 检测
	result := e.detector.Evaluate(snapshots, observations)

	// 3. 首次基线处理
	isFirstBaseline := len(snapshots) == 0
	if isFirstBaseline && e.rule.OnFirstBaseline == "silent" {
		result.Events = nil
	}

	// 4. 生成确定性去重键
	for i := range result.Events {
		beforeFP := ""
		afterFP := ""
		if len(result.Events[i].Before) > 0 {
			beforeFP = computeFingerprint(result.Events[i].Before)
		}
		if len(result.Events[i].After) > 0 {
			afterFP = computeFingerprint(result.Events[i].After)
		}
		if result.Events[i].URL == "" {
			result.Events[i].URL = site.URL
		}
		result.Events[i].SiteID = site.ID
		result.Events[i].DefinitionVersion = site.ConfigVersion
		result.Events[i].DedupeKey = GenerateDedupeKey(site.ID, site.ConfigVersion, result.Events[i].EventType, result.Events[i].ItemKey, beforeFP, afterFP)
	}

	// 5. 事务性持久化
	accountIDs := site.GetNotifyAccountIDs()
	if err := PersistEvaluation(site, isFirstBaseline, result, accountIDs); err != nil {
		return nil, false, fmt.Errorf("persist evaluation failed: %w", err)
	}

	return result.Events, isFirstBaseline, nil
}

func (e *Engine) observe(ctx context.Context) ([]Observation, error) {
	site := e.site
	rawResults, err := extractSiteResults(ctx, site, e.fetcher, e.extractor)
	if err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}
	if err := ResolveExtractedURLs(site.URL, rawResults); err != nil {
		return nil, fmt.Errorf("resolve URLs failed: %w", err)
	}

	observations := e.toObservations(rawResults)
	if len(observations) == 0 {
		return nil, fmt.Errorf("提取结果为空，请检查选择器")
	}

	identityCounts := make(map[string]int)
	for _, obs := range observations {
		if obs.ItemKey == "" {
			return nil, fmt.Errorf("身份字段为空，无法生成稳定标识")
		}
		identityCounts[obs.ItemKey]++
	}
	for key, count := range identityCounts {
		if count > 1 {
			return nil, fmt.Errorf("身份字段重复: %s (%d次)", key, count)
		}
	}
	return observations, nil
}

// ValidateExtraction 只读验证抓取、选择器、身份和价格解析，不写入任何状态。
func (e *Engine) ValidateExtraction(ctx context.Context) (*ExtractionValidationResult, error) {
	observations, err := e.observe(ctx)
	if err != nil {
		return nil, err
	}
	report := &ExtractionValidationResult{ExtractedItems: len(observations)}
	limit := len(observations)
	if limit > 5 {
		limit = 5
	}
	if e.rule.Type == "field_transition" {
		condition := e.rule.Conditions[0]
		for index, observation := range observations {
			value, ok := observation.Fields[condition.Field]
			if !ok || !value.Valid || value.DataType != "money" {
				return nil, fmt.Errorf("条目 %s 的价格字段 %s 无法解析", observation.ItemKey, condition.Field)
			}
			if index < limit {
				raw := value.Value
				if rawValue, exists := observation.Raw[condition.Field]; exists && rawValue != nil {
					raw = fmt.Sprint(rawValue)
				}
				report.Samples = append(report.Samples, ExtractionValidationSample{
					ItemKey: observation.ItemKey, Raw: raw,
					Normalized: formatPrice(value.Minor, value.Currency), Currency: value.Currency,
				})
			}
		}
	} else {
		for index, observation := range observations {
			if index >= limit {
				break
			}
			raw := observation.ItemKey
			if title, exists := observation.Raw["title"]; exists && title != nil && fmt.Sprint(title) != "" {
				raw = fmt.Sprint(title)
			}
			report.Samples = append(report.Samples, ExtractionValidationSample{
				ItemKey: observation.ItemKey, Raw: raw, Normalized: observation.ItemKey,
			})
		}
	}
	return report, nil
}

// PersistEvaluation 事务性持久化快照、事件和投递任务
func PersistEvaluation(site *database.Site, isFirstBaseline bool, result EvaluationResult, accountIDs []uint) error {
	if site == nil {
		return fmt.Errorf("site is required")
	}
	siteID := site.ID
	configVersion := site.ConfigVersion
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var currentVersion int
		if err := tx.Model(&database.Site{}).Select("config_version").Where("id = ?", siteID).Scan(&currentVersion).Error; err != nil {
			return fmt.Errorf("load current definition version failed: %w", err)
		}
		if currentVersion != configVersion {
			return ErrStaleDefinition
		}

		// 1. 保存快照
		if err := saveSnapshotsTx(tx, siteID, result.NextSnapshots, configVersion); err != nil {
			return fmt.Errorf("save snapshots failed: %w", err)
		}

		// 2. 保存事件并创建投递
		for _, event := range result.Events {
			beforeJSON, _ := json.Marshal(event.Before)
			afterJSON, _ := json.Marshal(event.After)

			// URL 回退由具备站点上下文的引擎统一处理。
			eventURL := event.URL
			if eventURL == "" {
				eventURL = site.URL
			}

			deliveryStatus := "pending"
			if len(accountIDs) == 0 {
				deliveryStatus = "skipped"
			}
			monitorEvent := &database.MonitorEvent{
				SiteID:            siteID,
				EventType:         event.EventType,
				ItemKey:           event.ItemKey,
				Title:             event.Title,
				URL:               eventURL,
				BeforeJSON:        string(beforeJSON),
				AfterJSON:         string(afterJSON),
				OldValue:          event.OldValue,
				NewValue:          event.NewValue,
				ChangeAmount:      event.ChangeAmount,
				ChangePercent:     event.ChangePercent,
				Currency:          event.Currency,
				DedupeKey:         event.DedupeKey,
				DefinitionVersion: configVersion,
				OccurredAt:        event.OccurredAt,
				DeliveryStatus:    deliveryStatus,
			}
			createResult := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "site_id"}, {Name: "dedupe_key"}},
				DoNothing: true,
			}).Create(monitorEvent)
			if createResult.Error != nil {
				return fmt.Errorf("create event failed: %w", createResult.Error)
			}
			if createResult.RowsAffected == 0 {
				continue
			}

			// 为每个账户创建投递任务
			for _, accountID := range accountIDs {
				delivery := &database.NotificationDelivery{
					EventID:   monitorEvent.ID,
					AccountID: accountID,
					SiteID:    siteID,
					Status:    "pending",
				}
				if err := tx.Create(delivery).Error; err != nil {
					return fmt.Errorf("create delivery failed: %w", err)
				}
			}
		}

		// 3. 更新基线状态
		if isFirstBaseline {
			updateResult := tx.Model(&database.Site{}).
				Where("id = ? AND config_version = ?", siteID, configVersion).
				Update("baseline_status", "ready")
			if updateResult.Error != nil {
				return fmt.Errorf("update baseline status failed: %w", updateResult.Error)
			}
			if updateResult.RowsAffected == 0 {
				return ErrStaleDefinition
			}
		}

		return nil
	})
}

// saveSnapshotsTx 事务内 upsert 快照
func saveSnapshotsTx(tx *gorm.DB, siteID uint, snapshots []Snapshot, configVersion int) error {
	if len(snapshots) == 0 {
		return nil
	}

	for _, s := range snapshots {
		payloadJSON, _ := json.Marshal(s.Payload)
		payloadStr := string(payloadJSON)

		firstSeen := s.FirstSeenAt
		if firstSeen.IsZero() {
			firstSeen = time.Now()
		}
		lastSeen := s.LastSeenAt
		if lastSeen.IsZero() {
			lastSeen = time.Now()
		}

		result := tx.Exec(`
			INSERT INTO monitor_snapshots (site_id, item_key, payload_json, fingerprint, definition_version, first_seen_at, last_seen_at, missing_checks, currency, price_minor, price_valid, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
			ON CONFLICT(site_id, item_key) DO UPDATE SET
				payload_json = excluded.payload_json,
				fingerprint = excluded.fingerprint,
				first_seen_at = CASE
					WHEN monitor_snapshots.definition_version <> excluded.definition_version THEN excluded.first_seen_at
					ELSE monitor_snapshots.first_seen_at
				END,
				definition_version = excluded.definition_version,
				last_seen_at = excluded.last_seen_at,
				missing_checks = excluded.missing_checks,
				currency = excluded.currency,
				price_minor = excluded.price_minor,
				price_valid = excluded.price_valid,
				updated_at = datetime('now')
		`, siteID, s.ItemKey, payloadStr, s.Fingerprint, configVersion, firstSeen, lastSeen, s.MissingChecks, s.Currency, s.PriceMinor, s.PriceValid)

		if result.Error != nil {
			return fmt.Errorf("upsert snapshot %s failed: %w", s.ItemKey, result.Error)
		}
	}
	return nil
}

// toObservations 将 ExtractResult 转换为 Observation
func (e *Engine) toObservations(results []ExtractResult) []Observation {
	dataTypes := e.parseFieldDataTypes()
	var obs []Observation

	for _, item := range results {
		fields := make(map[string]TypedValue)
		raw := make(map[string]interface{})

		for k, v := range item {
			strVal := fmt.Sprintf("%v", v)
			raw[k] = v
			dataType := "text"
			if dt, ok := dataTypes[k]; ok {
				dataType = dt
			}
			fields[k] = NormalizeField(strVal, dataType)
		}

		// 生成 item key。presence 的默认 source_url 规则需要对列表条目使用
		// 提取 URL、标题或内容指纹，否则整页条目会共享同一个 key。
		itemKey := GenerateItemKey(item, e.rule.Identity, e.site.URL)
		if e.rule.Type == "presence" && e.rule.Identity.Source == "source_url" {
			if extractedURL := strings.TrimSpace(fmt.Sprint(item["url"])); extractedURL != "" && extractedURL != "<nil>" {
				itemKey = extractedURL
			} else if title := strings.TrimSpace(fmt.Sprint(item["title"])); title != "" && title != "<nil>" {
				itemKey = title
			} else {
				itemKey = ComputeFingerprint(raw)
			}
		}

		obs = append(obs, Observation{
			ItemKey: itemKey,
			Fields:  fields,
			Raw:     raw,
			SeenAt:  time.Now(),
		})
	}
	return obs
}

// parseFieldDataTypes 解析字段数据类型配置
func (e *Engine) parseFieldDataTypes() map[string]string {
	result := make(map[string]string)

	if e.site.FieldDataTypes != "" {
		var configured map[string]string
		if err := json.Unmarshal([]byte(e.site.FieldDataTypes), &configured); err == nil {
			for k, v := range configured {
				result[k] = v
			}
			return result
		}
	}

	// 从策略配置推断
	for _, cond := range e.rule.Conditions {
		if cond.ValueType == "money" {
			result[cond.Field] = "money"
		}
	}

	return result
}

// DeliveryWorker 投递工作者：从队列中取出待投递任务并发送
func DeliveryWorker() {
	deliveries, err := PendingDeliveries(10)
	if err != nil {
		log.Printf("[DeliveryWorker] 查询失败: %v", err)
		return
	}

	for _, d := range deliveries {
		processDelivery(d)
	}
}

// ReconcileEventDeliveryStatuses 在启动时修复历史事件的聚合状态。
func ReconcileEventDeliveryStatuses() {
	var eventIDs []uint
	if err := database.GetDB().Model(&database.MonitorEvent{}).Pluck("id", &eventIDs).Error; err != nil {
		log.Printf("[DeliveryWorker] 加载待聚合事件失败: %v", err)
		return
	}
	for _, eventID := range eventIDs {
		if err := aggregateEventStatus(eventID); err != nil {
			log.Printf("[DeliveryWorker] 聚合历史事件失败 event=%d: %v", eventID, err)
		}
	}
}

func processDelivery(d database.NotificationDelivery) {
	// 原子 claim：从 pending/failed 改为 sending，设置 lease
	now := time.Now()
	leaseUntil := now.Add(2 * time.Minute)
	result := database.GetDB().Model(&database.NotificationDelivery{}).
		Where("id = ? AND status IN ?", d.ID, []string{"pending", "failed"}).
		Updates(map[string]interface{}{
			"status":      "sending",
			"lease_until": leaseUntil,
			// SQL 原子自增，避免并发 claim 时用查询期旧值覆盖丢失计数
			"attempts": gorm.Expr("attempts + 1"),
		})
	if result.Error != nil {
		log.Printf("[DeliveryWorker] claim 失败 delivery=%d: %v", d.ID, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		return // 已被其他 worker 领取
	}

	// 获取事件
	var event database.MonitorEvent
	if err := database.GetDB().First(&event, d.EventID).Error; err != nil {
		failDelivery(d.ID, "event not found: "+err.Error())
		return
	}

	// 获取站点
	var site database.Site
	if err := database.GetDB().First(&site, d.SiteID).Error; err != nil {
		failDelivery(d.ID, "site not found: "+err.Error())
		return
	}

	// 获取账户
	var account database.NotificationAccount
	if err := database.GetDB().First(&account, d.AccountID).Error; err != nil {
		failDelivery(d.ID, "account not found: "+err.Error())
		return
	}

	// 全局通知开关
	if !notify.IsEnabled() {
		if err := skipDelivery(d.ID); err != nil {
			log.Printf("[DeliveryWorker] 标记 skipped 失败 delivery=%d: %v", d.ID, err)
		}
		return
	}

	// 构建事件
	changeEvent := ChangeEvent{
		EventType:     event.EventType,
		ItemKey:       event.ItemKey,
		Title:         event.Title,
		URL:           event.URL,
		OldValue:      event.OldValue,
		NewValue:      event.NewValue,
		ChangeAmount:  event.ChangeAmount,
		ChangePercent: event.ChangePercent,
		Currency:      event.Currency,
	}

	// source_url 回退：事件 URL 为空时使用站点 URL
	eventURL := changeEvent.URL
	if eventURL == "" {
		eventURL = site.URL
		changeEvent.URL = eventURL
	}

	title, content := FormatEvent(changeEvent, site.Name)

	// 关键词过滤
	if site.NotifyFilter == "keyword" && site.NotifyKeywords != "" {
		if !matchEventKeywords(changeEvent, site.NotifyKeywords) {
			if err := skipDelivery(d.ID); err != nil {
				log.Printf("[DeliveryWorker] 标记 skipped 失败 delivery=%d: %v", d.ID, err)
			}
			return
		}
	}

	// 发送
	if err := notify.SendToAccount(&account, title, content); err != nil {
		log.Printf("[DeliveryWorker] 发送失败 delivery=%d account=%s: %v", d.ID, account.Name, err)
		failDelivery(d.ID, err.Error())
		return
	}

	// 标记成功
	if err := transitionDelivery(d.ID, "sent", map[string]interface{}{
		"sent_at":    now,
		"last_error": "",
	}); err != nil {
		log.Printf("[DeliveryWorker] 标记 sent 失败 delivery=%d: %v", d.ID, err)
	}
}

func skipDelivery(id uint) error {
	return transitionDelivery(id, "skipped", map[string]interface{}{"last_error": ""})
}

func failDelivery(id uint, errMsg string) {
	now := time.Now()
	var d database.NotificationDelivery
	if err := database.GetDB().First(&d, id).Error; err != nil {
		return
	}
	// 超过最大重试次数标记为 dead
	maxAttempts := 10
	if d.Attempts >= maxAttempts {
		if err := transitionDelivery(id, "dead", map[string]interface{}{
			"last_error": errMsg,
			"updated_at": now,
		}); err != nil {
			log.Printf("[DeliveryWorker] 标记 dead 失败 delivery=%d: %v", id, err)
		}
		return
	}
	// 指数退避：5min, 10min, 20min, 40min, 80min...
	attemptIndex := d.Attempts - 1
	if attemptIndex < 0 {
		attemptIndex = 0
	}
	backoff := time.Duration(5*(1<<uint(attemptIndex))) * time.Minute
	if backoff > 24*time.Hour {
		backoff = 24 * time.Hour
	}
	if err := transitionDelivery(id, "failed", map[string]interface{}{
		"last_error":      errMsg,
		"next_attempt_at": now.Add(backoff),
		"updated_at":      now,
	}); err != nil {
		log.Printf("[DeliveryWorker] 标记 failed 失败 delivery=%d: %v", id, err)
	}
}

// transitionDelivery 原子更新 Delivery 状态并聚合事件状态
func transitionDelivery(id uint, status string, extra map[string]interface{}) error {
	updates := map[string]interface{}{
		"status":      status,
		"lease_until": nil,
	}
	if status == "sent" || status == "skipped" || status == "dead" {
		updates["next_attempt_at"] = nil
	}
	for k, v := range extra {
		updates[k] = v
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var d database.NotificationDelivery
		if err := tx.Select("event_id").First(&d, id).Error; err != nil {
			return fmt.Errorf("load delivery failed: %w", err)
		}
		result := tx.Model(&database.NotificationDelivery{}).Where("id = ?", id).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("update delivery failed: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("delivery not found: %d", id)
		}
		return aggregateEventStatusTx(tx, d.EventID)
	})
}

// aggregateEventStatus 聚合事件的通知状态
func aggregateEventStatus(eventID uint) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		return aggregateEventStatusTx(tx, eventID)
	})
}

func aggregateEventStatusTx(tx *gorm.DB, eventID uint) error {
	type statusCount struct {
		Status string
		Count  int64
	}
	var rows []statusCount
	if err := tx.Model(&database.NotificationDelivery{}).
		Select("status, COUNT(*) AS count").
		Where("event_id = ?", eventID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("count delivery statuses failed: %w", err)
	}
	var total, sentCount, skippedCount, deadCount int64
	for _, row := range rows {
		total += row.Count
		switch row.Status {
		case "sent":
			sentCount = row.Count
		case "skipped":
			skippedCount = row.Count
		case "dead":
			deadCount = row.Count
		}
	}
	terminal := sentCount + skippedCount + deadCount

	var deliveryStatus string
	switch {
	case total == 0:
		deliveryStatus = "skipped"
	case terminal < total:
		deliveryStatus = "pending"
	case sentCount == total:
		deliveryStatus = "delivered"
	case skippedCount == total:
		deliveryStatus = "skipped"
	case sentCount > 0:
		deliveryStatus = "partial"
	default:
		deliveryStatus = "failed"
	}

	result := tx.Model(&database.MonitorEvent{}).Where("id = ?", eventID).Updates(map[string]interface{}{
		"delivery_status": deliveryStatus,
		"notified":        deliveryStatus == "delivered" || deliveryStatus == "partial",
	})
	if result.Error != nil {
		return fmt.Errorf("update event delivery status failed: %w", result.Error)
	}
	return nil
}

// PendingDeliveries 查询待投递的任务（含回收过期 sending）
func PendingDeliveries(limit int) ([]database.NotificationDelivery, error) {
	now := time.Now()

	// 回收过期 sending 任务（lease 超时）
	recoverResult := database.GetDB().Model(&database.NotificationDelivery{}).
		Where("status = ? AND lease_until IS NOT NULL AND lease_until < ?", "sending", now).
		Updates(map[string]interface{}{
			"status":          "failed",
			"lease_until":     nil,
			"next_attempt_at": now,
			"last_error":      "delivery lease expired",
		})
	if recoverResult.Error != nil {
		return nil, recoverResult.Error
	}

	var deliveries []database.NotificationDelivery
	if err := database.GetDB().Where("status IN ?", []string{"pending", "failed"}).
		Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now).
		Order("created_at asc").
		Limit(limit).
		Find(&deliveries).Error; err != nil {
		return nil, err
	}
	return deliveries, nil
}

// FormatEvent 格式化事件为通知文本
func FormatEvent(event ChangeEvent, siteName string) (string, string) {
	switch event.EventType {
	case "item_added":
		title := fmt.Sprintf("%s 有新内容", siteName)
		content := fmt.Sprintf("标题: %s\n链接: %s", event.Title, event.URL)
		return title, content
	case "price_dropped":
		title := fmt.Sprintf("降价提醒: %s", event.Title)
		content := fmt.Sprintf("商品: %s\n原价: %s\n现价: %s\n降价: %s (%.2f%%)\n链接: %s",
			event.Title, event.OldValue, event.NewValue,
			formatPrice(event.ChangeAmount, event.Currency), event.ChangePercent,
			event.URL)
		return title, content
	case "price_target_reached":
		title := fmt.Sprintf("到价提醒: %s", event.Title)
		content := fmt.Sprintf("商品: %s\n之前价格: %s\n当前价格: %s\n价格已进入目标范围\n链接: %s",
			event.Title, event.OldValue, event.NewValue, event.URL)
		return title, content
	default:
		title := fmt.Sprintf("%s 有更新", siteName)
		content := fmt.Sprintf("事件: %s\n商品: %s\n链接: %s", event.EventType, event.Title, event.URL)
		return title, content
	}
}

func matchEventKeywords(event ChangeEvent, keywords string) bool {
	kwList := strings.Split(keywords, ",")
	text := strings.ToLower(event.Title + " " + event.NewValue)
	for _, kw := range kwList {
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

// MarkEventNotified 标记事件已通知
func MarkEventNotified(eventID uint) error {
	return database.GetDB().Model(&database.MonitorEvent{}).Where("id = ?", eventID).Update("notified", true).Error
}
