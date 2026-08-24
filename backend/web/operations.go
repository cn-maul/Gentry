package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/fetcher"
	"github.com/cn-maul/Gentry/monitor"
	"github.com/cn-maul/Gentry/notify"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// addMonitorRequest 创建监控器的请求体
type addMonitorRequest struct {
	Name             string            `json:"name" binding:"required"`
	URL              string            `json:"url" binding:"required"`
	Container        string            `json:"container" binding:"required"`
	Item             string            `json:"item"`
	Group            string            `json:"group"`
	CheckInterval    int               `json:"check_interval"`
	IsActive         bool              `json:"is_active"`
	NotifyFilter     string            `json:"notify_filter"`
	NotifyKeywords   string            `json:"notify_keywords"`
	NotifyAccountIDs json.RawMessage   `json:"notify_account_ids"`
	Fields           []fieldRequest    `json:"fields"`
	StrategyType     string            `json:"strategy_type"`
	StrategyConfig   json.RawMessage   `json:"strategy_config"`
	FieldDataTypes   map[string]string `json:"field_data_types"`
	FetchConfig      json.RawMessage   `json:"fetch_config"`
}

type fieldRequest struct {
	Name      string `json:"name" binding:"required"`
	Selector  string `json:"selector"`
	Type      string `json:"type"`
	Attr      string `json:"attr"`
	Transform string `json:"transform"`
}

type monitorConfigResponse struct {
	ID               uint              `json:"id"`
	Name             string            `json:"name"`
	URL              string            `json:"url"`
	Container        string            `json:"container"`
	Item             string            `json:"item"`
	Group            string            `json:"group"`
	CheckInterval    int               `json:"check_interval"`
	IsActive         bool              `json:"is_active"`
	NotifyFilter     string            `json:"notify_filter"`
	NotifyKeywords   string            `json:"notify_keywords"`
	NotifyAccountIDs []uint            `json:"notify_account_ids"`
	Fields           []fieldRequest    `json:"fields"`
	StrategyType     string            `json:"strategy_type,omitempty"`
	StrategyConfig   json.RawMessage   `json:"strategy_config,omitempty"`
	FieldDataTypes   map[string]string `json:"field_data_types,omitempty"`
	FetchConfig      json.RawMessage   `json:"fetch_config,omitempty"`
	BaselineStatus   string            `json:"baseline_status,omitempty"`
}

type monitorSnapshotResponse struct {
	database.MonitorSnapshot
	PriceDisplay string `json:"price_display"`
	ItemTitle    string `json:"item_title,omitempty"`
}

func snapshotItemTitle(snapshot database.MonitorSnapshot) string {
	if strings.TrimSpace(snapshot.PayloadJSON) == "" {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(snapshot.PayloadJSON), &payload); err != nil {
		return ""
	}
	title, ok := payload["title"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(title)
}

func monitorConfigFromSite(site *database.Site) monitorConfigResponse {
	fields := make([]fieldRequest, 0, len(site.Fields))
	for _, f := range site.Fields {
		fields = append(fields, fieldRequest{
			Name:      f.Name,
			Selector:  f.Selector,
			Type:      f.Type,
			Attr:      f.Attr,
			Transform: f.Transform,
		})
	}
	var strategyConfig json.RawMessage
	if site.StrategyConfig != "" {
		strategyConfig = json.RawMessage(site.StrategyConfig)
	}
	var fieldDataTypes map[string]string
	if site.FieldDataTypes != "" {
		json.Unmarshal([]byte(site.FieldDataTypes), &fieldDataTypes)
	}
	var fetchConfig json.RawMessage
	if site.FetchConfig != "" {
		fetchConfig = json.RawMessage(site.FetchConfig)
	}
	return monitorConfigResponse{
		ID:               site.ID,
		Name:             site.Name,
		URL:              site.URL,
		Container:        site.Container,
		Item:             site.Item,
		Group:            site.GroupName,
		CheckInterval:    site.CheckInterval,
		IsActive:         site.IsActive,
		NotifyFilter:     site.NotifyFilter,
		NotifyKeywords:   site.NotifyKeywords,
		NotifyAccountIDs: site.GetNotifyAccountIDs(),
		Fields:           fields,
		StrategyType:     site.StrategyType,
		StrategyConfig:   strategyConfig,
		FieldDataTypes:   fieldDataTypes,
		FetchConfig:      fetchConfig,
		BaselineStatus:   site.BaselineStatus,
	}
}

func normalizeNotifyAccountIDs(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var ids []uint
	if err := json.Unmarshal(raw, &ids); err == nil {
		data, err := json.Marshal(uniqueAccountIDs(ids))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	var legacy string
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return "", fmt.Errorf("notify_account_ids must be an array of numbers")
	}
	if legacy == "" {
		return "", nil
	}
	if err := json.Unmarshal([]byte(legacy), &ids); err != nil {
		return "", fmt.Errorf("notify_account_ids contains invalid JSON array: %w", err)
	}
	data, err := json.Marshal(uniqueAccountIDs(ids))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func uniqueAccountIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func applyNotifyAccountIDs(site *database.Site, raw json.RawMessage) error {
	ids, err := normalizeNotifyAccountIDs(raw)
	if err != nil {
		return err
	}
	site.NotifyAccountIDs = ids
	return nil
}

// dbSiteFromRequest 从请求体构建 database.Site
func dbSiteFromRequest(req *addMonitorRequest) (*database.Site, error) {
	group := req.Group
	if group == "" {
		group = "默认"
	}
	strategyType := req.StrategyType
	if strategyType == "" {
		strategyType = "presence"
	}
	var strategyConfigStr string
	if len(req.StrategyConfig) > 0 {
		strategyConfigStr = string(req.StrategyConfig)
	}
	var fieldDataTypesStr string
	if len(req.FieldDataTypes) > 0 {
		data, _ := json.Marshal(req.FieldDataTypes)
		fieldDataTypesStr = string(data)
	}
	var fetchConfigStr string
	if len(req.FetchConfig) > 0 && string(req.FetchConfig) != "null" {
		fetchConfigStr = string(req.FetchConfig)
	}
	site := &database.Site{
		Name:           req.Name,
		URL:            req.URL,
		Container:      req.Container,
		Item:           req.Item,
		GroupName:      group,
		CheckInterval:  req.CheckInterval,
		IsActive:       req.IsActive,
		NotifyFilter:   req.NotifyFilter,
		NotifyKeywords: req.NotifyKeywords,
		StrategyType:   strategyType,
		StrategyConfig: strategyConfigStr,
		FieldDataTypes: fieldDataTypesStr,
		FetchConfig:    fetchConfigStr,
		BaselineStatus: "pending",
		ConfigVersion:  1,
	}
	if err := applyNotifyAccountIDs(site, req.NotifyAccountIDs); err != nil {
		return nil, err
	}
	site.Fields = siteFieldsFromRequest(req.Fields)
	return site, nil
}

func validateMonitorSourceURL(site *database.Site) error {
	config, err := monitor.ParseFetchConfig(site.FetchConfig, site.URL)
	if err != nil {
		return err
	}
	validationURL, err := monitor.FetchURLValidationTarget(config.URL)
	if err != nil {
		return err
	}
	return validateOutboundURL(validationURL)
}

func siteFieldsFromRequest(fields []fieldRequest) []database.SiteField {
	result := make([]database.SiteField, 0, len(fields))
	for _, f := range fields {
		ft := f.Type
		if ft == "" {
			ft = "text"
		}
		result = append(result, database.SiteField{
			Name:      f.Name,
			Selector:  f.Selector,
			Type:      ft,
			Attr:      f.Attr,
			Transform: f.Transform,
		})
	}
	return result
}

func siteFieldsToRequest(fields []database.SiteField) []fieldRequest {
	result := make([]fieldRequest, 0, len(fields))
	for _, f := range fields {
		result = append(result, fieldRequest{
			Name:      f.Name,
			Selector:  f.Selector,
			Type:      f.Type,
			Attr:      f.Attr,
			Transform: f.Transform,
		})
	}
	return result
}

func (s *WebServer) addMonitor(c *gin.Context) {
	var req addMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid request body: "+err.Error()))
		return
	}

	if !utf8.ValidString(req.Name) || !utf8.ValidString(req.URL) {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "名称或URL包含无效字符，请检查编码"))
		return
	}

	site, err := dbSiteFromRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid notify_account_ids: "+err.Error()))
		return
	}
	if err := monitor.NormalizeAndValidateSiteDefinition(site); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid monitor config: "+err.Error()))
		return
	}
	if err := validateMonitorSourceURL(site); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid monitor source: "+err.Error()))
		return
	}

	if err := database.CreateSiteWithFields(site); err != nil {
		c.JSON(http.StatusConflict, NewErrorResponse(409, "monitor already exists: "+err.Error()))
		return
	}

	if site.IsActive {
		if err := monitor.StartLoadedSite(site); err != nil {
			log.Printf("[Web] 启动新增监控器「%s」失败: %v", site.Name, err)
			// 回滚：删除已创建的监控器记录，避免留下半创建记录
			if dbErr := database.DeleteSiteCascade(site.ID); dbErr != nil {
				log.Printf("[Web] 回滚新增监控器「%s」数据库记录失败: %v", site.Name, dbErr)
			} else {
				monitor.UnregisterMonitor(site.Name)
			}
			c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "创建并启动监控器失败: "+err.Error()))
			return
		}
	} else {
		monitor.RegisterStoppedSite(site)
	}

	log.Printf("[Web] 新增监控器: %s", site.Name)
	c.JSON(http.StatusCreated, NewSuccessResponse(map[string]interface{}{
		"id":   site.ID,
		"name": site.Name,
	}))
}

