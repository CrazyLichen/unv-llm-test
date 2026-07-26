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

// MaterialLibraryRepo 素材库数据访问层
type MaterialLibraryRepo struct {
	db *gorm.DB
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewMaterialLibraryRepo 创建素材库仓储实例
func NewMaterialLibraryRepo(db *gorm.DB) *MaterialLibraryRepo {
	return &MaterialLibraryRepo{db: db}
}

// Create 插入一条素材库
func (r *MaterialLibraryRepo) Create(ctx context.Context, ml *model.MaterialLibrary) error {
	if err := r.db.WithContext(ctx).Create(ml).Error; err != nil {
		slog.Error("插入素材库失败", "id", ml.Id, "error", err)
		return fmt.Errorf("插入素材库失败: %w", err)
	}
	slog.Info("插入素材库成功", "id", ml.Id, "name", ml.Name)
	return nil
}

// GetByID 根据 ID 查询单条素材库
func (r *MaterialLibraryRepo) GetByID(ctx context.Context, id string) (*model.MaterialLibrary, error) {
	var ml model.MaterialLibrary
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&ml).Error
	if err == gorm.ErrRecordNotFound {
		slog.Warn("素材库不存在", "id", id)
		return nil, nil
	}
	if err != nil {
		slog.Error("按ID查询素材库失败", "id", id, "error", err)
		return nil, fmt.Errorf("按ID查询素材库失败: %w", err)
	}
	return &ml, nil
}

// List 分页查询素材库列表，返回列表和总数
func (r *MaterialLibraryRepo) List(ctx context.Context, page, pageSize int, libType string) ([]model.MaterialLibrary, int, error) {
	query := r.db.WithContext(ctx).Model(&model.MaterialLibrary{})
	if libType != "" {
		query = query.Where("type = ?", libType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("统计素材库数量失败", "error", err)
		return nil, 0, fmt.Errorf("统计素材库数量失败: %w", err)
	}

	var items []model.MaterialLibrary
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		slog.Error("查询素材库列表失败", "page", page, "pageSize", pageSize, "error", err)
		return nil, 0, fmt.Errorf("查询素材库列表失败: %w", err)
	}

	slog.Info("查询素材库列表", "page", page, "pageSize", pageSize, "total", total, "count", len(items))
	return items, int(total), nil
}

// Update 更新素材库
func (r *MaterialLibraryRepo) Update(ctx context.Context, id string, req *model.UpdateMaterialLibraryReq) error {
	updates := map[string]interface{}{
		"name":       req.Name,
		"updated_at": common.NowFormatted(),
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	result := r.db.WithContext(ctx).Model(&model.MaterialLibrary{}).Where("id = ?", id).Select(mapKeys(updates)).Updates(updates)
	if result.Error != nil {
		slog.Error("更新素材库失败", "id", id, "error", result.Error)
		return fmt.Errorf("更新素材库失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	slog.Info("更新素材库成功", "id", id)
	return nil
}

// Delete 删除素材库
func (r *MaterialLibraryRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.MaterialLibrary{})
	if result.Error != nil {
		slog.Error("删除素材库失败", "id", id, "error", result.Error)
		return fmt.Errorf("删除素材库失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	slog.Info("删除素材库成功", "id", id)
	return nil
}

// HasRelatedTasks 检查素材库是否已关联任务
func (r *MaterialLibraryRepo) HasRelatedTasks(ctx context.Context, libraryId string) (bool, error) {
	// tasks 表尚未创建，返回 false
	return false, nil
}

// ──────────────────────────── 素材文件 ────────────────────────────

// CreateFile 插入一条素材文件
func (r *MaterialLibraryRepo) CreateFile(ctx context.Context, mf *model.MaterialFile) error {
	if err := r.db.WithContext(ctx).Create(mf).Error; err != nil {
		slog.Error("插入素材文件失败", "id", mf.Id, "error", err)
		return fmt.Errorf("插入素材文件失败: %w", err)
	}
	slog.Info("插入素材文件成功", "id", mf.Id, "fileName", mf.FileName)
	return nil
}

// GetFileByID 根据 ID 查询单条素材文件
func (r *MaterialLibraryRepo) GetFileByID(ctx context.Context, id string) (*model.MaterialFile, error) {
	var mf model.MaterialFile
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&mf).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("按ID查询素材文件失败", "id", id, "error", err)
		return nil, fmt.Errorf("按ID查询素材文件失败: %w", err)
	}
	return &mf, nil
}

// GetFileByUploadId 根据 UploadId 查询素材文件
func (r *MaterialLibraryRepo) GetFileByUploadId(ctx context.Context, uploadId string) (*model.MaterialFile, error) {
	var mf model.MaterialFile
	err := r.db.WithContext(ctx).Where("upload_id = ?", uploadId).First(&mf).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("按UploadId查询素材文件失败", "uploadId", uploadId, "error", err)
		return nil, fmt.Errorf("按UploadId查询素材文件失败: %w", err)
	}
	return &mf, nil
}

// FindUploadingFileByName 查找素材库下同名且上传中的文件（断点续传）
func (r *MaterialLibraryRepo) FindUploadingFileByName(ctx context.Context, libraryId string, fileName string) (*model.MaterialFile, error) {
	var mf model.MaterialFile
	err := r.db.WithContext(ctx).
		Where("library_id = ? AND file_name = ? AND upload_status = ?", libraryId, fileName, "Uploading").
		First(&mf).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("查找同名上传中文件失败", "libraryId", libraryId, "fileName", fileName, "error", err)
		return nil, fmt.Errorf("查找同名上传中文件失败: %w", err)
	}
	return &mf, nil
}

