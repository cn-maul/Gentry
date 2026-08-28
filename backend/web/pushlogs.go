package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cn-maul/Gentry/database"
	"github.com/gin-gonic/gin"
)

// pushLogResponse 推送记录响应结构（数组字段解析为 JSON 数组返回给前端时间轴）。
type pushLogResponse struct {
	ID           uint      `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	SiteID       uint      `json:"site_id"`
	SiteName     string    `json:"site_name"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason"`
	AccountNames []string  `json:"account_names"`
	ItemCount    int       `json:"item_count"`
	Titles       []string  `json:"titles"`
	Detail       string    `json:"detail"`
}

func parseStringList(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []string{}
	}
	if list == nil {
		return []string{}
	}
	return list
}

func pushLogFromModel(l database.PushLog) pushLogResponse {
	return pushLogResponse{
		ID:           l.ID,
		CreatedAt:    l.CreatedAt,
		SiteID:       l.SiteID,
		SiteName:     l.SiteName,
		Status:       l.Status,
		Reason:       l.Reason,
		AccountNames: parseStringList(l.AccountNames),
		ItemCount:    l.ItemCount,
		Titles:       parseStringList(l.Titles),
		Detail:       l.Detail,
	}
}

// listPushLogs 分页返回推送记录，支持 status / site_id 过滤。
func listPushLogs(c *gin.Context) {
	page, pageSize, okParams := parsePagination(c)
	if !okParams {
		return
	}

	db := database.GetDB().Model(&database.PushLog{})
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		db = db.Where("status = ?", status)
	}
	if siteIDRaw := c.Query("site_id"); siteIDRaw != "" {
		siteID, err := strconv.Atoi(siteIDRaw)
		if err != nil || siteID < 1 {
			fail(c, http.StatusBadRequest, "site_id 无效")
			return
		}
		db = db.Where("site_id = ?", siteID)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to count push logs: "+err.Error())
		return
	}

	var logs []database.PushLog
	if err := db.Order("created_at desc, id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to load push logs: "+err.Error())
		return
	}

	records := make([]pushLogResponse, 0, len(logs))
	for _, l := range logs {
		records = append(records, pushLogFromModel(l))
	}

	ok(c, map[string]interface{}{
		"total":   total,
		"page":    page,
		"size":    pageSize,
		"records": records,
	})
}
