package repository

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
)

// ──────────────────────────── 结构体 ────────────────────────────

// TaskRepo 任务数据访问层
type TaskRepo struct {
	db *gorm.DB
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewTaskRepo 创建任务仓储实例
func NewTaskRepo(db *gorm.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

// Create 插入一条任务
func (r *TaskRepo) Create(ctx context.Context, task *model.Task) error {
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		slog.Error("插入任务失败", "id", task.Id, "error", err)
		return fmt.Errorf("插入任务失败: %w", err)
	}
	slog.Info("插入任务成功", "id", task.Id, "name", task.Name)
	return nil
}

// GetByID 根据 ID 查询单条任务
func (r *TaskRepo) GetByID(ctx context.Context, id string) (*model.Task, error) {
	var task model.Task
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error
	if err == gorm.ErrRecordNotFound {
		slog.Warn("任务不存在", "id", id)
		return nil, nil
	}
	if err != nil {
		slog.Error("按ID查询任务失败", "id", id, "error", err)
		return nil, fmt.Errorf("按ID查询任务失败: %w", err)
	}
	return &task, nil
}

// List 分页查询任务列表，返回列表和总数
func (r *TaskRepo) List(ctx context.Context, page, pageSize int, status string) ([]model.Task, int, error) {
	query := r.db.WithContext(ctx).Model(&model.Task{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("统计任务数量失败", "error", err)
		return nil, 0, fmt.Errorf("统计任务数量失败: %w", err)
	}

	var items []model.Task
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		slog.Error("查询任务列表失败", "page", page, "pageSize", pageSize, "error", err)
		return nil, 0, fmt.Errorf("查询任务列表失败: %w", err)
	}

	slog.Info("查询任务列表", "page", page, "pageSize", pageSize, "total", total, "count", len(items))
	return items, int(total), nil
}

// UpdateStatus 更新任务状态
func (r *TaskRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	result := r.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      status,
		"fail_reason": nil,
		"updated_at":  common.NowFormatted(),
	})
	if result.Error != nil {
		slog.Error("更新任务状态失败", "id", id, "error", result.Error)
		return fmt.Errorf("更新任务状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateStatusWithReason 更新任务状态并设置失败原因
func (r *TaskRepo) UpdateStatusWithReason(ctx context.Context, id string, status string, failReason string) error {
	result := r.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      status,
		"fail_reason": failReason,
		"updated_at":  common.NowFormatted(),
	})
	if result.Error != nil {
		slog.Error("更新任务状态失败", "id", id, "status", status, "error", result.Error)
		return fmt.Errorf("更新任务状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete 删除任务
func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Task{})
	if result.Error != nil {
		slog.Error("删除任务失败", "id", id, "error", result.Error)
		return fmt.Errorf("删除任务失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	slog.Info("删除任务成功", "id", id)
	return nil
}

// FindByMaterialLibraryId 根据素材库 ID 查询任务
func (r *TaskRepo) FindByMaterialLibraryId(ctx context.Context, libraryId string) (*model.Task, error) {
	var task model.Task
	err := r.db.WithContext(ctx).Where("material_library_id = ?", libraryId).First(&task).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("按素材库ID查询任务失败", "libraryId", libraryId, "error", err)
		return nil, fmt.Errorf("按素材库ID查询任务失败: %w", err)
	}
	return &task, nil
}

// FindByModelConfigId 根据模型配置 ID 查询是否存在关联任务
func (r *TaskRepo) FindByModelConfigId(ctx context.Context, modelConfigId string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Task{}).Where("model_config_id = ?", modelConfigId).Count(&count).Error
	if err != nil {
		slog.Error("检查模型配置关联任务失败", "modelConfigId", modelConfigId, "error", err)
		return false, fmt.Errorf("检查模型配置关联任务失败: %w", err)
	}
	return count > 0, nil
}

// FindRecoverableTasks 查询可恢复的任务（Pending 或 Analyzing 状态）
func (r *TaskRepo) FindRecoverableTasks(ctx context.Context) ([]model.Task, error) {
	var tasks []model.Task
	err := r.db.WithContext(ctx).Where("status IN ?", []string{"Pending", "Analyzing", "Failed"}).Find(&tasks).Error
	if err != nil {
		slog.Error("查询可恢复任务失败", "error", err)
		return nil, fmt.Errorf("查询可恢复任务失败: %w", err)
	}
	return tasks, nil
}

// ──────────────────────────── Image 数据访问 ────────────────────────────

// CreateImage 插入一条检测素材
func (r *TaskRepo) CreateImage(ctx context.Context, img *model.Image) error {
	if err := r.db.WithContext(ctx).Create(img).Error; err != nil {
		slog.Error("插入检测素材失败", "id", img.Id, "error", err)
		return fmt.Errorf("插入检测素材失败: %w", err)
	}
	return nil
}

// CreateImages 批量插入检测素材
func (r *TaskRepo) CreateImages(ctx context.Context, images []model.Image) error {
	if len(images) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&images).Error; err != nil {
		slog.Error("批量插入检测素材失败", "count", len(images), "error", err)
		return fmt.Errorf("批量插入检测素材失败: %w", err)
	}
	return nil
}

