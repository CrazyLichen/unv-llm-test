package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ──────────────────────────── 结构体 ────────────────────────────

// Response 统一响应结构
type Response struct {
	// ErrorCode 错误码
	ErrorCode int `json:"ErrorCode"`
	// ErrorMsg 错误描述
	ErrorMsg string `json:"ErrorMsg"`
	// Data 业务数据
	Data interface{} `json:"Data"`
}

// PageData 分页响应结构
type PageData struct {
	// Total 总记录数
	Total int `json:"Total"`
	// Page 当前页码
	Page int `json:"Page"`
	// PageSize 每页数量
	PageSize int `json:"PageSize"`
	// Items 数据列表
	Items interface{} `json:"Items"`
}

// ──────────────────────────── 导出函数 ────────────────────────────

// OK 返回成功响应，Data 可为 nil
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		ErrorCode: Success,
		ErrorMsg:  "",
		Data:      data,
	})
}

// Fail 返回失败响应
func Fail(c *gin.Context, httpStatus int, errorCode int, errorMsg string) {
	c.JSON(httpStatus, Response{
		ErrorCode: errorCode,
		ErrorMsg:  errorMsg,
		Data:      nil,
	})
}

// MaskApiKey 对 API Key 进行脱敏，保留前3后4位，中间用 **** 替代
func MaskApiKey(key string) string {
	if len(key) <= 7 {
		return "****"
	}
	return key[:3] + "****" + key[len(key)-4:]
}
