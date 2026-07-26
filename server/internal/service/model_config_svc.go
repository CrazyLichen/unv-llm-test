package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"llm-test-server/internal/common"
	"llm-test-server/internal/llm"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)

// ──────────────────────────── 结构体 ────────────────────────────

// ModelConfigService 模型配置业务逻辑层
type ModelConfigService struct {
	repo      *repository.ModelConfigRepo
	llmClient *llm.LLMClient
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewModelConfigService 创建模型配置服务实例
func NewModelConfigService(repo *repository.ModelConfigRepo, llmClient *llm.LLMClient) *ModelConfigService {
	return &ModelConfigService{repo: repo, llmClient: llmClient}
}

// Create 创建模型配置
func (s *ModelConfigService) Create(ctx context.Context, req *model.CreateModelConfigReq) error {
	id, err := generateID(req.ModelId)
	if err != nil {
		return fmt.Errorf("生成ID失败: %w", err)
	}

	temp := 0.7
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	maxTokens := int32(4096)
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	now := common.NowFormatted()
	mc := &model.ModelConfig{
		Id:          id,
		ModelName:   req.ModelName,
		ModelId:     req.ModelId,
		ApiUrl:      req.ApiUrl,
		ApiKey:      req.ApiKey,
		Temperature: temp,
		MaxTokens:   maxTokens,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, mc); err != nil {
		slog.Error("创建模型配置失败", "modelId", req.ModelId, "error", err)
		return err
	}

	slog.Info("创建模型配置成功", "id", id, "modelId", req.ModelId, "modelName", req.ModelName)
	return nil
}

// GetByID 按 ID 查询模型配置
func (s *ModelConfigService) GetByID(ctx context.Context, id string) (*model.ModelConfig, error) {
	mc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if mc == nil {
		return nil, common.ErrModelConfigNotFound
	}
	mc.ApiKey = common.MaskApiKey(mc.ApiKey)
	return mc, nil
}

// List 分页查询模型配置列表
func (s *ModelConfigService) List(ctx context.Context, page, pageSize int) ([]model.ModelConfig, int, error) {
	items, total, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	// ApiKey 脱敏
	for i := range items {
		items[i].ApiKey = common.MaskApiKey(items[i].ApiKey)
	}
	return items, total, nil
}

// Update 更新模型配置
func (s *ModelConfigService) Update(ctx context.Context, id string, req *model.UpdateModelConfigReq) error {
	// 检查是否存在
	mc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if mc == nil {
		return common.ErrModelConfigNotFound
	}

	if err := s.repo.Update(ctx, id, req); err != nil {
		slog.Error("更新模型配置失败", "id", id, "error", err)
		return err
	}

	// 配置变更，清理 LLM client 缓存
	s.llmClient.RemoveClient(id)

	slog.Info("更新模型配置成功", "id", id)
	return nil
}

// Delete 删除模型配置
func (s *ModelConfigService) Delete(ctx context.Context, id string) error {
	// 检查是否存在
	mc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if mc == nil {
		return common.ErrModelConfigNotFound
	}

	// 检查是否有关联任务
	hasRelated, err := s.repo.HasRelatedTasks(ctx, id)
	if err != nil {
		return err
	}
	if hasRelated {
		slog.Warn("删除模型配置被拒绝，存在关联任务", "id", id)
		return common.ErrModelConfigBoundByTask
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		slog.Error("删除模型配置失败", "id", id, "error", err)
		return err
	}

	// 配置删除，清理 LLM client 缓存
	s.llmClient.RemoveClient(id)

	slog.Info("删除模型配置成功", "id", id)
	return nil
}

// TestConnectivity 测试模型连通性
func (s *ModelConfigService) TestConnectivity(ctx context.Context, id string) (*model.TestModelConfigResp, error) {
	mc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if mc == nil {
		return nil, common.ErrModelConfigNotFound
	}

	latency, err := s.llmClient.TestConnectivity(ctx, mc.Id, llm.ModelConfigParam{
		ApiUrl:      mc.ApiUrl,
		ApiKey:      mc.ApiKey,
		ModelId:     mc.ModelId,
		Temperature: mc.Temperature,
		MaxTokens:   mc.MaxTokens,
	})
	if err != nil {
		return nil, err
	}

	slog.Info("模型连通性测试成功", "id", id, "modelId", mc.ModelId, "latency", latency)
	return &model.TestModelConfigResp{Latency: latency}, nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// generateID 生成 mc_{ModelId}_{uuid32} 格式的 ID
func generateID(modelId string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "mc_" + modelId + "_" + hex.EncodeToString(bytes), nil
}
