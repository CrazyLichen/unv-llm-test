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

// TaskController 任务 HTTP 处理器
type TaskController struct {
	svc *service.TaskService
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewTaskController 创建任务控制器实例
func NewTaskController(svc *service.TaskService) *TaskController {
	return &TaskController{svc: svc}
}

// Create 创建任务
func (ctrl *TaskController) Create(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	var req model.CreateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.Create(c.Request.Context(), &req); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// List 获取任务列表
func (ctrl *TaskController) List(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	// 按 ID 精确查询
	if id := c.Query("Id"); id != "" {
		item, err := ctrl.svc.GetByID(c.Request.Context(), id)
		if err != nil {
			handleError(c, err)
			return
		}
		common.OK(c, common.PageData{
			Total:    1,
			Page:     1,
			PageSize: 1,
			Items:    []model.TaskItem{*item},
		})
		return
	}

	// 分页查询
	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "20"))
	status := c.Query("Status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	items, total, err := ctrl.svc.List(c.Request.Context(), page, pageSize, status)
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

// Delete 删除任务
func (ctrl *TaskController) Delete(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	if err := ctrl.svc.Delete(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// Update 更新任务状态（暂停/恢复）
func (ctrl *TaskController) Update(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.UpdateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.Update(c.Request.Context(), id, &req); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// ListImages 获取任务分析结果列表
func (ctrl *TaskController) ListImages(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	// 按 ImageId 精确查询
	if imageId := c.Query("ImageId"); imageId != "" {
		item, total, err := ctrl.svc.ListImages(c.Request.Context(), id, imageId, 1, 1, "", "")
		if err != nil {
			handleError(c, err)
			return
		}
		common.OK(c, common.PageData{
			Total:    total,
			Page:     1,
			PageSize: 1,
			Items:    item,
		})
		return
	}

	// 分页查询
	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "24"))
	status := c.Query("Status")
	correction := c.Query("Correction")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}

	items, total, err := ctrl.svc.ListImages(c.Request.Context(), id, "", page, pageSize, status, correction)
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

// UploadImage 补录漏报照片
func (ctrl *TaskController) UploadImage(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	file, err := c.FormFile("file")
	if err != nil {
		slog.Warn("读取上传文件失败", "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: 未选择文件")
		return
	}

	if err := ctrl.svc.UploadMissedPhoto(c.Request.Context(), id, file); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// UpdateCorrection 标记素材误报/恢复
func (ctrl *TaskController) UpdateCorrection(c *gin.Context) {
	id := c.Param("id")
	imageId := c.Param("imageId")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id, "imageId", imageId)

	var req model.UpdateCorrectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.MarkCorrection(c.Request.Context(), id, imageId, req.Correction); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}