func (s *WebServer) removeMonitor(c *gin.Context) {
	name := c.Param("name")

	// 如果正在运行则停止
	if monitor.Exists(name) {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := monitor.QuiesceMonitor(name, stopCtx)
		stopCancel()
		if err != nil {
			c.JSON(http.StatusConflict, NewErrorResponse(409, "monitor is busy: "+err.Error()))
			return
		}
	}
	// 从注册表移除
	monitor.UnregisterMonitor(name)

	// 从数据库删除（先清理关联数据）
	var site database.Site
	if err := database.GetDB().Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	if err := database.DeleteSiteCascade(site.ID); err != nil {
		log.Printf("[Web] 删除监控器「%s」失败: %v", name, err)
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "delete failed: "+err.Error()))
		return
	}

	log.Printf("[Web] 删除监控器: %s", name)
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

func (s *WebServer) startMonitor(c *gin.Context) {
	name := c.Param("name")

	if monitor.Exists(name) {
		if monitor.GetMonitor(name).GetStatus().IsRunning {
			c.JSON(http.StatusOK, NewErrorResponse(0, "monitor is already running"))
			return
		}
	}

	if err := monitor.StartSite(name); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "启动失败: "+err.Error()))
		return
	}

	log.Printf("[Web] 启动监控器: %s", name)
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

func (s *WebServer) stopMonitor(c *gin.Context) {
	name := c.Param("name")

	if !monitor.Exists(name) {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	if !monitor.GetMonitor(name).GetStatus().IsRunning {
		c.JSON(http.StatusOK, NewErrorResponse(0, "monitor is already stopped"))
		return
	}

	if err := monitor.StopSite(name); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "停止失败: "+err.Error()))
		return
	}

	log.Printf("[Web] 停止监控器: %s", name)
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

func (s *WebServer) updateMonitor(c *gin.Context) {
	oldName := c.Param("name")

	var req addMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid request body: "+err.Error()))
		return
	}

	// 从数据库查找（必须 Preload Fields）
	var originalSite database.Site
	if err := database.GetDB().Preload("Fields").Where("name = ?", oldName).First(&originalSite).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	// 计算旧指纹（在修改 site 之前）
	originalSite.Fields = append([]database.SiteField(nil), originalSite.Fields...)
	oldFields := siteFieldsToRequest(originalSite.Fields)
	oldFingerprint := computeDetectionFingerprintWithFetch(originalSite.URL, originalSite.Container, originalSite.Item, oldFields, originalSite.StrategyType, originalSite.StrategyConfig, originalSite.FieldDataTypes, originalSite.FetchConfig)

	// 构建新配置
	group := req.Group
	if group == "" {
		group = "默认"
	}
	strategyType := req.StrategyType
	if strategyType == "" {
		strategyType = "presence"
	}
	var strategyConfigStr string
	if len(req.StrategyConfig) > 0 {
		strategyConfigStr = string(req.StrategyConfig)
	}
	var fieldDataTypesStr string
	if len(req.FieldDataTypes) > 0 {
		data, _ := json.Marshal(req.FieldDataTypes)
		fieldDataTypesStr = string(data)
	}
	var fetchConfigStr string
	if len(req.FetchConfig) > 0 && string(req.FetchConfig) != "null" {
		fetchConfigStr = string(req.FetchConfig)
	}

	// 构建并校验候选定义
	candidate := originalSite
	candidate.Name = req.Name
	candidate.URL = req.URL
	candidate.Container = req.Container
	candidate.Item = req.Item
	candidate.GroupName = group
	candidate.CheckInterval = req.CheckInterval
	candidate.IsActive = req.IsActive
	candidate.NotifyFilter = req.NotifyFilter
	candidate.NotifyKeywords = req.NotifyKeywords
	candidate.StrategyType = strategyType
	candidate.StrategyConfig = strategyConfigStr
	candidate.FieldDataTypes = fieldDataTypesStr
	candidate.FetchConfig = fetchConfigStr
	candidate.Fields = siteFieldsFromRequest(req.Fields)
	if err := applyNotifyAccountIDs(&candidate, req.NotifyAccountIDs); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid notify_account_ids: "+err.Error()))
		return
	}
	if err := monitor.NormalizeAndValidateSiteDefinition(&candidate); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid monitor config: "+err.Error()))
		return
	}
	if err := validateMonitorSourceURL(&candidate); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid monitor source: "+err.Error()))
		return
	}

	newFingerprint := computeDetectionFingerprintWithFetch(candidate.URL, candidate.Container, candidate.Item, siteFieldsToRequest(candidate.Fields), candidate.StrategyType, candidate.StrategyConfig, candidate.FieldDataTypes, candidate.FetchConfig)
	needsBaseline := oldFingerprint != newFingerprint
	if needsBaseline {
		candidate.ConfigVersion++
		candidate.BaselineStatus = "needs_baseline"
	}

	// 先停止旧实例并等待在途检查退出，防止旧定义在事务后写回快照。
	quiesceCtx, quiesceCancel := context.WithTimeout(context.Background(), 15*time.Second)
	quiesceErr := monitor.QuiesceMonitor(oldName, quiesceCtx)
	quiesceCancel()
	if quiesceErr != nil {
		if _, restoreErr := monitor.AtomicReplaceMonitor(&originalSite, oldName); restoreErr != nil {
			log.Printf("[Web] 中止更新后恢复旧监控器「%s」失败: %v", oldName, restoreErr)
		}
		c.JSON(http.StatusConflict, NewErrorResponse(409, "monitor is busy: "+quiesceErr.Error()))
		return
	}

	if err := database.UpdateMonitorDefinition(&candidate, candidate.Fields, needsBaseline); err != nil {
		if _, restoreErr := monitor.AtomicReplaceMonitor(&originalSite, oldName); restoreErr != nil {
			log.Printf("[Web] 恢复旧监控器「%s」失败: %v", oldName, restoreErr)
		}
		log.Printf("[Web] 更新监控器「%s」失败: %v", oldName, err)
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "update failed: "+err.Error()))
		return
	}
	site := candidate

	// 使用 AtomicReplaceMonitor 原子式处理内存 registry 更新
	if req.IsActive {
		var updatedSite database.Site
		if err := database.GetDB().Preload("Fields").First(&updatedSite, site.ID).Error; err != nil {
			log.Printf("[Web] 重新加载更新后的站点失败: %v", err)
			c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "reload updated monitor failed: "+err.Error()))
			return
		}
		updatedSite.IsActive = true
		if _, err := monitor.AtomicReplaceMonitor(&updatedSite, oldName); err != nil {
			log.Printf("[Web] 重启监控器「%s」失败: %v", updatedSite.Name, err)
			c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "restart failed: "+err.Error()))
			return
		}
	} else {
		var updatedSite database.Site
		if err := database.GetDB().Preload("Fields").First(&updatedSite, site.ID).Error; err != nil {
			log.Printf("[Web] 重新加载更新后的站点失败: %v", err)
			c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "reload updated monitor failed: "+err.Error()))
			return
		}
		updatedSite.IsActive = false
		if _, err := monitor.AtomicReplaceMonitor(&updatedSite, oldName); err != nil {
			log.Printf("[Web] 替换停止监控器「%s」失败: %v", updatedSite.Name, err)
			c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "replace failed: "+err.Error()))
			return
		}
	}

	newName := req.Name
	if newName == "" {
		newName = oldName
	}
	log.Printf("[Web] 更新监控器: %s -> %s", oldName, newName)
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