// GetImageByID 根据 ID 查询单条检测素材
func (r *TaskRepo) GetImageByID(ctx context.Context, id string) (*model.Image, error) {
	var img model.Image
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&img).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("按ID查询检测素材失败: %w", err)
	}
	return &img, nil
}

// GetImageByIDAndTaskId 按 ID + 任务 ID 精确查询检测素材
func (r *TaskRepo) GetImageByIDAndTaskId(ctx context.Context, taskId, imageId string) (*model.Image, error) {
	var img model.Image
	err := r.db.WithContext(ctx).Where("id = ? AND task_id = ?", imageId, taskId).First(&img).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("按ID和任务ID查询检测素材失败: %w", err)
	}
	return &img, nil
}

// ListImages 分页查询任务下检测素材列表
func (r *TaskRepo) ListImages(ctx context.Context, taskId string, page, pageSize int, status string, correction string) ([]model.Image, int, error) {
	query := r.db.WithContext(ctx).Model(&model.Image{}).Where("task_id = ?", taskId)

	// 默认排除已删除误报的素材
	if correction == "" {
		query = query.Where("correction IS NULL OR correction != 'DeletedFp'")
	} else {
		query = query.Where("correction = ?", correction)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("统计素材数量失败", "taskId", taskId, "error", err)
		return nil, 0, fmt.Errorf("统计素材数量失败: %w", err)
	}

	var items []model.Image
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		slog.Error("查询素材列表失败", "taskId", taskId, "page", page, "pageSize", pageSize, "error", err)
		return nil, 0, fmt.Errorf("查询素材列表失败: %w", err)
	}

	slog.Info("查询素材列表", "taskId", taskId, "page", page, "pageSize", pageSize, "total", total, "count", len(items))
	return items, int(total), nil
}

// ListPendingImages 查询任务下所有待检测素材
func (r *TaskRepo) ListPendingImages(ctx context.Context, taskId string) ([]model.Image, error) {
	var images []model.Image
	err := r.db.WithContext(ctx).Where("task_id = ? AND status = ?", taskId, "Pending").Find(&images).Error
	if err != nil {
		return nil, fmt.Errorf("查询待检测素材失败: %w", err)
	}
	return images, nil
}

// UpdateImage 更新检测素材
func (r *TaskRepo) UpdateImage(ctx context.Context, id string, updates map[string]interface{}) error {
	result := r.db.WithContext(ctx).Model(&model.Image{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		slog.Error("更新检测素材失败", "id", id, "error", result.Error)
		return fmt.Errorf("更新检测素材失败: %w", result.Error)
	}
	return nil
}

// DeleteImagesByTaskId 删除任务下所有检测素材
func (r *TaskRepo) DeleteImagesByTaskId(ctx context.Context, taskId string) error {
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskId).Delete(&model.Image{}).Error; err != nil {
		slog.Error("删除任务检测素材失败", "taskId", taskId, "error", err)
		return fmt.Errorf("删除任务检测素材失败: %w", err)
	}
	return nil
}

// CountImagesByStatus 按状态统计任务下的素材数量
func (r *TaskRepo) CountImagesByStatus(ctx context.Context, taskId string) (total int, pending int, detected int, notDetected int, failed int, truePositive int, falsePositive int, err error) {
	// 排除已删除误报的素材
	excludeDeletedFp := "task_id = ? AND (correction IS NULL OR correction != 'DeletedFp')"

	var totalVal int64
	r.db.WithContext(ctx).Model(&model.Image{}).Where(excludeDeletedFp, taskId).Count(&totalVal)
	total = int(totalVal)

	r.db.WithContext(ctx).Model(&model.Image{}).Where("task_id = ? AND status = ?", taskId, "Pending").Count(&totalVal)
	pending = int(totalVal)

	r.db.WithContext(ctx).Model(&model.Image{}).Where(excludeDeletedFp+" AND status = ?", taskId, "NotDetected").Count(&totalVal)
	notDetected = int(totalVal)

	r.db.WithContext(ctx).Model(&model.Image{}).Where(excludeDeletedFp+" AND status = ?", taskId, "Failed").Count(&totalVal)
	failed = int(totalVal)

	// Detected 且非 DeletedFp
	r.db.WithContext(ctx).Model(&model.Image{}).Where(excludeDeletedFp+" AND status = ?", taskId, "Detected").Count(&totalVal)
	detected = int(totalVal)

	// TruePositive = Detected AND Correction IS NULL
	r.db.WithContext(ctx).Model(&model.Image{}).Where("task_id = ? AND status = ? AND correction IS NULL", taskId, "Detected").Count(&totalVal)
	truePositive = int(totalVal)

	// FalsePositive = Detected AND Correction = 'FalsePositive'
	r.db.WithContext(ctx).Model(&model.Image{}).Where("task_id = ? AND status = ? AND correction = ?", taskId, "Detected", "FalsePositive").Count(&totalVal)
	falsePositive = int(totalVal)

	return
}
