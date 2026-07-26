package controller

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
	"llm-test-server/internal/service"
)

// ──────────────────────────── 结构体 ────────────────────────────

// ModelConfigController 模型配置 HTTP 处理器
type ModelConfigController struct {
	svc *service.ModelConfigService
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewModelConfigController 创建模型配置控制器实例
func NewModelConfigController(svc *service.ModelConfigService) *ModelConfigController {
	return &ModelConfigController{svc: svc}
}

// Create 创建模型配置
func (ctrl *ModelConfigController) Create(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	var req model.CreateModelConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.Create(c.Request.Context(), &req); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// List 获取模型配置列表
func (ctrl *ModelConfigController) List(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	// 按 ID 精确查询
	if id := c.Query("Id"); id != "" {
		mc, err := ctrl.svc.GetByID(c.Request.Context(), id)
		if err != nil {
			handleError(c, err)
			return
		}
		common.OK(c, common.PageData{
			Total:    1,
			Page:     1,
			PageSize: 1,
			Items:    []model.ModelConfig{*mc},
		})
		return
	}

	// 分页查询
	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	items, total, err := ctrl.svc.List(c.Request.Context(), page, pageSize)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, common.PageData{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	})
}

// Update 更新模型配置
func (ctrl *ModelConfigController) Update(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.UpdateModelConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.Update(c.Request.Context(), id, &req); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// Delete 删除模型配置
func (ctrl *ModelConfigController) Delete(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	if err := ctrl.svc.Delete(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// Test 测试模型连通性
func (ctrl *ModelConfigController) Test(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	result, err := ctrl.svc.TestConnectivity(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, result)
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// handleError 统一错误处理
func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*service.AppError); ok {
		httpStatus := http.StatusBadRequest
		if appErr.Code >= 50000 {
			httpStatus = http.StatusInternalServerError
		}
		slog.Error("请求处理失败", "errorCode", appErr.Code, "errorMsg", appErr.Msg, "path", c.Request.URL.Path)
		common.Fail(c, httpStatus, appErr.Code, appErr.Msg)
		return
	}
	slog.Error("请求处理未知错误", "error", err.Error(), "path", c.Request.URL.Path)
	common.Fail(c, http.StatusInternalServerError, common.ErrModelCallFailed, err.Error())
}