func (s *WebServer) getUpdates(c *gin.Context) {
	name := c.Param("name")

	var site database.Site
	if err := database.GetDB().Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	page := 1
	pageSize := 20
	if rawPage := c.Query("page"); rawPage != "" {
		parsed, err := strconv.Atoi(rawPage)
		if err != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, "page must be a positive integer"))
			return
		}
		page = parsed
	}
	if rawSize := c.Query("size"); rawSize != "" {
		parsed, err := strconv.Atoi(rawSize)
		if err != nil || parsed < 1 || parsed > 100 {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, "size must be between 1 and 100"))
			return
		}
		pageSize = parsed
	}

	var total int64
	if err := database.GetDB().Model(&database.UpdateRecord{}).
		Where("site_id = ?", site.ID).
		Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to count updates: "+err.Error()))
		return
	}

	var records []database.UpdateRecord
	if err := database.GetDB().Where("site_id = ?", site.ID).
		Order("created_at desc, id asc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to load updates: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"total":   total,
		"page":    page,
		"size":    pageSize,
		"records": records,
	}))
}

func (s *WebServer) getMonitorConfig(c *gin.Context) {
	name := c.Param("name")

	var site database.Site
	if err := database.GetDB().Preload("Fields").Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	c.JSON(http.StatusOK, NewSuccessResponse(monitorConfigFromSite(&site)))
}

func (s *WebServer) markAllNotified(c *gin.Context) {
	name := c.Param("name")

	var site database.Site
	if err := database.GetDB().Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	now := time.Now()
	result := database.GetDB().Model(&database.UpdateRecord{}).
		Where("site_id = ? AND notified = ?", site.ID, false).
		Updates(map[string]interface{}{
			"notified":    true,
			"notified_at": now,
		})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to mark updates as notified: "+result.Error.Error()))
		return
	}

	log.Printf("[Web] 标记 %s 的 %d 条记录为已推送", name, result.RowsAffected)
	c.JSON(http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"updated": result.RowsAffected,
	}))
}

func (s *WebServer) markRead(c *gin.Context) {
	name := c.Param("name")
	var site database.Site
	if err := database.GetDB().Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}
	if err := database.GetDB().Model(&database.UpdateRecord{}).
		Where("site_id = ? AND is_read = ?", site.ID, false).
		Update("is_read", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to mark updates as read: "+err.Error()))
		return
	}
	monitor.MarkRead(name)
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

func (s *WebServer) updateNotifyAccounts(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		AccountIDs json.RawMessage `json:"notify_account_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}

	var site database.Site
	if err := database.GetDB().Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	if err := applyNotifyAccountIDs(&site, req.AccountIDs); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid notify_account_ids: "+err.Error()))
		return
	}
	if err := database.GetDB().Save(&site).Error; err != nil {
		log.Printf("[Web] 更新推送账户失败「%s」: %v", name, err)
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "保存失败: "+err.Error()))
		return
	}

	// 同步更新运行中的监控器实例（避免下次检查周期仍用旧配置）
	if m := monitor.GetMonitor(name); m != nil {
		m.UpdateSiteNotifyAccounts(site.NotifyAccountIDs)
	}

	log.Printf("[Web] 更新 %s 的推送账户: %s", name, site.NotifyAccountIDs)
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

func (s *WebServer) listGroups(c *gin.Context) {
	var groups []string
	if err := database.GetDB().Model(&database.Site{}).
		Select("DISTINCT group_name").
		Order("group_name asc").
		Pluck("group_name", &groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to load groups: "+err.Error()))
		return
	}

	if len(groups) == 0 {
		groups = []string{"默认"}
	}

	c.JSON(http.StatusOK, NewSuccessResponse(groups))
}

func (s *WebServer) healthCheck(c *gin.Context) {
	db := database.GetDB()
	sqlDB, err := db.DB()
	dbOk := err == nil && sqlDB.Ping() == nil

	c.JSON(http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"status":   "ok",
		"database": dbOk,
		"monitors": len(monitor.GetAllMonitors()),
	}))
}

func (s *WebServer) getStats(c *gin.Context) {
	db := database.GetDB()

	var totalMonitors int64
	db.Model(&database.Site{}).Count(&totalMonitors)

	runningMonitors := 0
	for _, status := range monitor.GetAllMonitors() {
		if status.IsRunning {
			runningMonitors++
		}
	}

	legacyScope := func() *gorm.DB {
		return db.Model(&database.UpdateRecord{}).
			Joins("JOIN sites ON sites.id = update_records.site_id").
			Where("COALESCE(sites.strategy_type, 'presence') <> ?", "field_transition")
	}
	var legacyTotal, eventTotal int64
	legacyScope().Count(&legacyTotal)
	db.Model(&database.MonitorEvent{}).Count(&eventTotal)
	totalUpdates := legacyTotal + eventTotal

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	var legacyLastHour, eventsLastHour int64
	legacyScope().Where("update_records.created_at >= ?", oneHourAgo).Count(&legacyLastHour)
	db.Model(&database.MonitorEvent{}).Where("created_at >= ?", oneHourAgo).Count(&eventsLastHour)
	updatesLastHour := legacyLastHour + eventsLastHour

	var legacyPending, eventPending int64
	legacyScope().Where("update_records.notified = ?", false).Count(&legacyPending)
	db.Model(&database.MonitorEvent{}).Where("delivery_status = ?", "pending").Count(&eventPending)
	unnotifiedUpdates := legacyPending + eventPending

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var legacyPushedToday, eventPushedToday int64
	legacyScope().Where("update_records.notified = ? AND update_records.notified_at >= ?", true, todayStart).Count(&legacyPushedToday)
	db.Model(&database.NotificationDelivery{}).
		Distinct("event_id").
		Where("status = ? AND sent_at >= ?", "sent", todayStart).
		Count(&eventPushedToday)
	pushedToday := legacyPushedToday + eventPushedToday

	var totalAccounts int64
	db.Model(&database.NotificationAccount{}).Count(&totalAccounts)

	c.JSON(http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"total_monitors":     totalMonitors,
		"running_monitors":   runningMonitors,
		"total_updates":      totalUpdates,
		"updates_last_hour":  updatesLastHour,
		"unnotified_updates": unnotifiedUpdates,
		"pushed_today":       pushedToday,
		"total_accounts":     totalAccounts,
	}))
}

// ===== 推送账户 CRUD =====

type scanRuleFieldRequest struct {
	Name      string `json:"name" binding:"required"`
	Selector  string `json:"selector"`
	Type      string `json:"type"`
	Attr      string `json:"attr"`
	Transform string `json:"transform"`
}

type scanRuleRequest struct {
	Name        string                 `json:"name" binding:"required"`
	URLContains string                 `json:"url_contains" binding:"required_without=ScopeType"`
	SourceURL   string                 `json:"source_url"`
	ScopeType   string                 `json:"scope_type"`
	Container   string                 `json:"container" binding:"required"`
	Item        string                 `json:"item" binding:"required"`
	Priority    int                    `json:"priority"`
	Enabled     *bool                  `json:"enabled"`
	Description string                 `json:"description"`
	FetchConfig json.RawMessage        `json:"fetch_config"`
	Fields      []scanRuleFieldRequest `json:"fields"`
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type scanRuleResponse struct {
	ID          uint                   `json:"id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Name        string                 `json:"name"`
	URLContains string                 `json:"url_contains"`
	SourceURL   string                 `json:"source_url"`
	ScopeType   string                 `json:"scope_type"`
	MatchHost   string                 `json:"match_host"`
	MatchPath   string                 `json:"match_path"`
	MatchQuery  string                 `json:"match_query"`
	Container   string                 `json:"container"`
	Item        string                 `json:"item"`
	Priority    int                    `json:"priority"`
	Enabled     bool                   `json:"enabled"`
	Description string                 `json:"description"`
	FetchConfig json.RawMessage        `json:"fetch_config,omitempty"`
	Fields      []scanRuleFieldRequest `json:"fields"`
}

func scanRuleFromModel(rule database.ScanRuleTemplate) scanRuleResponse {
	fields := make([]scanRuleFieldRequest, 0, len(rule.Fields))
	for _, f := range rule.Fields {
		fields = append(fields, scanRuleFieldRequest{
			Name:      f.Name,
			Selector:  f.Selector,
			Type:      f.Type,
			Attr:      f.Attr,
			Transform: f.Transform,
		})
	}
	var fetchConfig json.RawMessage
	if rule.FetchConfig != "" {
		fetchConfig = json.RawMessage(rule.FetchConfig)
	}
	return scanRuleResponse{
		ID:          rule.ID,
		CreatedAt:   rule.CreatedAt,
		UpdatedAt:   rule.UpdatedAt,
		Name:        rule.Name,
		URLContains: rule.URLContains,
		SourceURL:   rule.SourceURL,
		ScopeType:   rule.ScopeType,
		MatchHost:   rule.MatchHost,
		MatchPath:   rule.MatchPath,
		MatchQuery:  rule.MatchQuery,
		Container:   rule.Container,
		Item:        rule.Item,
		Priority:    rule.Priority,
		Enabled:     rule.Enabled,
		Description: rule.Description,
		FetchConfig: fetchConfig,
		Fields:      fields,
	}
}

