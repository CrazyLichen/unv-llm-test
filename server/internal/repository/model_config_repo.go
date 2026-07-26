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

// ModelConfigRepo 模型配置数据访问层
type ModelConfigRepo struct {
	db *gorm.DB
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewModelConfigRepo 创建模型配置仓储实例
func NewModelConfigRepo(db *gorm.DB) *ModelConfigRepo {
	return &ModelConfigRepo{db: db}
}

// Create 插入一条模型配置
func (r *ModelConfigRepo) Create(ctx context.Context, mc *model.ModelConfig) error {
	if err := r.db.WithContext(ctx).Create(mc).Error; err != nil {
		slog.Error("插入模型配置失败", "id", mc.Id, "error", err)
		return fmt.Errorf("插入模型配置失败: %w", err)
	}
	slog.Info("插入模型配置成功", "id", mc.Id, "modelId", mc.ModelId)
	return nil
}

// GetByID 根据 ID 查询单条模型配置
func (r *ModelConfigRepo) GetByID(ctx context.Context, id string) (*model.ModelConfig, error) {
	var mc model.ModelConfig
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&mc).Error
	if err == gorm.ErrRecordNotFound {
		slog.Warn("模型配置不存在", "id", id)
		return nil, nil
	}
	if err != nil {
		slog.Error("按ID查询模型配置失败", "id", id, "error", err)
		return nil, fmt.Errorf("按ID查询模型配置失败: %w", err)
	}
	return &mc, nil
}

// List 分页查询模型配置列表，返回列表和总数
func (r *ModelConfigRepo) List(ctx context.Context, page, pageSize int) ([]model.ModelConfig, int, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.ModelConfig{}).Count(&total).Error; err != nil {
		slog.Error("统计模型配置数量失败", "error", err)
		return nil, 0, fmt.Errorf("统计模型配置数量失败: %w", err)
	}

	var items []model.ModelConfig
	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		slog.Error("查询模型配置列表失败", "page", page, "pageSize", pageSize, "error", err)
		return nil, 0, fmt.Errorf("查询模型配置列表失败: %w", err)
	}

	slog.Info("查询模型配置列表", "page", page, "pageSize", pageSize, "total", total, "count", len(items))
	return items, int(total), nil
}

// Update 更新模型配置（仅更新非 nil 字段）
func (r *ModelConfigRepo) Update(ctx context.Context, id string, req *model.UpdateModelConfigReq) error {
	// 构建更新字段映射，仅包含非 nil 的字段
	updates := map[string]interface{}{}
	if req.ModelName != nil {
		updates["model_name"] = *req.ModelName
	}
	if req.ModelId != nil {
		updates["model_id"] = *req.ModelId
	}
	if req.ApiUrl != nil {
		updates["api_url"] = *req.ApiUrl
	}
	if req.ApiKey != nil {
		updates["api_key"] = *req.ApiKey
	}
	if req.Temperature != nil {
		updates["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		updates["max_tokens"] = *req.MaxTokens
	}

	// 无字段需要更新
	if len(updates) == 0 {
		slog.Warn("更新模型配置无字段变更", "id", id)
		return nil
	}

	// 手动设置更新时间
	updates["updated_at"] = common.NowFormatted()

	// 使用 Select 指定要更新的列，确保零值字段也能被更新
	result := r.db.WithContext(ctx).Model(&model.ModelConfig{}).Where("id = ?", id).Select(mapKeys(updates)).Updates(updates)
	if result.Error != nil {
		slog.Error("更新模型配置失败", "id", id, "error", result.Error)
		return fmt.Errorf("更新模型配置失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	slog.Info("更新模型配置成功", "id", id, "fields", len(updates)-1)
	return nil
}

// Delete 删除模型配置
func (r *ModelConfigRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.ModelConfig{})
	if result.Error != nil {
		slog.Error("删除模型配置失败", "id", id, "error", result.Error)
		return fmt.Errorf("删除模型配置失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	slog.Info("删除模型配置成功", "id", id)
	return nil
}

// HasRelatedTasks 检查是否有关联任务（tasks 表尚未创建，预留接口）
func (r *ModelConfigRepo) HasRelatedTasks(ctx context.Context, modelConfigId string) (bool, error) {
	// tasks 表尚未创建，返回 false
	// 后续创建 tasks 表后实现：db.Where("model_config_id = ?", modelConfigId).Count(...)
	return false, nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// mapKeys 提取 map 的所有 key，用于 GORM Select 指定更新列
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
