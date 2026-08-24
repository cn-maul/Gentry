package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/monitor"
	"github.com/gin-gonic/gin"
)

const defaultGroupName = "默认"

// findSiteByName 按名称加载站点（含字段），未找到返回 false。
func findSiteByName(name string) (*database.Site, bool) {
	var site database.Site
	if err := database.GetDB().Preload("Fields").Where("name = ?", name).First(&site).Error; err != nil {
		return nil, false
	}
	return &site, true
}

func listMonitors(c *gin.Context) {
	ok(c, monitor.GetAllMonitors())
}

func getMonitor(c *gin.Context) {
	m := monitor.GetMonitor(c.Param("name"))
	if m == nil {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}
	ok(c, m.GetStatus())
}

func addMonitor(c *gin.Context) {
	var req addMonitorRequest
	if !bindJSON(c, &req) {
		return
	}
	if !utf8.ValidString(req.Name) || !utf8.ValidString(req.URL) {
		fail(c, http.StatusBadRequest, "名称或URL包含无效字符，请检查编码")
		return
	}

	site, err := dbSiteFromRequest(&req)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid notify_account_ids: "+err.Error())
		return
	}
	if err := validateMonitorDefinition(site); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := database.CreateSiteWithFields(site); err != nil {
		fail(c, http.StatusConflict, "monitor already exists: "+err.Error())
		return
	}

	if site.IsActive {
		if err := monitor.StartLoadedSite(site); err != nil {
			log.Printf("[Web] 启动新增监控器「%s」失败: %v", site.Name, err)
			// 回滚：删除已创建的记录，避免留下半创建状态
			if dbErr := database.DeleteSiteCascade(site.ID); dbErr != nil {
				log.Printf("[Web] 回滚新增监控器「%s」数据库记录失败: %v", site.Name, dbErr)
			} else {
				monitor.UnregisterMonitor(site.Name)
			}
			fail(c, http.StatusInternalServerError, "创建并启动监控器失败: "+err.Error())
			return
		}
	} else {
		monitor.RegisterStoppedSite(site)
	}

	log.Printf("[Web] 新增监控器: %s", site.Name)
	created(c, map[string]interface{}{"id": site.ID, "name": site.Name})
}

// validateMonitorDefinition 校验监控器定义与数据源出网安全。
func validateMonitorDefinition(site *database.Site) error {
	if err := monitor.NormalizeAndValidateSiteDefinition(site); err != nil {
		return fmt.Errorf("invalid monitor config: %w", err)
	}
	if err := validateMonitorSourceURL(site); err != nil {
		return fmt.Errorf("invalid monitor source: %w", err)
	}
	return nil
}

func removeMonitor(c *gin.Context) {
	name := c.Param("name")

	// 如果正在运行则先停止
	if monitor.Exists(name) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := monitor.QuiesceMonitor(name, ctx)
		cancel()
		if err != nil {
			fail(c, http.StatusConflict, "monitor is busy: "+err.Error())
			return
		}
	}
	monitor.UnregisterMonitor(name)

	site, found := findSiteByName(name)
	if !found {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}
	if err := database.DeleteSiteCascade(site.ID); err != nil {
		log.Printf("[Web] 删除监控器「%s」失败: %v", name, err)
		fail(c, http.StatusInternalServerError, "delete failed: "+err.Error())
		return
	}

	log.Printf("[Web] 删除监控器: %s", name)
	ok(c, nil)
}

func startMonitor(c *gin.Context) {
	name := c.Param("name")
	if m := monitor.GetMonitor(name); m != nil && m.GetStatus().IsRunning {
		ok(c, nil) // 幂等：已在运行
		return
	}
	if err := monitor.StartSite(name); err != nil {
		fail(c, http.StatusInternalServerError, "启动失败: "+err.Error())
		return
	}
	log.Printf("[Web] 启动监控器: %s", name)
	ok(c, nil)
}

func stopMonitor(c *gin.Context) {
	name := c.Param("name")
	m := monitor.GetMonitor(name)
	if m == nil {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}
	if !m.GetStatus().IsRunning {
		ok(c, nil) // 幂等：已停止
		return
	}
	if err := monitor.StopSite(name); err != nil {
		fail(c, http.StatusInternalServerError, "停止失败: "+err.Error())
		return
	}
	log.Printf("[Web] 停止监控器: %s", name)
	ok(c, nil)
}

