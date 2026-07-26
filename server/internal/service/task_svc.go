package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)

// ──────────────────────────── 结构体 ────────────────────────────

// TaskService 任务业务逻辑层
type TaskService struct {
	repo      *repository.TaskRepo
	mcRepo    *repository.ModelConfigRepo
	mlRepo    *repository.MaterialLibraryRepo
	scheduler *Scheduler
	uploadDir string
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewTaskService 创建任务服务实例
func NewTaskService(
	repo *repository.TaskRepo,
	mcRepo *repository.ModelConfigRepo,
	mlRepo *repository.MaterialLibraryRepo,
	scheduler *Scheduler,
	uploadDir string,
) *TaskService {
	return &TaskService{
		repo:      repo,
		mcRepo:    mcRepo,
		mlRepo:    mlRepo,
		scheduler: scheduler,
		uploadDir: uploadDir,
	}
}

// Create 创建任务
func (s *TaskService) Create(ctx context.Context, req *model.CreateTaskReq) error {
	// 校验 Type=Video 时 FrameInterval 必填
	if req.Type == "Video" && (req.FrameInterval == nil || *req.FrameInterval <= 0) {
		return common.NewErrParamValidation("视频任务必须指定抽帧间隔")
	}

	// 校验 ModelConfigId 是否存在
	mc, err := s.mcRepo.GetByID(ctx, req.ModelConfigId)
	if err != nil {
		slog.Error("查询模型配置失败", "modelConfigId", req.ModelConfigId, "error", err)
		return err
	}
	if mc == nil {
		slog.Warn("模型配置不存在", "modelConfigId", req.ModelConfigId)
		return common.ErrModelConfigNotFound
	}

	// 校验 MaterialLibraryId 是否存在
	ml, err := s.mlRepo.GetByID(ctx, req.MaterialLibraryId)
	if err != nil {
		slog.Error("查询素材库失败", "materialLibraryId", req.MaterialLibraryId, "error", err)
		return err
	}
	if ml == nil {
		slog.Warn("素材库不存在", "materialLibraryId", req.MaterialLibraryId)
		return common.ErrMaterialLibNotFound
	}

	// 校验素材库类型与任务类型一致
	if ml.Type != req.Type {
		slog.Warn("素材库类型与任务类型不匹配", "materialLibraryId", req.MaterialLibraryId, "libType", ml.Type, "taskType", req.Type)
		return common.ErrLibTypeMismatch
	}

	// 校验素材库中不存在未完成上传的文件
	hasIncomplete, err := s.mlRepo.HasIncompleteFiles(ctx, req.MaterialLibraryId)
	if err != nil {
		slog.Error("检查素材库文件上传状态失败", "materialLibraryId", req.MaterialLibraryId, "error", err)
		return err
	}
	if hasIncomplete {
		slog.Warn("素材库存在未完成上传的文件", "materialLibraryId", req.MaterialLibraryId)
		return common.ErrFileUploadIncomplete
	}

	// 生成任务 ID
	id, err := generateTaskID()
	if err != nil {
		return fmt.Errorf("生成任务ID失败: %w", err)
	}

	now := common.NowFormatted()
	task := &model.Task{
		Id:                id,
		Name:              req.Name,
		Type:              req.Type,
		Status:            "Pending",
		ModelConfigId:     req.ModelConfigId,
		MaterialLibraryId: req.MaterialLibraryId,
		Prompt:            req.Prompt,
		Target:            req.Target,
		FrameInterval:     req.FrameInterval,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.repo.Create(ctx, task); err != nil {
		return err
	}

	// Type=Image 时：为素材库下所有图片创建 Image 记录
	if req.Type == "Image" {
		if err := s.createImageRecords(ctx, id, req.MaterialLibraryId); err != nil {
			slog.Error("创建图片素材记录失败", "taskId", id, "error", err)
			return err
		}
	}

	// 入队调度
	s.scheduler.Enqueue(id)

	slog.Info("创建任务成功", "id", id, "type", req.Type, "name", req.Name)
	return nil
}

// GetByID 按 ID 查询任务（附带 Progress 和关联名称）
func (s *TaskService) GetByID(ctx context.Context, id string) (*model.TaskItem, error) {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		slog.Error("查询任务失败", "id", id, "error", err)
		return nil, err
	}
	if task == nil {
		slog.Warn("任务不存在", "id", id)
		return nil, common.ErrTaskNotFound
	}
	return s.toTaskItem(ctx, task)
}

// List 分页查询任务列表
func (s *TaskService) List(ctx context.Context, page, pageSize int, status string) ([]model.TaskItem, int, error) {
	tasks, total, err := s.repo.List(ctx, page, pageSize, status)
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.TaskItem, 0, len(tasks))
	for _, t := range tasks {
		item, err := s.toTaskItem(ctx, &t)
		if err != nil {
			slog.Warn("转换任务项失败", "taskId", t.Id, "error", err)
			continue
		}
		items = append(items, *item)
	}

	return items, total, nil
}

