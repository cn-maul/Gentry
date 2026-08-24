package web

import (
	"net/http"
	"time"

	"github.com/cn-maul/Gentry/database"
	"github.com/cn-maul/Gentry/monitor"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func healthCheck(c *gin.Context) {
	sqlDB, err := database.GetDB().DB()
	dbOk := err == nil && sqlDB.Ping() == nil

	ok(c, map[string]interface{}{
		"status":   "ok",
		"database": dbOk,
		"monitors": len(monitor.GetAllMonitors()),
	})
}

func getStats(c *gin.Context) {
	db := database.GetDB()

	var totalMonitors int64
	db.Model(&database.Site{}).Count(&totalMonitors)

	runningMonitors := 0
	for _, status := range monitor.GetAllMonitors() {
		if status.IsRunning {
			runningMonitors++
		}
	}

	updates := func() *gorm.DB { return db.Model(&database.UpdateRecord{}) }
	var totalUpdates int64
	updates().Count(&totalUpdates)

	var updatesLastHour int64
	updates().Where("created_at >= ?", time.Now().Add(-1*time.Hour)).Count(&updatesLastHour)

	var unnotifiedUpdates int64
	updates().Where("notified = ?", false).Count(&unnotifiedUpdates)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var pushedToday int64
	updates().Where("notified = ? AND notified_at >= ?", true, todayStart).Count(&pushedToday)

	var totalAccounts int64
	db.Model(&database.NotificationAccount{}).Count(&totalAccounts)

	ok(c, map[string]interface{}{
		"total_monitors":     totalMonitors,
		"running_monitors":   runningMonitors,
		"total_updates":      totalUpdates,
		"updates_last_hour":  updatesLastHour,
		"unnotified_updates": unnotifiedUpdates,
		"pushed_today":       pushedToday,
		"total_accounts":     totalAccounts,
	})
}

func listGroups(c *gin.Context) {
	var groups []string
	if err := database.GetDB().Model(&database.Site{}).
		Distinct("group_name").
		Order("group_name asc").
		Pluck("group_name", &groups).Error; err != nil {
		fail(c, http.StatusInternalServerError, "failed to load groups: "+err.Error())
		return
	}
	if len(groups) == 0 {
		groups = []string{defaultGroupName}
	}
	ok(c, groups)
}
