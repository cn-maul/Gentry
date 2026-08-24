package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewSuccessResponse 构造标准成功响应体。
func NewSuccessResponse(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

// NewErrorResponse 构造标准错误响应体。
func NewErrorResponse(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// ok 输出 200 成功响应。
func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, NewSuccessResponse(data))
}

// created 输出 201 创建成功响应。
func created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, NewSuccessResponse(data))
}

// fail 输出错误响应；业务码与 HTTP 状态码保持一致。
func fail(c *gin.Context, status int, message string) {
	c.JSON(status, NewErrorResponse(status, message))
}

// bindJSON 解析请求体；失败时写出 400 响应并返回 false。
func bindJSON(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return false
	}
	return true
}