func scanRulesFromModels(rules []database.ScanRuleTemplate) []scanRuleResponse {
	result := make([]scanRuleResponse, 0, len(rules))
	for _, rule := range rules {
		result = append(result, scanRuleFromModel(rule))
	}
	return result
}

func scanRulesForExport(rules []database.ScanRuleTemplate) []scanRuleResponse {
	result := scanRulesFromModels(rules)
	for index := range result {
		if result[index].ScopeType != monitor.ScanRuleScopeSection || result[index].MatchHost == "" || result[index].MatchPath == "" {
			continue
		}
		scheme := "https"
		if source, err := url.Parse(result[index].SourceURL); err == nil && (source.Scheme == "http" || source.Scheme == "https") {
			scheme = source.Scheme
		}
		scopeURL := &url.URL{
			Scheme: scheme,
			Host:   result[index].MatchHost,
			Path:   strings.TrimSuffix(result[index].MatchPath, "/") + "/",
		}
		result[index].SourceURL = scopeURL.String()
	}
	return result
}

func scanRuleFieldsFromRequest(fields []scanRuleFieldRequest) []database.ScanRuleField {
	result := make([]database.ScanRuleField, 0, len(fields))
	for _, f := range fields {
		ft := f.Type
		if ft == "" {
			ft = "text"
		}
		result = append(result, database.ScanRuleField{
			Name:      f.Name,
			Selector:  f.Selector,
			Type:      ft,
			Attr:      f.Attr,
			Transform: f.Transform,
		})
	}
	return result
}

func dbScanRuleFromRequest(req *scanRuleRequest) *database.ScanRuleTemplate {
	priority := req.Priority
	if priority <= 0 {
		priority = 50
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule := &database.ScanRuleTemplate{
		Name:        req.Name,
		URLContains: req.URLContains,
		Container:   req.Container,
		Item:        req.Item,
		Priority:    priority,
		Enabled:     enabled,
		Description: req.Description,
		FetchConfig: string(req.FetchConfig),
		Fields:      scanRuleFieldsFromRequest(req.Fields),
	}
	return rule
}

type quickScanRuleRequest struct {
	Name      string                    `json:"name" binding:"required"`
	URL       string                    `json:"url" binding:"required"`
	Keywords  string                    `json:"keywords"`
	ScopeType string                    `json:"scope_type"`
	Config    monitor.ScanMonitorConfig `json:"config" binding:"required"`
}

type scanRuleImportRequest struct {
	Name        string                 `json:"name"`
	URLContains string                 `json:"url_contains"`
	SourceURL   string                 `json:"source_url"`
	ScopeType   string                 `json:"scope_type"`
	MatchHost   string                 `json:"match_host"`
	MatchPath   string                 `json:"match_path"`
	MatchQuery  string                 `json:"match_query"`
	Container   string                 `json:"container"`
	Item        string                 `json:"item"`
	Priority    int                    `json:"priority"`
	Enabled     *bool                  `json:"enabled"`
	Description string                 `json:"description"`
	FetchConfig json.RawMessage        `json:"fetch_config"`
	Fields      []scanRuleFieldRequest `json:"fields"`
}

type scanRuleExportDocument struct {
	Version    int                `json:"version"`
	ExportedAt time.Time          `json:"exported_at"`
	Rules      []scanRuleResponse `json:"rules"`
}

func validateScanRuleDefinition(container, item string, fields []scanRuleFieldRequest) error {
	container = strings.TrimSpace(container)
	item = strings.TrimSpace(item)
	if container == "" || item == "" {
		return fmt.Errorf("扫描结果缺少容器或列表项配置")
	}
	if _, err := cascadia.Compile(container); err != nil {
		return fmt.Errorf("容器选择器无效: %w", err)
	}
	if _, err := cascadia.Compile(item); err != nil {
		return fmt.Errorf("列表项选择器无效: %w", err)
	}
	hasTitle := false
	for _, field := range fields {
		if strings.TrimSpace(field.Name) == "" {
			return fmt.Errorf("字段名称不能为空")
		}
		if field.Name == "title" {
			hasTitle = true
		}
		if strings.TrimSpace(field.Selector) != "" {
			if _, err := cascadia.Compile(field.Selector); err != nil {
				return fmt.Errorf("字段 %q 的选择器无效: %w", field.Name, err)
			}
		}
	}
	if !hasTitle {
		return fmt.Errorf("规则至少需要一个 title 字段")
	}
	return nil
}

func validateScanRuleConfig(container, item string, fields []scanRuleFieldRequest, fetchConfig *monitor.FetchConfig) error {
	if fetchConfig == nil || fetchConfig.Mode == "" || fetchConfig.Mode == monitor.FetchModeHTML {
		return validateScanRuleDefinition(container, item, fields)
	}
	if fetchConfig.Mode != monitor.FetchModeAPIJSON {
		return fmt.Errorf("不支持的规则数据源模式: %s", fetchConfig.Mode)
	}
	if strings.TrimSpace(fetchConfig.URL) == "" || strings.TrimSpace(fetchConfig.ItemsPath) == "" {
		return fmt.Errorf("JSON API 规则缺少接口 URL 或列表路径")
	}
	hasTitle := false
	for _, field := range fields {
		if strings.TrimSpace(field.Name) == "" || strings.TrimSpace(field.Selector) == "" {
			return fmt.Errorf("JSON API 规则字段名称和路径不能为空")
		}
		if field.Name == "title" {
			hasTitle = true
		}
	}
	if !hasTitle {
		return fmt.Errorf("规则至少需要一个 title 字段")
	}
	return nil
}

func validateScanRuleSource(rule *database.ScanRuleTemplate, fallbackURL string) error {
	if strings.TrimSpace(rule.FetchConfig) == "" {
		return nil
	}
	config, err := monitor.ParseFetchConfig(rule.FetchConfig, fallbackURL)
	if err != nil {
		return err
	}
	validationURL, err := monitor.FetchURLValidationTarget(config.URL)
	if err != nil {
		return err
	}
	return validateOutboundURL(validationURL)
}

func scanRuleFieldsFromConfig(fields []monitor.ScanFieldConfig) []scanRuleFieldRequest {
	result := make([]scanRuleFieldRequest, 0, len(fields))
	for _, field := range fields {
		result = append(result, scanRuleFieldRequest{
			Name: field.Name, Selector: field.Selector, Type: field.Type,
			Attr: field.Attr, Transform: field.Transform,
		})
	}
	return result
}

func dbQuickScanRuleFromRequest(req *quickScanRuleRequest) (*database.ScanRuleTemplate, error) {
	fields := scanRuleFieldsFromConfig(req.Config.Fields)
	if err := validateScanRuleConfig(req.Config.Container, req.Config.Item, fields, req.Config.Fetch); err != nil {
		return nil, err
	}
	var fetchConfig string
	if req.Config.Fetch != nil {
		data, err := json.Marshal(req.Config.Fetch)
		if err != nil {
			return nil, err
		}
		fetchConfig = string(data)
	}
	rule := &database.ScanRuleTemplate{
		Name:        strings.TrimSpace(req.Name),
		Container:   strings.TrimSpace(req.Config.Container),
		Item:        strings.TrimSpace(req.Config.Item),
		Priority:    50,
		Enabled:     true,
		Description: "通过快速扫描生成",
		FetchConfig: fetchConfig,
		Fields:      scanRuleFieldsFromRequest(fields),
	}
	if rule.Name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	if err := monitor.ApplyScanRuleScope(rule, req.URL, req.ScopeType); err != nil {
		return nil, err
	}
	if _, err := monitor.GeneralizeKnownPriceRule(rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func dbImportedScanRule(req scanRuleImportRequest) (*database.ScanRuleTemplate, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	priority := req.Priority
	if priority <= 0 {
		priority = 50
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	var parsedFetch *monitor.FetchConfig
	if len(req.FetchConfig) > 0 && string(req.FetchConfig) != "null" {
		var config monitor.FetchConfig
		if err := json.Unmarshal(req.FetchConfig, &config); err != nil {
			return nil, fmt.Errorf("fetch_config 无效: %w", err)
		}
		parsedFetch = &config
	}
	if err := validateScanRuleConfig(req.Container, req.Item, req.Fields, parsedFetch); err != nil {
		return nil, err
	}
	rule := &database.ScanRuleTemplate{
		Name:        strings.TrimSpace(req.Name),
		URLContains: strings.TrimSpace(req.URLContains),
		Container:   strings.TrimSpace(req.Container),
		Item:        strings.TrimSpace(req.Item),
		Priority:    priority,
		Enabled:     enabled,
		Description: req.Description,
		FetchConfig: string(req.FetchConfig),
		Fields:      scanRuleFieldsFromRequest(req.Fields),
	}
	if req.ScopeType == "" {
		if rule.URLContains == "" {
			return nil, fmt.Errorf("旧版规则缺少 url_contains")
		}
		return rule, nil
	}
	if req.ScopeType == monitor.ScanRuleScopeSection && strings.TrimSpace(req.MatchHost) != "" && strings.TrimSpace(req.MatchPath) != "" {
		source, err := url.Parse(strings.TrimSpace(req.SourceURL))
		if err != nil || source.Scheme == "" || source.Host == "" {
			return nil, fmt.Errorf("同站目录规则的 source_url 无效")
		}
		matchPath := path.Clean("/" + strings.TrimPrefix(req.MatchPath, "/"))
		if !strings.EqualFold(source.Host, req.MatchHost) || strings.TrimSuffix(path.Clean(source.Path), "/") != strings.TrimSuffix(matchPath, "/") {
			return nil, fmt.Errorf("同站目录规则的 source_url 与匹配范围不一致")
		}
		scopeProbe := *source
		scopeProbe.Path = strings.TrimSuffix(matchPath, "/") + "/__gentry_scope__"
		if err := monitor.ApplyScanRuleScope(rule, scopeProbe.String(), req.ScopeType); err != nil {
			return nil, err
		}
		rule.SourceURL = source.String()
		return rule, nil
	}
	if err := monitor.ApplyScanRuleScope(rule, req.SourceURL, req.ScopeType); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *WebServer) listScanRules(c *gin.Context) {
	var rules []database.ScanRuleTemplate
	if err := database.GetDB().Preload("Fields").Order("priority desc, created_at asc").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "加载扫描规则失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewSuccessResponse(scanRulesFromModels(rules)))
}

func (s *WebServer) createScanRule(c *gin.Context) {
	var req scanRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}
	rule := dbScanRuleFromRequest(&req)
	if err := validateScanRuleSource(rule, ""); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则数据源无效: "+err.Error()))
		return
	}
	if err := database.CreateScanRuleTemplate(rule); err != nil {
		c.JSON(http.StatusConflict, NewErrorResponse(409, "创建扫描规则失败: "+err.Error()))
		return
	}
	if err := database.GetDB().Preload("Fields").First(rule, rule.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "读取新建扫描规则失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusCreated, NewSuccessResponse(scanRuleFromModel(*rule)))
}

func (s *WebServer) quickCreateScanRule(c *gin.Context) {
	var req quickScanRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "URL 无效: "+err.Error()))
		return
	}
	rule, err := dbQuickScanRuleFromRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则无效: "+err.Error()))
		return
	}
	if rule.FetchConfig != "" {
		config, parseErr := monitor.ParseFetchConfig(rule.FetchConfig, req.URL)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则数据源无效: "+parseErr.Error()))
			return
		}
		validationURL, validationErr := monitor.FetchURLValidationTarget(config.URL)
		if validationErr != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则数据源 URL 无效: "+validationErr.Error()))
			return
		}
		if err := validateOutboundURL(validationURL); err != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则数据源 URL 无效: "+err.Error()))
			return
		}
	}
	if err := database.CreateScanRuleTemplate(rule); err != nil {
		c.JSON(http.StatusConflict, NewErrorResponse(409, "创建扫描规则失败: "+err.Error()))
		return
	}
	if err := database.GetDB().Preload("Fields").First(rule, rule.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "读取新建扫描规则失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusCreated, NewSuccessResponse(scanRuleFromModel(*rule)))
}

