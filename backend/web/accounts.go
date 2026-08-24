package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/notify"
	"github.com/gin-gonic/gin"
)

// sensitiveKeysByService 各推送服务中需要脱敏/回填的敏感字段。
var sensitiveKeysByService = map[string][]string{
	"pushplus":  {"token"},
	"serverchan": {"sendkey"},
	"webhook":   {"url"},
	"bark":      {"key"},
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
	out := make([]accountResponse, 0, len(accounts))
	for _, account := range accounts {
		out = append(out, accountFromModel(account))
	}
	return out
}

func listAccounts(c *gin.Context) {
	var accounts []database.NotificationAccount
	if err := database.GetDB().Order("created_at desc").Find(&accounts).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to load notification accounts: "+err.Error())
		return
	}
	ok(c, accountsFromModels(accounts))
}

func createAccount(c *gin.Context) {
	var req accountRequest
	if !bindJSON(c, &req) {
		return
	}
	if !utf8.ValidString(req.Name) {
		fail(c, http.StatusBadRequest, "账户名称包含无效字符，请检查编码")
		return
	}
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		fail(c, http.StatusBadRequest, "配置参数无效: "+err.Error())
		return
	}
	account := &database.NotificationAccount{Name: req.Name, Service: req.Service, ConfigJSON: string(configJSON)}
	if err := validateAccountConfig(&req); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := database.GetDB().Create(account).Error; err != nil {
		fail(c, http.StatusConflict, "创建账户失败: "+err.Error())
		return
	}

	log.Printf("[通知] 创建推送账户: %s (%s)", account.Name, account.Service)
	created(c, accountFromModel(*account))
}

func updateAccount(c *gin.Context) {
	var req accountRequest
	if !bindJSON(c, &req) {
		return
	}
	if !utf8.ValidString(req.Name) {
		fail(c, http.StatusBadRequest, "账户名称包含无效字符，请检查编码")
		return
	}

	var account database.NotificationAccount
	if err := database.GetDB().First(&account, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, "账户不存在")
		return
	}

	mergedConfig, err := mergeMaskedSensitiveConfig(req.Service, req.Config, account.ConfigJSON)
	if err != nil {
		fail(c, http.StatusBadRequest, "配置参数无效: "+err.Error())
		return
	}
	configJSON, marshalErr := json.Marshal(mergedConfig)
	if marshalErr != nil {
		fail(c, http.StatusBadRequest, "配置参数无效: "+marshalErr.Error())
		return
	}
	if err := validateAccountConfig(&req); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	account.Name = req.Name
	account.Service = req.Service
	account.ConfigJSON = string(configJSON)
	if err := database.GetDB().Save(&account).Error; err != nil {
		log.Printf("[通知] 更新推送账户失败「%s」: %v", account.Name, err)
		fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	log.Printf("[通知] 更新推送账户: %s", account.Name)
	ok(c, accountFromModel(account))
}

// validateAccountConfig 校验账户配置合法性与出网 URL 安全。
func validateAccountConfig(req *accountRequest) error {
	if err := notify.ValidateAccountConfig(req.Service, req.Config); err != nil {
		return fmt.Errorf("账户配置无效: %w", err)
	}
	if req.Service == "webhook" {
		if webhookURL, _ := req.Config["url"].(string); webhookURL != "" {
			if err := validateOutboundURL(webhookURL); err != nil {
				return fmt.Errorf("Webhook URL 无效: %w", err)
			}
		}
	}
	return nil
}

// mergeMaskedSensitiveConfig 将更新请求中仍是脱敏占位的敏感字段回填为已存储的原值。
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
	for _, key := range sensitiveKeysByService[service] {
		incomingValue, incomingOK := merged[key].(string)
		existingValue, existingOK := existing[key].(string)
		if incomingOK && existingOK && incomingValue == maskSecret(existingValue) {
			merged[key] = existingValue
		}
	}
	return merged, nil
}

func deleteAccount(c *gin.Context) {
	id := c.Param("id")

	// 账户仍被监控器引用时拒绝删除。
	var sites []database.Site
	if err := database.GetDB().Find(&sites).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to check account references: "+err.Error())
		return
	}
	for _, site := range sites {
		for _, accountID := range site.GetNotifyAccountIDs() {
			if fmt.Sprintf("%d", accountID) == id {
				fail(c, http.StatusConflict, "该账户仍被监控器引用，无法删除")
				return
			}
		}
	}

	result := database.GetDB().Delete(&database.NotificationAccount{}, id)
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, "删除账户失败: "+result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, "账户不存在")
		return
	}
	ok(c, nil)
}

func listNotificationProviders(c *gin.Context) {
	ok(c, notify.ListProviderMetadata())
}

// ===== 推送全局开关 =====

func getNotificationSettings(c *gin.Context) {
	enabledVal, _ := database.GetSetting("notifications_enabled")
	ok(c, map[string]interface{}{"enabled": enabledVal == "true"})
}

func updateNotificationSettings(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := database.SetSetting("notifications_enabled", strconv.FormatBool(req.Enabled)); err != nil {
		fail(c, http.StatusInternalServerError, "保存推送设置失败: "+err.Error())
		return
	}
	notify.SetEnabled(req.Enabled)

	log.Printf("[通知] 推送开关已更新: enabled=%v", req.Enabled)
	ok(c, nil)
}
