package web

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/monitor"
	"github.com/gin-gonic/gin"
)

// parsePagination 解析 page/size 查询参数，返回 (page, size, ok)。
func parsePagination(c *gin.Context) (int, int, bool) {
	page, size := 1, 20
	if raw := c.Query("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			fail(c, http.StatusBadRequest, "page must be a positive integer")
			return 0, 0, false
		}
		page = parsed
	}
	if raw := c.Query("size"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			fail(c, http.StatusBadRequest, "size must be between 1 and 100")
			return 0, 0, false
		}
		size = parsed
	}
	return page, size, true
}

func getUpdates(c *gin.Context) {
	site, found := findSiteByName(c.Param("name"))
	if !found {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}
	page, pageSize, okParams := parsePagination(c)
	if !okParams {
		return
	}

	db := database.GetDB()
	var total int64
	if err := db.Model(&database.UpdateRecord{}).Where("site_id = ?", site.ID).Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to count updates: "+err.Error())
		return
	}

	var records []database.UpdateRecord
	if err := db.Where("site_id = ?", site.ID).
		Order("created_at desc, id asc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to load updates: "+err.Error())
		return
	}

	ok(c, map[string]interface{}{
		"total":   total,
		"page":    page,
		"size":    pageSize,
		"records": records,
	})
}

func markAllNotified(c *gin.Context) {
	name := c.Param("name")
	site, found := findSiteByName(name)
	if !found {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}

	result := database.GetDB().Model(&database.UpdateRecord{}).
		Where("site_id = ? AND notified = ?", site.ID, false).
		Updates(map[string]interface{}{"notified": true, "notified_at": time.Now()})
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, "failed to mark updates as notified: "+result.Error.Error())
		return
	}

	log.Printf("[Web] 标记 %s 的 %d 条记录为已推送", name, result.RowsAffected)
	ok(c, map[string]interface{}{"updated": result.RowsAffected})
}

func markRead(c *gin.Context) {
	name := c.Param("name")
	site, found := findSiteByName(name)
	if !found {
		fail(c, http.StatusNotFound, "monitor not found")
		return
	}
	if err := database.GetDB().Model(&database.UpdateRecord{}).
		Where("site_id = ? AND is_read = ?", site.ID, false).
		Update("is_read", true).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to mark updates as read: "+err.Error())
		return
	}
	monitor.MarkRead(name)
	ok(c, nil)
}