func (s *WebServer) exportScanRules(c *gin.Context) {
	var rules []database.ScanRuleTemplate
	if err := database.GetDB().Preload("Fields").Order("priority desc, created_at asc").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "导出扫描规则失败: "+err.Error()))
		return
	}
	c.Header("Content-Disposition", `attachment; filename="gentry-scan-rules.json"`)
	c.JSON(http.StatusOK, NewSuccessResponse(scanRuleExportDocument{
		Version: 1, ExportedAt: time.Now(), Rules: scanRulesForExport(rules),
	}))
}

func (s *WebServer) importScanRules(c *gin.Context) {
	var document struct {
		Version int                     `json:"version"`
		Rules   []scanRuleImportRequest `json:"rules"`
	}
	if err := c.ShouldBindJSON(&document); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则文件无效: "+err.Error()))
		return
	}
	if document.Version != 1 {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, fmt.Sprintf("不支持的规则文件版本: %d", document.Version)))
		return
	}
	if len(document.Rules) == 0 || len(document.Rules) > 500 {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则文件必须包含 1 到 500 条规则"))
		return
	}

	prepared := make([]*database.ScanRuleTemplate, 0, len(document.Rules))
	seen := make(map[string]struct{}, len(document.Rules))
	for index, imported := range document.Rules {
		rule, err := dbImportedScanRule(imported)
		if err != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, fmt.Sprintf("第 %d 条规则无效: %v", index+1, err)))
			return
		}
		if err := validateScanRuleSource(rule, rule.SourceURL); err != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, fmt.Sprintf("第 %d 条规则数据源无效: %v", index+1, err)))
			return
		}
		if _, exists := seen[rule.Name]; exists {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, fmt.Sprintf("规则文件中名称重复: %s", rule.Name)))
			return
		}
		seen[rule.Name] = struct{}{}
		prepared = append(prepared, rule)
	}

	var existing []string
	if err := database.GetDB().Model(&database.ScanRuleTemplate{}).Pluck("name", &existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "读取现有规则失败: "+err.Error()))
		return
	}
	existingNames := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		existingNames[name] = struct{}{}
	}
	toCreate := make([]*database.ScanRuleTemplate, 0, len(prepared))
	skipped := 0
	for _, rule := range prepared {
		if _, exists := existingNames[rule.Name]; exists {
			skipped++
			continue
		}
		toCreate = append(toCreate, rule)
	}

	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		for _, rule := range toCreate {
			if err := tx.Omit("Fields").Create(rule).Error; err != nil {
				return err
			}
			for index := range rule.Fields {
				rule.Fields[index].RuleID = rule.ID
			}
			if len(rule.Fields) > 0 {
				if err := tx.Create(&rule.Fields).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "导入扫描规则失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewSuccessResponse(map[string]int{"imported": len(toCreate), "skipped": skipped}))
}

func (s *WebServer) updateScanRule(c *gin.Context) {
	var req scanRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}
	id := c.Param("id")
	var rule database.ScanRuleTemplate
	if err := database.GetDB().Preload("Fields").First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "扫描规则不存在"))
		return
	}
	updated := dbScanRuleFromRequest(&req)
	var parsedFetch *monitor.FetchConfig
	if updated.FetchConfig != "" {
		config, parseErr := monitor.ParseFetchConfig(updated.FetchConfig, firstNonEmptyString(req.SourceURL, rule.SourceURL))
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则数据源无效: "+parseErr.Error()))
			return
		}
		parsedFetch = &config
	}
	if err := validateScanRuleConfig(updated.Container, updated.Item, req.Fields, parsedFetch); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则无效: "+err.Error()))
		return
	}
	if err := validateScanRuleSource(updated, rule.SourceURL); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则数据源无效: "+err.Error()))
		return
	}
	rule.Name = updated.Name
	rule.URLContains = updated.URLContains
	rule.Container = updated.Container
	rule.Item = updated.Item
	rule.Priority = updated.Priority
	rule.Enabled = updated.Enabled
	rule.Description = updated.Description
	rule.FetchConfig = updated.FetchConfig
	if req.ScopeType != "" {
		sourceURL := firstNonEmptyString(req.SourceURL, rule.SourceURL)
		if err := monitor.ApplyScanRuleScope(&rule, sourceURL, req.ScopeType); err != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则适用范围无效: "+err.Error()))
			return
		}
	}
	if _, err := monitor.GeneralizeKnownPriceRule(&rule); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则通用化失败: "+err.Error()))
		return
	}
	if err := database.UpdateScanRuleTemplate(&rule, updated.Fields); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "更新扫描规则失败: "+err.Error()))
		return
	}
	if err := database.GetDB().Preload("Fields").First(&rule, rule.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "读取更新后的扫描规则失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewSuccessResponse(scanRuleFromModel(rule)))
}

func (s *WebServer) deleteScanRule(c *gin.Context) {
	id := c.Param("id")
	var rule database.ScanRuleTemplate
	if err := database.GetDB().First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "扫描规则不存在"))
		return
	}
	if err := database.DeleteScanRuleTemplate(rule.ID); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "删除扫描规则失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