// Delete 删除任务（级联硬删除）
func (s *TaskService) Delete(ctx context.Context, id string) error {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		slog.Error("查询任务失败", "id", id, "error", err)
		return err
	}
	if task == nil {
		slog.Warn("删除任务不存在", "id", id)
		return common.ErrTaskNotFound
	}

	// 取消正在执行的任务
	s.scheduler.CancelTask(id)

	// 删除抽帧文件目录
	frameDir := filepath.Join(s.uploadDir, "tasks", id, "frames")
	if err := os.RemoveAll(frameDir); err != nil {
		slog.Warn("删除抽帧文件目录失败", "dir", frameDir, "error", err)
	}
	// 清理 tasks/id 空目录
	taskDir := filepath.Join(s.uploadDir, "tasks", id)
	os.Remove(taskDir)

	// 级联删除素材记录
	if err := s.repo.DeleteImagesByTaskId(ctx, id); err != nil {
		slog.Error("删除任务素材记录失败", "taskId", id, "error", err)
		return err
	}

	// 删除任务
	if err := s.repo.Delete(ctx, id); err != nil {
		slog.Error("删除任务记录失败", "id", id, "error", err)
		return err
	}

	slog.Info("删除任务成功", "id", id)
	return nil
}

// Update 更新任务状态（暂停/恢复）
func (s *TaskService) Update(ctx context.Context, id string, req *model.UpdateTaskReq) error {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		slog.Error("查询任务失败", "id", id, "error", err)
		return err
	}
	if task == nil {
		slog.Warn("更新任务不存在", "id", id)
		return common.ErrTaskNotFound
	}

	switch req.Status {
	case "Paused":
		// 仅 Pending 和 Analyzing 状态可暂停
		if task.Status != "Pending" && task.Status != "Analyzing" {
			slog.Warn("任务状态不允许暂停", "id", id, "currentStatus", task.Status)
			return common.ErrTaskStatusInvalid
		}
		if err := s.repo.UpdateStatus(ctx, id, "Paused"); err != nil {
			slog.Error("更新任务状态为Paused失败", "id", id, "error", err)
			return err
		}
		// 取消 context
		s.scheduler.CancelTask(id)
		slog.Info("暂停任务成功", "id", id, "prevStatus", task.Status)

	case "Analyzing":
		// Paused 或 Failed 状态可恢复
		if task.Status != "Paused" && task.Status != "Failed" {
			slog.Warn("任务状态不允许恢复", "id", id, "currentStatus", task.Status)
			return common.ErrTaskStatusInvalid
		}
		if err := s.repo.UpdateStatus(ctx, id, "Analyzing"); err != nil {
			slog.Error("更新任务状态为Analyzing失败", "id", id, "error", err)
			return err
		}
		// 重新入队（创建新 context）
		s.scheduler.Enqueue(id)
		slog.Info("恢复任务成功", "id", id)

	default:
		slog.Warn("不支持的状态更新", "id", id, "status", req.Status)
		return common.NewErrParamValidation("不支持的状态: " + req.Status)
	}

	return nil
}

