package web

import (
	"net/http"
	"strings"

	"github.com/cn-maul/Gentry/database"
	"github.com/gin-gonic/gin"
)

const defaultCategoryName = "默认"

// categoryResponse 分类响应体
type categoryResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type categoryRequest struct {
	Name string `json:"name" binding:"required"`
}

// syncCategoriesFromSites 把监控器正在使用的分组补录为分类，兼容旧数据。
func syncCategoriesFromSites() {
	db := database.GetDB()
	var groups []string
	if err := db.Model(&database.Site{}).Distinct("group_name").Pluck("group_name", &groups).Error; err != nil {
		return
	}
	seen := map[string]bool{}
	var cats []database.Category
	if err := db.Find(&cats).Error; err == nil {
		for _, cat := range cats {
			seen[cat.Name] = true
		}
	}
	for _, group := range groups {
		name := strings.TrimSpace(group)
		if name == "" || seen[name] {
			continue
		}
		if err := db.Create(&database.Category{Name: name}).Error; err == nil {
			seen[name] = true
		}
	}
}

// listCategories GET /api/v1/settings/categories
func listCategories(c *gin.Context) {
	syncCategoriesFromSites()
	var cats []database.Category
	if err := database.GetDB().Order("id asc").Find(&cats).Error; err != nil {
		fail(c, http.StatusInternalServerError, "加载分类失败: "+err.Error())
		return
	}
	out := make([]categoryResponse, 0, len(cats))
	for _, cat := range cats {
		out = append(out, categoryResponse{ID: cat.ID, Name: cat.Name})
	}
	ok(c, out)
}

// createCategory POST /api/v1/settings/categories
func createCategory(c *gin.Context) {
	var req categoryRequest
	if !bindJSON(c, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		fail(c, http.StatusBadRequest, "分类名称不能为空")
		return
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(&database.Category{}).Where("name = ?", name).Count(&count).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询分类失败: "+err.Error())
		return
	}
	if count > 0 {
		fail(c, http.StatusConflict, "分类「"+name+"」已存在")
		return
	}
	cat := database.Category{Name: name}
	if err := db.Create(&cat).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建分类失败: "+err.Error())
		return
	}
	ok(c, categoryResponse{ID: cat.ID, Name: cat.Name})
}

// renameCategory PUT /api/v1/settings/categories/:id
func renameCategory(c *gin.Context) {
	id := parseUintParam(c.Param("id"))
	var req categoryRequest
	if !bindJSON(c, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		fail(c, http.StatusBadRequest, "分类名称不能为空")
		return
	}
	db := database.GetDB()
	var cat database.Category
	if err := db.First(&cat, id).Error; err != nil {
		fail(c, http.StatusNotFound, "分类不存在")
		return
	}
	if cat.Name == name {
		ok(c, categoryResponse{ID: cat.ID, Name: cat.Name})
		return
	}
	var count int64
	if err := db.Model(&database.Category{}).Where("name = ? AND id <> ?", name, id).Count(&count).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询分类失败: "+err.Error())
		return
	}
	if count > 0 {
		fail(c, http.StatusConflict, "分类「"+name+"」已存在")
		return
	}
	if err := db.Model(&database.Category{}).Where("id = ?", id).Update("name", name).Error; err != nil {
		fail(c, http.StatusInternalServerError, "重命名失败: "+err.Error())
		return
	}
	// 同步更新该分类下所有监控器的分组名
	if err := db.Model(&database.Site{}).Where("group_name = ?", cat.Name).Update("group_name", name).Error; err != nil {
		fail(c, http.StatusInternalServerError, "同步监控器分组失败: "+err.Error())
		return
	}
	ok(c, categoryResponse{ID: cat.ID, Name: name})
}

// deleteCategory DELETE /api/v1/settings/categories/:id
func deleteCategory(c *gin.Context) {
	id := parseUintParam(c.Param("id"))
	db := database.GetDB()
	var cat database.Category
	if err := db.First(&cat, id).Error; err != nil {
		fail(c, http.StatusNotFound, "分类不存在")
		return
	}
	if cat.Name == defaultCategoryName {
		fail(c, http.StatusBadRequest, "「默认」分类不可删除")
		return
	}
	// 该分类下的监控器迁移回「默认」
	res := db.Model(&database.Site{}).Where("group_name = ?", cat.Name).Update("group_name", defaultCategoryName)
	if res.Error != nil {
		fail(c, http.StatusInternalServerError, "迁移监控器分类失败: "+res.Error.Error())
		return
	}
	if err := db.Delete(&database.Category{ID: cat.ID}).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除分类失败: "+err.Error())
		return
	}
	ok(c, gin.H{"moved": res.RowsAffected})
}