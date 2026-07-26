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

// MaterialLibraryController 素材库 HTTP 处理器
type MaterialLibraryController struct {
	svc *service.MaterialLibraryService
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewMaterialLibraryController 创建素材库控制器实例
func NewMaterialLibraryController(svc *service.MaterialLibraryService) *MaterialLibraryController {
	return &MaterialLibraryController{svc: svc}
}

// Create 创建素材库
func (ctrl *MaterialLibraryController) Create(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	var req model.CreateMaterialLibraryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: "+err.Error())
		return
	}

	result, err := ctrl.svc.Create(c.Request.Context(), &req)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, result)
}

// List 获取素材库列表
func (ctrl *MaterialLibraryController) List(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	// 按 ID 精确查询
	if id := c.Query("Id"); id != "" {
		ml, err := ctrl.svc.GetByID(c.Request.Context(), id)
		if err != nil {
			handleError(c, err)
			return
		}
		common.OK(c, common.PageData{
			Total:    1,
			Page:     1,
			PageSize: 1,
			Items:    []model.MaterialLibrary{*ml},
		})
		return
	}

	// 分页查询
	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "20"))
	libType := c.Query("Type")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	items, total, err := ctrl.svc.List(c.Request.Context(), page, pageSize, libType)
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

// Update 更新素材库
func (ctrl *MaterialLibraryController) Update(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.UpdateMaterialLibraryReq
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

// Delete 删除素材库
func (ctrl *MaterialLibraryController) Delete(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	if err := ctrl.svc.Delete(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// UploadImages 批量上传图片
func (ctrl *MaterialLibraryController) UploadImages(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	result, err := ctrl.svc.UploadImages(c.Request.Context(), id, c)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, result)
}

// InitVideoUpload 初始化视频上传
func (ctrl *MaterialLibraryController) InitVideoUpload(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.InitVideoUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: "+err.Error())
		return
	}

	result, err := ctrl.svc.InitVideoUpload(c.Request.Context(), id, &req)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, result)
}

// UploadChunk 上传视频分片
func (ctrl *MaterialLibraryController) UploadChunk(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	uploadId := c.PostForm("UploadId")
	chunkIndexStr := c.PostForm("ChunkIndex")
	if uploadId == "" || chunkIndexStr == "" {
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "UploadId 和 ChunkIndex 为必填")
		return
	}

	chunkIndex, err := strconv.ParseInt(chunkIndexStr, 10, 32)
	if err != nil {
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "ChunkIndex 格式错误")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "分片文件不存在")
		return
	}

	if err := ctrl.svc.UploadChunk(c.Request.Context(), uploadId, int32(chunkIndex), file); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// CompleteVideoUpload 完成视频上传
func (ctrl *MaterialLibraryController) CompleteVideoUpload(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.CompleteVideoUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.CompleteVideoUpload(c.Request.Context(), req.UploadId); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// ListFiles 获取素材库文件列表
func (ctrl *MaterialLibraryController) ListFiles(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "24"))
	uploadStatus := c.Query("UploadStatus")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}

	items, total, err := ctrl.svc.ListFiles(c.Request.Context(), id, page, pageSize, uploadStatus)
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

// DeleteFile 删除素材文件
func (ctrl *MaterialLibraryController) DeleteFile(c *gin.Context) {
	id := c.Param("id")
	fileId := c.Param("fileId")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id, "fileId", fileId)

	if err := ctrl.svc.DeleteFile(c.Request.Context(), id, fileId); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}