// ListImages 获取任务分析结果列表
func (s *TaskService) ListImages(ctx context.Context, taskId, imageId string, page, pageSize int, status, correction string) ([]model.ImageItem, int, error) {
	// 校验任务存在性
	task, err := s.repo.GetByID(ctx, taskId)
	if err != nil {
		return nil, 0, err
	}
	if task == nil {
		return nil, 0, common.ErrTaskNotFound
	}

	// 按 ImageId 精确查询
	if imageId != "" {
		img, err := s.repo.GetImageByIDAndTaskId(ctx, taskId, imageId)
		if err != nil {
			return nil, 0, err
		}
		if img == nil {
			return nil, 0, common.ErrImageNotFound
		}
		item := s.toImageItem(img)
		return []model.ImageItem{item}, 1, nil
	}

	// 分页查询
	images, total, err := s.repo.ListImages(ctx, taskId, page, pageSize, status, correction)
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.ImageItem, 0, len(images))
	for i := range images {
		items = append(items, s.toImageItem(&images[i]))
	}

	return items, total, nil
}

// UploadMissedPhoto 补录漏报照片
func (s *TaskService) UploadMissedPhoto(ctx context.Context, taskId string, file *multipart.FileHeader) error {
	// 校验任务存在性
	task, err := s.repo.GetByID(ctx, taskId)
	if err != nil {
		return err
	}
	if task == nil {
		return common.ErrTaskNotFound
	}

	// 生成素材 ID
	imgId, err := generateImageID()
	if err != nil {
		return fmt.Errorf("生成素材ID失败: %w", err)
	}

	// 保存文件到磁盘
	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	frameDir := filepath.Join(s.uploadDir, "tasks", taskId, "frames")
	if err := os.MkdirAll(frameDir, 0755); err != nil {
		return common.NewErrFileUploadFailed("创建帧目录失败")
	}

	fullPath := filepath.Join(frameDir, imgId+ext)
	src, err := file.Open()
	if err != nil {
		return common.NewErrFileUploadFailed("打开上传文件失败")
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return common.NewErrFileUploadFailed("创建目标文件失败")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(fullPath)
		return common.NewErrFileUploadFailed("写入文件失败")
	}

	// 创建 Image 记录
	accessUrl := filepath.Join("/uploads/tasks", taskId, "frames", imgId+ext)
	img := &model.Image{
		Id:        imgId,
		TaskId:    taskId,
		AccessUrl: accessUrl,
		Status:    "Detected",
		Detection: sql.NullString{Valid: false},
		CreatedAt: common.NowFormatted(),
	}

	if err := s.repo.CreateImage(ctx, img); err != nil {
		os.Remove(fullPath)
		return err
	}

	slog.Info("补录漏报照片成功", "taskId", taskId, "imageId", imgId)
	return nil
}