type accountRequest struct {
	Name    string                 `json:"name" binding:"required"`
	Service string                 `json:"service" binding:"required"`
	Config  map[string]interface{} `json:"config" binding:"required"`
}

type accountResponse struct {
	ID        uint                   `json:"id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Name      string                 `json:"name"`
	Service   string                 `json:"service"`
	Config    map[string]interface{} `json:"config"`
}

func accountFromModel(account database.NotificationAccount) accountResponse {
	config := map[string]interface{}{}
	if account.ConfigJSON != "" {
		if err := json.Unmarshal([]byte(account.ConfigJSON), &config); err != nil {
			log.Printf("[通知] 解析账户配置失败 #%d: %v", account.ID, err)
		}
	}
	return accountResponse{
		ID:        account.ID,
		CreatedAt: account.CreatedAt,
		UpdatedAt: account.UpdatedAt,
		Name:      account.Name,
		Service:   account.Service,
		Config:    maskSensitiveConfig(account.Service, config),
	}
}

func accountsFromModels(accounts []database.NotificationAccount) []accountResponse {
	result := make([]accountResponse, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, accountFromModel(account))
	}
	return result
}

func (s *WebServer) listAccounts(c *gin.Context) {
	var accounts []database.NotificationAccount
	if err := database.GetDB().Order("created_at desc").Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to load notification accounts: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, NewSuccessResponse(accountsFromModels(accounts)))
}

func (s *WebServer) createAccount(c *gin.Context) {
	var req accountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}
	if !utf8.ValidString(req.Name) {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "账户名称包含无效字符，请检查编码"))
		return
	}

	configJSON, marshalErr := json.Marshal(req.Config)
	if marshalErr != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "配置参数无效: "+marshalErr.Error()))
		return
	}
	account := &database.NotificationAccount{
		Name:       req.Name,
		Service:    req.Service,
		ConfigJSON: string(configJSON),
	}
	if err := notify.ValidateAccountConfig(req.Service, req.Config); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "账户配置无效: "+err.Error()))
		return
	}
	if req.Service == "webhook" {
		if webhookURL, _ := req.Config["url"].(string); webhookURL != "" {
			if err := validateOutboundURL(webhookURL); err != nil {
				c.JSON(http.StatusBadRequest, NewErrorResponse(400, "Webhook URL 无效: "+err.Error()))
				return
			}
		}
	}
	if err := database.GetDB().Create(account).Error; err != nil {
		c.JSON(http.StatusConflict, NewErrorResponse(409, "创建账户失败: "+err.Error()))
		return
	}

	log.Printf("[通知] 创建推送账户: %s (%s)", account.Name, account.Service)
	c.JSON(http.StatusCreated, NewSuccessResponse(accountFromModel(*account)))
}

func (s *WebServer) updateAccount(c *gin.Context) {
	var req accountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}
	if !utf8.ValidString(req.Name) {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "账户名称包含无效字符，请检查编码"))
		return
	}

	id := c.Param("id")
	var account database.NotificationAccount
	if err := database.GetDB().First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "账户不存在"))
		return
	}

	mergedConfig, mergeErr := mergeMaskedSensitiveConfig(req.Service, req.Config, account.ConfigJSON)
	if mergeErr != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "配置参数无效: "+mergeErr.Error()))
		return
	}
	configJSON, marshalErr := json.Marshal(mergedConfig)
	if marshalErr != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "配置参数无效: "+marshalErr.Error()))
		return
	}
	if err := notify.ValidateAccountConfig(req.Service, mergedConfig); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "账户配置无效: "+err.Error()))
		return
	}
	if req.Service == "webhook" {
		if webhookURL, _ := req.Config["url"].(string); webhookURL != "" {
			if err := validateOutboundURL(webhookURL); err != nil {
				c.JSON(http.StatusBadRequest, NewErrorResponse(400, "Webhook URL 无效: "+err.Error()))
				return
			}
		}
	}
	account.Name = req.Name
	account.Service = req.Service
	account.ConfigJSON = string(configJSON)
	if err := database.GetDB().Save(&account).Error; err != nil {
		log.Printf("[通知] 更新推送账户失败「%s」: %v", account.Name, err)
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "更新失败: "+err.Error()))
		return
	}

	log.Printf("[通知] 更新推送账户: %s", account.Name)
	c.JSON(http.StatusOK, NewSuccessResponse(accountFromModel(account)))
}

func mergeMaskedSensitiveConfig(service string, incoming map[string]interface{}, existingJSON string) (map[string]interface{}, error) {
	merged := make(map[string]interface{}, len(incoming))
	for key, value := range incoming {
		merged[key] = value
	}
	if existingJSON == "" {
		return merged, nil
	}
	var existing map[string]interface{}
	if err := json.Unmarshal([]byte(existingJSON), &existing); err != nil {
		return nil, err
	}
	keys := []string{}
	switch service {
	case "pushplus":
		keys = []string{"token"}
	case "serverchan":
		keys = []string{"sendkey"}
	case "webhook":
		keys = []string{"url"}
	case "bark":
		keys = []string{"key"}
	}
	for _, key := range keys {
		incomingValue, incomingOK := merged[key].(string)
		existingValue, existingOK := existing[key].(string)
		if incomingOK && existingOK && incomingValue == maskSecret(existingValue) {
			merged[key] = existingValue
		}
	}
	return merged, nil
}

func (s *WebServer) deleteAccount(c *gin.Context) {
	id := c.Param("id")
	var sites []database.Site
	if err := database.GetDB().Find(&sites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to check account references: "+err.Error()))
		return
	}
	for _, site := range sites {
		for _, accountID := range site.GetNotifyAccountIDs() {
			if fmt.Sprintf("%d", accountID) == id {
				c.JSON(http.StatusConflict, NewErrorResponse(409, "该账户仍被监控器引用，无法删除"))
				return
			}
		}
	}
	result := database.GetDB().Delete(&database.NotificationAccount{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "删除账户失败: "+result.Error.Error()))
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "账户不存在"))
		return
	}
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

func (s *WebServer) listNotificationProviders(c *gin.Context) {
	providers := notify.ListProviderMetadata()
	c.JSON(http.StatusOK, NewSuccessResponse(providers))
}

// 推送全局开关

func (s *WebServer) getNotificationSettings(c *gin.Context) {
	enabledVal, _ := database.GetSetting("notifications_enabled")
	enabled := enabledVal == "true"

	c.JSON(http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"enabled": enabled,
	}))
}

func (s *WebServer) updateNotificationSettings(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}

	if err := database.SetSetting("notifications_enabled", fmt.Sprintf("%t", req.Enabled)); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "保存推送设置失败: "+err.Error()))
		return
	}
	notify.SetEnabled(req.Enabled)

	log.Printf("[通知] 推送开关已更新: enabled=%v", req.Enabled)
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

// ===== 智能扫描 =====

func (s *WebServer) previewScan(c *gin.Context) {
	var req struct {
		URL          string `json:"url" binding:"required"`
		Keywords     string `json:"keywords"`
		StrategyType string `json:"strategy_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "URL 无效: "+err.Error()))
		return
	}

	// 解析关键词
	var keywords []string
	for _, kw := range splitKeywords(req.Keywords) {
		if kw != "" {
			keywords = append(keywords, kw)
		}
	}

	result, err := monitor.SmartScan(&monitor.ScanSettings{
		URL:          req.URL,
		Keywords:     keywords,
		StrategyType: req.StrategyType,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "扫描失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, NewSuccessResponse(result))
}

type smartCreateRequest struct {
	Name             string                     `json:"name" binding:"required"`
	URL              string                     `json:"url" binding:"required"`
	ContainerCSS     string                     `json:"container_css"`
	Config           *monitor.ScanMonitorConfig `json:"config"`
	Group            string                     `json:"group"`
	CheckInterval    int                        `json:"check_interval"`
	IsActive         *bool                      `json:"is_active"`
	NotifyFilter     string                     `json:"notify_filter"`
	NotifyKeywords   string                     `json:"notify_keywords"`
	NotifyAccountIDs json.RawMessage            `json:"notify_account_ids"`
}

func dbSiteFromScanRequest(req *smartCreateRequest) (*database.Site, error) {
	config := req.Config
	if config == nil {
		// legacy path - build config from container_css
		return nil, fmt.Errorf("config is required")
	}
	if config.Container == "" {
		return nil, fmt.Errorf("container selector is required")
	}
	if config.Item == "" {
		config.Item = "a"
	}
	hasTitle := false
	for _, f := range config.Fields {
		if f.Name == "title" {
			hasTitle = true
			break
		}
	}
	if !hasTitle {
		config.Fields = append([]monitor.ScanFieldConfig{{Name: "title", Selector: "", Type: "text"}}, config.Fields...)
	}
	group := req.Group
	if group == "" {
		group = "默认"
	}
	site := &database.Site{
		Name:           req.Name,
		URL:            req.URL,
		Container:      config.Container,
		Item:           config.Item,
		GroupName:      group,
		CheckInterval:  req.CheckInterval,
		IsActive:       true,
		NotifyFilter:   req.NotifyFilter,
		NotifyKeywords: req.NotifyKeywords,
		Fields:         monitor.ScanFieldsToSiteFields(config.Fields),
	}
	if site.CheckInterval <= 0 {
		site.CheckInterval = 3600
	}
	if req.IsActive != nil {
		site.IsActive = *req.IsActive
	}
	if site.NotifyFilter == "" {
		site.NotifyFilter = "all"
	}
	if len(req.NotifyAccountIDs) > 0 {
		if err := applyNotifyAccountIDs(site, req.NotifyAccountIDs); err != nil {
			return nil, err
		}
	}
	return site, nil
}

func (s *WebServer) smartCreate(c *gin.Context) {
	var req smartCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}
	if !utf8.ValidString(req.Name) || !utf8.ValidString(req.URL) {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "名称或URL包含无效字符，请检查编码"))
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "URL 无效: "+err.Error()))
		return
	}

	// legacy fallback: use container_css
	if req.Config == nil {
		if req.ContainerCSS != "" {
			_, err := monitor.MonitorFromScan(req.Name, req.URL, req.ContainerCSS)
			if err != nil {
				c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "创建失败: "+err.Error()))
				return
			}
			c.JSON(http.StatusCreated, NewSuccessResponse(map[string]interface{}{"name": req.Name}))
			return
		}
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "config or container_css is required"))
		return
	}

	site, err := dbSiteFromScanRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, err.Error()))
		return
	}

	// 先以非活跃状态创建，启动成功后再标记为活跃
	site.IsActive = false
	if err := database.CreateSiteWithFields(site); err != nil {
		c.JSON(http.StatusConflict, NewErrorResponse(409, "创建站点失败: "+err.Error()))
		return
	}

	shouldBeActive := req.IsActive == nil || *req.IsActive
	if shouldBeActive {
		site.IsActive = true
		if err := monitor.StartLoadedSite(site); err != nil {
			// 启动失败不删除记录，保留为 is_active=false 以便用户手动修复
			log.Printf("[Web] 启动智能创建监控器「%s」失败: %v", site.Name, err)
			c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "创建并启动监控器失败: "+err.Error()))
			return
		}
	} else {
		monitor.ReplaceStoppedSite(site, "")
	}

	log.Printf("[Web] 智能创建监控器: %s", site.Name)
	c.JSON(http.StatusCreated, NewSuccessResponse(map[string]interface{}{
		"id":   site.ID,
		"name": site.Name,
	}))
}