func updateMonitor(c *gin.Context) {
	oldName := c.Param("name")

	var req addMonitorRequest
	if !bindJSON(c, &req) {
		return
	}

	original, found := findSiteByName(oldName)
	if !found {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}

	oldFingerprint := computeDetectionFingerprint(
		original.URL, original.Container, original.Item,
		fieldRequestsFromSiteFields(original.Fields), original.FetchConfig)

	// 在原记录副本上构建候选定义，避免污染回滚用的原始数据。
	candidate := *original
	candidate.Name = req.Name
	candidate.URL = req.URL
	candidate.Container = req.Container
	candidate.Item = req.Item
	if req.Group != "" {
		candidate.GroupName = req.Group
	} else {
		candidate.GroupName = defaultGroupName
	}
	candidate.CheckInterval = req.CheckInterval
	candidate.IsActive = req.IsActive
	candidate.NotifyFilter = req.NotifyFilter
	candidate.NotifyKeywords = req.NotifyKeywords
	candidate.FetchConfig = rawJSON(req.FetchConfig)
	candidate.Fields = siteFieldsFromFieldRequests(req.Fields)
	if err := applyNotifyAccountIDs(&candidate, req.NotifyAccountIDs); err != nil {
		fail(c, http.StatusBadRequest, "invalid notify_account_ids: "+err.Error())
		return
	}
	if err := validateMonitorDefinition(&candidate); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	newFingerprint := computeDetectionFingerprint(
		candidate.URL, candidate.Container, candidate.Item,
		fieldRequestsFromSiteFields(candidate.Fields), candidate.FetchConfig)
	needsBaseline := oldFingerprint != newFingerprint
	if needsBaseline {
		candidate.ConfigVersion++
		candidate.BaselineStatus = "needs_baseline"
	}

	restore := func(reason string) {
		if _, err := monitor.AtomicReplaceMonitor(original, oldName); err != nil {
			log.Printf("[Web] 恢复旧监控器「%s」失败（%s）: %v", oldName, reason, err)
		}
	}

	// 先停止旧实例并等待在途检查退出，防止旧定义在事务后写回快照。
	quiesceCtx, quiesceCancel := context.WithTimeout(context.Background(), 15*time.Second)
	quiesceErr := monitor.QuiesceMonitor(oldName, quiesceCtx)
	quiesceCancel()
	if quiesceErr != nil {
		restore("中止更新")
		fail(c, http.StatusConflict, "monitor is busy: "+quiesceErr.Error())
		return
	}

	if err := database.UpdateSiteWithFields(&candidate, candidate.Fields); err != nil {
		restore("更新失败")
		log.Printf("[Web] 更新监控器「%s」失败: %v", oldName, err)
		fail(c, http.StatusInternalServerError, "update failed: "+err.Error())
		return
	}

	// 重新加载更新后的站点并原子替换内存实例。
	var updatedSite database.Site
	if err := database.GetDB().Preload("Fields").First(&updatedSite, candidate.ID).Error; err != nil {
		log.Printf("[Web] 重新加载更新后的站点失败: %v", err)
		fail(c, http.StatusInternalServerError, "reload updated monitor failed: "+err.Error())
		return
	}
	updatedSite.IsActive = req.IsActive
	if _, err := monitor.AtomicReplaceMonitor(&updatedSite, oldName); err != nil {
		log.Printf("[Web] 替换监控器「%s」失败: %v", updatedSite.Name, err)
		fail(c, http.StatusInternalServerError, "replace failed: "+err.Error())
		return
	}

	newName := req.Name
	if newName == "" {
		newName = oldName
	}
	log.Printf("[Web] 更新监控器: %s -> %s", oldName, newName)
	ok(c, nil)
}

func getMonitorConfig(c *gin.Context) {
	site, found := findSiteByName(c.Param("name"))
	if !found {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}
	ok(c, monitorConfigResponse{
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
		Fields:           fieldRequestsFromSiteFields(site.Fields),
		FetchConfig:      jsonRawOrNull(site.FetchConfig),
		BaselineStatus:   site.BaselineStatus,
	})
}

func manualCheck(c *gin.Context) {
	name := c.Param("name")
	site, found := findSiteByName(name)
	if !found {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}

	m := monitor.GetMonitor(name)
	if m == nil {
		m = monitor.NewDetachedMonitor(site)
	}
	outcome, err := m.CheckNow(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "check failed: "+err.Error())
		return
	}

	ok(c, map[string]interface{}{
		"updates":           outcome.Updates,
		"count":             len(outcome.Updates),
		"is_first_baseline": outcome.IsFirstBaseline,
	})
}

func validateMonitorConfig(c *gin.Context) {
	var req addMonitorRequest
	if !bindJSON(c, &req) {
		return
	}

	site, err := dbSiteFromRequest(&req)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid monitor config: "+err.Error())
		return
	}
	if err := validateMonitorDefinition(site); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	report, err := monitor.ValidateExtraction(c.Request.Context(), site)
	if err != nil {
		fail(c, http.StatusBadRequest, "config validation failed: "+err.Error())
		return
	}
	ok(c, map[string]interface{}{
		"valid":           true,
		"status":          "valid",
		"extracted_items": report.ExtractedItems,
		"items": []map[string]interface{}{
			{
				"status":  "ok",
				"label":   "条目提取",
				"detail":  fmt.Sprintf("成功提取并验证 %d 条记录", report.ExtractedItems),
				"samples": report.Samples,
			},
		},
		"errors":  []string{},
		"summary": fmt.Sprintf("配置有效，共提取 %d 条记录；本次验证未写入基线或发送通知。", report.ExtractedItems),
	})
}

func updateNotifyAccounts(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		AccountIDs json.RawMessage `json:"notify_account_ids"`
	}
	if !bindJSON(c, &req) {
		return
	}

	site, found := findSiteByName(name)
	if !found {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}
	if err := applyNotifyAccountIDs(site, req.AccountIDs); err != nil {
		fail(c, http.StatusBadRequest, "invalid notify_account_ids: "+err.Error())
		return
	}
	if err := database.GetDB().Save(site).Error; err != nil {
		log.Printf("[Web] 更新推送账户失败「%s」: %v", name, err)
		fail(c, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	// 同步更新运行中的实例，避免下一检查周期仍用旧配置。
	if m := monitor.GetMonitor(name); m != nil {
		m.UpdateSiteNotifyAccounts(site.NotifyAccountIDs)
	}

	log.Printf("[Web] 更新 %s 的推送账户: %s", name, site.NotifyAccountIDs)
	ok(c, nil)
}
