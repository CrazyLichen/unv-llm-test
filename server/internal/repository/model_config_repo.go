package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
)

// ──────────────────────────── 结构体 ────────────────────────────

// ModelConfigRepo 模型配置数据访问层
type ModelConfigRepo struct {
	db *sql.DB
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewModelConfigRepo 创建模型配置仓储实例
func NewModelConfigRepo(db *sql.DB) *ModelConfigRepo {
	return &ModelConfigRepo{db: db}
}

// Create 插入一条模型配置
func (r *ModelConfigRepo) Create(ctx context.Context, mc *model.ModelConfig) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO model_configs (id, model_name, model_id, api_url, api_key, temperature, max_tokens, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mc.Id, mc.ModelName, mc.ModelId, mc.ApiUrl, mc.ApiKey, mc.Temperature, mc.MaxTokens, mc.CreatedAt, mc.UpdatedAt,
	)
	if err != nil {
		slog.Error("插入模型配置失败", "id", mc.Id, "error", err)
		return fmt.Errorf("插入模型配置失败: %w", err)
	}
	slog.Info("插入模型配置成功", "id", mc.Id, "modelId", mc.ModelId)
	return nil
}

// GetByID 根据 ID 查询单条模型配置
func (r *ModelConfigRepo) GetByID(ctx context.Context, id string) (*model.ModelConfig, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, model_name, model_id, api_url, api_key, temperature, max_tokens, created_at, updated_at
		 FROM model_configs WHERE id = ?`, id)

	var mc model.ModelConfig
	err := row.Scan(&mc.Id, &mc.ModelName, &mc.ModelId, &mc.ApiUrl, &mc.ApiKey, &mc.Temperature, &mc.MaxTokens, &mc.CreatedAt, &mc.UpdatedAt)
	if err == sql.ErrNoRows {
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
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_configs`).Scan(&total)
	if err != nil {
		slog.Error("统计模型配置数量失败", "error", err)
		return nil, 0, fmt.Errorf("统计模型配置数量失败: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, model_name, model_id, api_url, api_key, temperature, max_tokens, created_at, updated_at
		 FROM model_configs ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, offset)
	if err != nil {
		slog.Error("查询模型配置列表失败", "page", page, "pageSize", pageSize, "error", err)
		return nil, 0, fmt.Errorf("查询模型配置列表失败: %w", err)
	}
	defer rows.Close()

	var items []model.ModelConfig
	for rows.Next() {
		var mc model.ModelConfig
		if err := rows.Scan(&mc.Id, &mc.ModelName, &mc.ModelId, &mc.ApiUrl, &mc.ApiKey, &mc.Temperature, &mc.MaxTokens, &mc.CreatedAt, &mc.UpdatedAt); err != nil {
			slog.Error("扫描模型配置记录失败", "error", err)
			return nil, 0, fmt.Errorf("扫描模型配置记录失败: %w", err)
		}
		items = append(items, mc)
	}

	slog.Info("查询模型配置列表", "page", page, "pageSize", pageSize, "total", total, "count", len(items))
	return items, total, nil
}

// Update 更新模型配置（仅更新非 nil 字段）
func (r *ModelConfigRepo) Update(ctx context.Context, id string, req *model.UpdateModelConfigReq) error {
	var sets []string
	var args []interface{}

	if req.ModelName != nil {
		sets = append(sets, "model_name = ?")
		args = append(args, *req.ModelName)
	}
	if req.ModelId != nil {
		sets = append(sets, "model_id = ?")
		args = append(args, *req.ModelId)
	}
	if req.ApiUrl != nil {
		sets = append(sets, "api_url = ?")
		args = append(args, *req.ApiUrl)
	}
	if req.ApiKey != nil {
		sets = append(sets, "api_key = ?")
		args = append(args, *req.ApiKey)
	}
	if req.Temperature != nil {
		sets = append(sets, "temperature = ?")
		args = append(args, *req.Temperature)
	}
	if req.MaxTokens != nil {
		sets = append(sets, "max_tokens = ?")
		args = append(args, *req.MaxTokens)
	}

	// 无字段需要更新
	if len(sets) == 0 {
		slog.Warn("更新模型配置无字段变更", "id", id)
		return nil
	}

	sets = append(sets, "updated_at = ?")
	args = append(args, common.NowFormatted())
	args = append(args, id)

	query := fmt.Sprintf("UPDATE model_configs SET %s WHERE id = ?", joinSets(sets))
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		slog.Error("更新模型配置失败", "id", id, "error", err)
		return fmt.Errorf("更新模型配置失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	slog.Info("更新模型配置成功", "id", id, "fields", len(sets)-1)
	return nil
}

// Delete 删除模型配置
func (r *ModelConfigRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM model_configs WHERE id = ?`, id)
	if err != nil {
		slog.Error("删除模型配置失败", "id", id, "error", err)
		return fmt.Errorf("删除模型配置失败: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	slog.Info("删除模型配置成功", "id", id)
	return nil
}

// HasRelatedTasks 检查是否有关联任务（tasks 表尚未创建，预留接口）
func (r *ModelConfigRepo) HasRelatedTasks(ctx context.Context, modelConfigId string) (bool, error) {
	// tasks 表尚未创建，返回 false
	// 后续创建 tasks 表后实现：SELECT COUNT(*) FROM tasks WHERE model_config_id = ?
	return false, nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// joinSets 拼接 SET 子句
func joinSets(sets []string) string {
	result := sets[0]
	for i := 1; i < len(sets); i++ {
		result += ", " + sets[i]
	}
	return result
}