// splitKeywords 分割关键词（支持中英文逗号、空格）
func splitKeywords(s string) []string {
	var result []string
	current := ""
	for _, r := range s {
		if r == ',' || r == '，' || r == '　' || r == ' ' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func (s *WebServer) testScanRule(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "参数错误: "+err.Error()))
		return
	}
	if err := validateOutboundURL(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "URL 无效: "+err.Error()))
		return
	}

	id := c.Param("id")
	var rule database.ScanRuleTemplate
	if err := database.GetDB().Preload("Fields").First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "扫描规则不存在"))
		return
	}

	// 验证 URL 匹配
	if !monitor.ScanRuleMatchesURL(rule, req.URL) {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "测试 URL 不在该规则的适用范围内"))
		return
	}
	if rule.FetchConfig != "" {
		fetchConfig, parseErr := monitor.ParseFetchConfig(rule.FetchConfig, req.URL)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则数据源无效: "+parseErr.Error()))
			return
		}
		if fetchConfig.Mode == monitor.FetchModeAPIJSON {
			site := &database.Site{
				URL: req.URL, Container: rule.Container, Item: rule.Item,
				FetchConfig: rule.FetchConfig,
			}
			for _, field := range rule.Fields {
				site.Fields = append(site.Fields, database.SiteField{
					Name: field.Name, Selector: field.Selector, Type: field.Type,
					Attr: field.Attr, Transform: field.Transform,
				})
			}
			items, extractErr := monitor.ExtractConfiguredSource(c.Request.Context(), site)
			if extractErr != nil {
				c.JSON(http.StatusBadRequest, NewErrorResponse(400, "规则提取失败: "+extractErr.Error()))
				return
			}
			sampleItems := items
			if len(sampleItems) > 10 {
				sampleItems = sampleItems[:10]
			}
			fields := make([]monitor.ScanFieldConfig, 0, len(rule.Fields))
			for _, field := range rule.Fields {
				fieldType := field.Type
				if fieldType == "" {
					fieldType = "text"
				}
				fields = append(fields, monitor.ScanFieldConfig{
					Name: field.Name, Selector: field.Selector, Type: fieldType,
					Attr: field.Attr, Transform: field.Transform,
				})
			}
			c.JSON(http.StatusOK, NewSuccessResponse(&monitor.ScanResult{
				URL: req.URL,
				Containers: []monitor.ContainerInfo{{
					ContainerTag: "JSON", ContainerCSS: rule.Container,
					ItemTag: "OBJECT", ItemCSS: rule.Item, ItemCount: len(items),
					Config:   monitor.ScanMonitorConfig{Container: rule.Container, Item: rule.Item, Fields: fields, Fetch: &fetchConfig},
					Strategy: "rule_test", Confidence: 100,
					Diagnostics: []string{"测试规则匹配成功", fmt.Sprintf("JSON API 提取到 %d 个条目", len(items))},
					SampleItems: sampleItems,
				}},
			}))
			return
		}
	}

	// 使用规则模板的 selector 执行提取
	html, err := fetcher.New().Fetch(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "获取页面失败: "+err.Error()))
		return
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "解析页面失败: "+err.Error()))
		return
	}
	if doc.Find(rule.Container).Length() == 0 {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, fmt.Sprintf("容器选择器 %q 未匹配到元素", rule.Container)))
		return
	}

	fields := make([]monitor.ScanFieldConfig, 0, len(rule.Fields))
	for _, field := range rule.Fields {
		fieldType := field.Type
		if fieldType == "" {
			fieldType = "text"
		}
		fields = append(fields, monitor.ScanFieldConfig{
			Name: field.Name, Selector: field.Selector, Type: fieldType,
			Attr: field.Attr, Transform: field.Transform,
		})
	}
	if len(fields) == 0 {
		fields = append(fields, monitor.ScanFieldConfig{Name: "title", Type: "text"})
	}
	selectors := monitor.ScanConfigToSelectors(monitor.ScanMonitorConfig{
		Container: rule.Container,
		Item:      rule.Item,
		Fields:    fields,
	})
	items, err := monitor.NewExtractor(selectors).Extract(html)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "解析页面失败: "+err.Error()))
		return
	}
	if err := monitor.ResolveExtractedURLs(req.URL, items); err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "解析链接失败: "+err.Error()))
		return
	}

	/*
			// 提取文本并处理可能的空值
			var text string
			if item != nil {
				text = strings.TrimSpace(item.Text())
			}
			if text == "" {
				return
			}
			// 创建 ExtractResult 并确保 entry 不为 nil
			entry := monitor.ExtractResult{"title": text}
			// 尝试从 item 内部的 <a> 标签提取 URL
			firstLink := item.Find("a[href]").First()
			if firstLink != nil && firstLink.Length() > 0 {
				if href, exists := firstLink.Attr("href"); exists {
					entry["url"] = href
				}
			} else if item != nil && item.Is("a") {
				// item 本身就是 <a> 标签，提取自身 href
				if href, exists := item.Attr("href"); exists {
					// 当 item 是 <a> 且没有内部 a[href] 时，由 extractField(attr) 兜底
					entry["url"] = href
				}
			}
			// 如果 firstLink 为空但 item 本身是 <a> 标签，提取自身 href
			// 这是处理 "当前 <a> 自身" 的 href 的关键路径
			if _, ok := entry["url"]; !ok {
				if item != nil && item.Is("a") {
					if href, exists := item.Attr("href"); exists {
						entry["url"] = href
					}
				}
			}
			// 确保 entry 不为 nil 再追加
			if entry != nil {
				items = append(items, entry)
			}
		})
	*/

	// 即使没有匹配到项目，也返回结果（由前端展示"未找到项目"）
	if len(items) == 0 {
		c.JSON(http.StatusOK, NewSuccessResponse(&monitor.ScanResult{
			URL: req.URL,
			Containers: []monitor.ContainerInfo{{
				ContainerCSS: rule.Container,
				ItemCSS:      rule.Item,
				ItemCount:    0,
				Config: monitor.ScanMonitorConfig{
					Container: rule.Container,
					Item:      rule.Item,
				},
				Strategy: "rule_test",
				// 当容器未找到时，置信度设为 0
				Confidence:  0,
				Diagnostics: []string{"测试规则匹配成功", "未找到匹配项目"},
				SampleItems: []monitor.ExtractResult{},
			}},
		}))
		return
	}

	result := &monitor.ScanResult{
		URL: req.URL,
		Containers: []monitor.ContainerInfo{{
			ContainerCSS: rule.Container,
			ItemCSS:      rule.Item,
			ItemCount:    len(items),
			Config: monitor.ScanMonitorConfig{
				Container: rule.Container,
				Item:      rule.Item,
			},
			Strategy:    "rule_test",
			Confidence:  1,
			Diagnostics: []string{"测试规则匹配成功", fmt.Sprintf("容器: %s, 项目: %s", rule.Container, rule.Item)},
			SampleItems: items,
		}},
	}

	c.JSON(http.StatusOK, NewSuccessResponse(result))
}