// FindCompletedOrMergingFileByName 查找素材库下同名且已完成或合并中的文件
func (r *MaterialLibraryRepo) FindCompletedOrMergingFileByName(ctx context.Context, libraryId string, fileName string) (*model.MaterialFile, error) {
	var mf model.MaterialFile
	err := r.db.WithContext(ctx).
		Where("library_id = ? AND file_name = ? AND upload_status IN ?", libraryId, fileName, []string{"Completed", "Merging"}).
		First(&mf).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("查找同名已完成文件失败", "libraryId", libraryId, "fileName", fileName, "error", err)
		return nil, fmt.Errorf("查找同名已完成文件失败: %w", err)
	}
	return &mf, nil
}

// ListFiles 分页查询素材文件列表，按创建时间升序排列
func (r *MaterialLibraryRepo) ListFiles(ctx context.Context, libraryId string, page, pageSize int, uploadStatus string) ([]model.MaterialFile, int, error) {
	query := r.db.WithContext(ctx).Model(&model.MaterialFile{}).Where("library_id = ?", libraryId)
	if uploadStatus != "" {
		query = query.Where("upload_status = ?", uploadStatus)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("统计素材文件数量失败", "libraryId", libraryId, "error", err)
		return nil, 0, fmt.Errorf("统计素材文件数量失败: %w", err)
	}

	var items []model.MaterialFile
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		slog.Error("查询素材文件列表失败", "libraryId", libraryId, "page", page, "error", err)
		return nil, 0, fmt.Errorf("查询素材文件列表失败: %w", err)
	}

	return items, int(total), nil
}

// UpdateFile 更新素材文件
func (r *MaterialLibraryRepo) UpdateFile(ctx context.Context, id string, updates map[string]interface{}) error {
	result := r.db.WithContext(ctx).Model(&model.MaterialFile{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		slog.Error("更新素材文件失败", "id", id, "error", result.Error)
		return fmt.Errorf("更新素材文件失败: %w", result.Error)
	}
	return nil
}

// DeleteFile 删除素材文件
func (r *MaterialLibraryRepo) DeleteFile(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.MaterialFile{})
	if result.Error != nil {
		slog.Error("删除素材文件失败", "id", id, "error", result.Error)
		return fmt.Errorf("删除素材文件失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	slog.Info("删除素材文件成功", "id", id)
	return nil
}

// UpdateLibraryStats 更新素材库的文件数量和总大小
func (r *MaterialLibraryRepo) UpdateLibraryStats(ctx context.Context, libraryId string) error {
	var count int64
	var totalSize int64
	r.db.WithContext(ctx).Model(&model.MaterialFile{}).
		Where("library_id = ? AND upload_status = ?", libraryId, "Completed").
		Count(&count)
	r.db.WithContext(ctx).Model(&model.MaterialFile{}).
		Where("library_id = ? AND upload_status = ?", libraryId, "Completed").
		Select("COALESCE(SUM(file_size), 0)").Scan(&totalSize)

	result := r.db.WithContext(ctx).Model(&model.MaterialLibrary{}).Where("id = ?", libraryId).
		Updates(map[string]interface{}{
			"file_count": int32(count),
			"total_size": totalSize,
			"updated_at": common.NowFormatted(),
		})
	if result.Error != nil {
		slog.Error("更新素材库统计失败", "libraryId", libraryId, "error", result.Error)
		return fmt.Errorf("更新素材库统计失败: %w", result.Error)
	}
	return nil
}

// HasIncompleteFiles 检查素材库中是否有未完成上传的文件
func (r *MaterialLibraryRepo) HasIncompleteFiles(ctx context.Context, libraryId string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.MaterialFile{}).
		Where("library_id = ? AND upload_status != ?", libraryId, "Completed").
		Count(&count).Error
	if err != nil {
		slog.Error("检查未完成文件失败", "libraryId", libraryId, "error", err)
		return false, fmt.Errorf("检查未完成文件失败: %w", err)
	}
	return count > 0, nil
}