// MarkCorrection 标记素材误报/恢复
func (s *TaskService) MarkCorrection(ctx context.Context, taskId, imageId string, correction *string) error {
	// 校验任务存在性
	task, err := s.repo.GetByID(ctx, taskId)
	if err != nil {
		return err
	}
	if task == nil {
		return common.ErrTaskNotFound
	}

	// 查询素材
	img, err := s.repo.GetImageByIDAndTaskId(ctx, taskId, imageId)
	if err != nil {
		return err
	}
	if img == nil {
		return common.ErrImageNotFound
	}

	// 校验 Correction 值域
	if correction != nil && *correction != "FalsePositive" && *correction != "DeletedFp" {
		return common.NewErrParamValidation("Correction 值域无效，允许: null, FalsePositive, DeletedFp")
	}

	// 更新 Correction 字段
	if err := s.repo.UpdateImage(ctx, imageId, map[string]interface{}{
		"correction": correction,
	}); err != nil {
		return err
	}

	slog.Info("更新矫正标记成功", "taskId", taskId, "imageId", imageId, "correction", correction)
	return nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// toImageItem 将 Image 实体转换为 ImageItem
func (s *TaskService) toImageItem(img *model.Image) model.ImageItem {
	item := model.ImageItem{
		Id:             img.Id,
		TaskId:         img.TaskId,
		AccessUrl:      img.AccessUrl,
		MaterialFileId: img.MaterialFileId,
		FrameIndex:     img.FrameIndex,
		Status:         img.Status,
		FailReason:     img.FailReason,
		Correction:     img.Correction,
	}

	// 解析 Detection JSON
	if img.Detection.Valid {
		var det model.Detection
		if err := json.Unmarshal([]byte(img.Detection.String), &det); err == nil {
			item.Detection = &det
		}
	}

	return item
}

// createImageRecords 为图片集任务创建 Image 记录
func (s *TaskService) createImageRecords(ctx context.Context, taskId string, libraryId string) error {
	// 查询素材库下所有已完成的文件
	files, _, err := s.mlRepo.ListFiles(ctx, libraryId, 1, 100000, "Completed")
	if err != nil {
		slog.Error("查询素材库文件失败", "libraryId", libraryId, "error", err)
		return err
	}

	slog.Info("为任务创建图片素材记录", "taskId", taskId, "fileCount", len(files))

	images := make([]model.Image, 0, len(files))
	for _, f := range files {
		imgId, err := generateImageID()
		if err != nil {
			return fmt.Errorf("生成素材ID失败: %w", err)
		}
		fileId := f.Id
		images = append(images, model.Image{
			Id:             imgId,
			TaskId:         taskId,
			AccessUrl:      f.AccessUrl,
			MaterialFileId: &fileId,
			Status:         "Pending",
			CreatedAt:      common.NowFormatted(),
		})
	}

	if err := s.repo.CreateImages(ctx, images); err != nil {
		slog.Error("批量创建图片素材记录失败", "taskId", taskId, "count", len(images), "error", err)
		return err
	}

	slog.Info("创建图片素材记录成功", "taskId", taskId, "count", len(images))
	return nil
}

// toTaskItem 将 Task 实体转换为 TaskItem（附带关联名称和进度）
func (s *TaskService) toTaskItem(ctx context.Context, task *model.Task) (*model.TaskItem, error) {
	// 查询关联名称
	mc, _ := s.mcRepo.GetByID(ctx, task.ModelConfigId)
	mcName := ""
	if mc != nil {
		mcName = mc.ModelName
	}

	ml, _ := s.mlRepo.GetByID(ctx, task.MaterialLibraryId)
	mlName := ""
	if ml != nil {
		mlName = ml.Name
	}

	// 查询进度
	progress := s.getProgress(ctx, task.Id)

	return &model.TaskItem{
		Id:                 task.Id,
		Name:               task.Name,
		Type:               task.Type,
		Status:             task.Status,
		ModelConfigId:      task.ModelConfigId,
		ModelConfigName:    mcName,
		MaterialLibraryId:  task.MaterialLibraryId,
		MaterialLibraryName: mlName,
		Prompt:             task.Prompt,
		Target:             task.Target,
		FrameInterval:      task.FrameInterval,
		FailReason:         task.FailReason,
		Progress:           progress,
		CreatedAt:          task.CreatedAt,
	}, nil
}

// getProgress 获取任务检测进度
func (s *TaskService) getProgress(ctx context.Context, taskId string) model.TaskProgress {
	total, pending, detected, notDetected, failed, truePositive, falsePositive, err := s.repo.CountImagesByStatus(ctx, taskId)
	if err != nil {
		slog.Warn("查询任务进度失败", "taskId", taskId, "error", err)
		return model.TaskProgress{}
	}

	completed := detected + notDetected + failed
	return model.TaskProgress{
		Total:     total,
		Completed: completed,
		CompletedDetail: model.CompletedDetail{
			Detected: detected,
			DetectedDetail: model.DetectedDetail{
				TruePositive:  truePositive,
				FalsePositive: falsePositive,
			},
			NotDetected: notDetected,
			Failed:      failed,
		},
		Pending: pending,
	}
}

// generateTaskID 生成 task_{uuid32} 格式的任务 ID
func generateTaskID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "task_" + hex.EncodeToString(bytes), nil
}

// generateImageID 生成 img_{uuid32} 格式的素材 ID
func generateImageID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "img_" + hex.EncodeToString(bytes), nil
}