// ===== 新引擎 API =====

func (s *WebServer) getMonitorEvents(c *gin.Context) {
	name := c.Param("name")
	var site database.Site
	if err := database.GetDB().Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	page := 1
	pageSize := 20
	if rawPage := c.Query("page"); rawPage != "" {
		parsed, err := strconv.Atoi(rawPage)
		if err == nil && parsed > 0 {
			page = parsed
		}
	}
	if rawSize := c.Query("size"); rawSize != "" {
		parsed, err := strconv.Atoi(rawSize)
		if err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	var total int64
	database.GetDB().Model(&database.MonitorEvent{}).Where("site_id = ?", site.ID).Count(&total)

	var events []database.MonitorEvent
	if err := database.GetDB().Where("site_id = ?", site.ID).
		Order("occurred_at desc, id asc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to load events: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"total":  total,
		"page":   page,
		"size":   pageSize,
		"events": events,
	}))
}

func (s *WebServer) getMonitorSnapshots(c *gin.Context) {
	name := c.Param("name")
	var site database.Site
	if err := database.GetDB().Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	var snapshots []database.MonitorSnapshot
	if err := database.GetDB().Where("site_id = ? AND definition_version = ?", site.ID, site.ConfigVersion).Order("last_seen_at desc, id asc").Find(&snapshots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to load snapshots: "+err.Error()))
		return
	}

	result := make([]monitorSnapshotResponse, 0, len(snapshots))
	for _, snapshot := range snapshots {
		priceDisplay := ""
		if snapshot.PriceValid {
			priceDisplay = monitor.FormatPrice(snapshot.PriceMinor, snapshot.Currency)
		}
		result = append(result, monitorSnapshotResponse{
			MonitorSnapshot: snapshot,
			PriceDisplay:    priceDisplay,
			ItemTitle:       snapshotItemTitle(snapshot),
		})
	}
	c.JSON(http.StatusOK, NewSuccessResponse(result))
}

func (s *WebServer) resetBaseline(c *gin.Context) {
	name := c.Param("name")
	var site database.Site
	if err := database.GetDB().Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	var err error
	if runningMonitor := monitor.GetMonitor(name); runningMonitor != nil {
		err = runningMonitor.ResetBaseline(c.Request.Context())
	} else {
		_, err = database.ResetMonitorBaseline(site.ID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "failed to reset baseline: "+err.Error()))
		return
	}

	log.Printf("[Web] 重置基线: %s", name)
	c.JSON(http.StatusOK, NewSuccessResponse(nil))
}

func (s *WebServer) manualCheck(c *gin.Context) {
	name := c.Param("name")
	var site database.Site
	if err := database.GetDB().Preload("Fields").Where("name = ?", name).First(&site).Error; err != nil {
		c.JSON(http.StatusNotFound, NewErrorResponse(404, "monitor not found"))
		return
	}

	runningMonitor := monitor.GetMonitor(name)
	if runningMonitor == nil {
		runningMonitor = monitor.NewDetachedMonitor(&site)
	}
	outcome, err := runningMonitor.CheckNow(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse(500, "check failed: "+err.Error()))
		return
	}
	count := len(outcome.Events)
	if outcome.StrategyType == "presence" {
		count = len(outcome.Updates)
	}

	c.JSON(http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"events":            outcome.Events,
		"updates":           outcome.Updates,
		"count":             count,
		"is_first_baseline": outcome.IsFirstBaseline,
	}))
}

func (s *WebServer) validateMonitorConfig(c *gin.Context) {
	var req addMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid request: "+err.Error()))
		return
	}

	site, err := dbSiteFromRequest(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid monitor config: "+err.Error()))
		return
	}
	if err := monitor.NormalizeAndValidateSiteDefinition(site); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid monitor config: "+err.Error()))
		return
	}
	if err := validateMonitorSourceURL(site); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid monitor source: "+err.Error()))
		return
	}
	engine, err := monitor.NewEngine(site)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "invalid monitor config: "+err.Error()))
		return
	}
	report, err := engine.ValidateExtraction(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(400, "config validation failed: "+err.Error()))
		return
	}
	label := "条目提取"
	if site.StrategyType == "field_transition" {
		label = "商品身份与价格解析"
	}
	c.JSON(http.StatusOK, NewSuccessResponse(map[string]interface{}{
		"valid":           true,
		"status":          "valid",
		"extracted_items": report.ExtractedItems,
		"items": []map[string]interface{}{
			{
				"status":  "ok",
				"label":   label,
				"detail":  fmt.Sprintf("成功提取并验证 %d 条记录", report.ExtractedItems),
				"samples": report.Samples,
			},
		},
		"errors":           []string{},
		"summary":          fmt.Sprintf("配置有效，共提取 %d 条记录；本次验证未写入基线或发送通知。", report.ExtractedItems),
		"strategy_config":  json.RawMessage(site.StrategyConfig),
		"field_data_types": json.RawMessage(site.FieldDataTypes),
	}))
}

// computeDetectionFingerprint 计算检测语义指纹，用于判断配置变化是否需要重建基线
func computeDetectionFingerprint(url, container, item string, fields []fieldRequest, strategyType, strategyConfig, fieldDataTypes string) string {
	return computeDetectionFingerprintWithFetch(url, container, item, fields, strategyType, strategyConfig, fieldDataTypes, "")
}

func computeDetectionFingerprintWithFetch(url, container, item string, fields []fieldRequest, strategyType, strategyConfig, fieldDataTypes, fetchConfig string) string {
	type canonicalField struct {
		Name      string `json:"name"`
		Selector  string `json:"selector"`
		Type      string `json:"type"`
		Attr      string `json:"attr"`
		Transform string `json:"transform"`
	}
	canonicalFields := make([]canonicalField, 0, len(fields))
	for _, field := range fields {
		fieldType := field.Type
		if fieldType == "" {
			fieldType = "text"
		}
		canonicalFields = append(canonicalFields, canonicalField{
			Name: field.Name, Selector: field.Selector, Type: fieldType, Attr: field.Attr, Transform: field.Transform,
		})
	}
	sort.Slice(canonicalFields, func(i, j int) bool {
		left, _ := json.Marshal(canonicalFields[i])
		right, _ := json.Marshal(canonicalFields[j])
		return string(left) < string(right)
	})

	canonicalStrategy := canonicalJSONString(strategyConfig)
	if rule, err := monitor.ParseDetectionRule(strategyConfig); err == nil {
		if strategyType == "" {
			strategyType = "presence"
		}
		if rule.Type == strategyType {
			if data, marshalErr := json.Marshal(rule); marshalErr == nil {
				canonicalStrategy = string(data)
			}
		}
	}
	definition := struct {
		URL            string           `json:"url"`
		Container      string           `json:"container"`
		Item           string           `json:"item"`
		StrategyType   string           `json:"strategy_type"`
		StrategyConfig string           `json:"strategy_config"`
		FieldDataTypes string           `json:"field_data_types"`
		FetchConfig    string           `json:"fetch_config"`
		Fields         []canonicalField `json:"fields"`
	}{
		URL: strings.TrimSpace(url), Container: container, Item: item,
		StrategyType: strategyType, StrategyConfig: canonicalStrategy,
		FieldDataTypes: canonicalJSONString(fieldDataTypes), Fields: canonicalFields,
		FetchConfig: canonicalJSONString(fetchConfig),
	}
	data, _ := json.Marshal(definition)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func canonicalJSONString(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "{}"
	}
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return strings.TrimSpace(raw)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return string(data)
}
