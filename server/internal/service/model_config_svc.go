package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)

// ──────────────────────────── 结构体 ────────────────────────────

// ModelConfigService 模型配置业务逻辑层
type ModelConfigService struct {
	repo *repository.ModelConfigRepo
}

// AppError 业务错误，携带错误码和消息
type AppError struct {
	// Code 错误码
	Code int
	// Msg 错误消息
	Msg string
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewModelConfigService 创建模型配置服务实例
func NewModelConfigService(repo *repository.ModelConfigRepo) *ModelConfigService {
	return &ModelConfigService{repo: repo}
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
		return nil, &AppError{Code: common.ErrModelConfigNotFound, Msg: "模型配置不存在"}
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
		return &AppError{Code: common.ErrModelConfigNotFound, Msg: "模型配置不存在"}
	}

	if err := s.repo.Update(ctx, id, req); err != nil {
		slog.Error("更新模型配置失败", "id", id, "error", err)
		return err
	}

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
		return &AppError{Code: common.ErrModelConfigNotFound, Msg: "模型配置不存在"}
	}

	// 检查是否有关联任务
	hasRelated, err := s.repo.HasRelatedTasks(ctx, id)
	if err != nil {
		return err
	}
	if hasRelated {
		slog.Warn("删除模型配置被拒绝，存在关联任务", "id", id)
		return &AppError{Code: common.ErrTaskStatusConflict, Msg: "该模型配置下存在关联任务，无法删除"}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		slog.Error("删除模型配置失败", "id", id, "error", err)
		return err
	}

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
		return nil, &AppError{Code: common.ErrModelConfigNotFound, Msg: "模型配置不存在"}
	}

	// 构造 OpenAI 兼容请求体
	body := fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"Hello"}],"max_tokens":5}`, mc.ModelId)

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mc.ApiUrl, strings.NewReader(body))
	if err != nil {
		slog.Error("创建连通性测试请求失败", "id", id, "error", err)
		return nil, &AppError{Code: common.ErrModelCallFailed, Msg: fmt.Sprintf("创建请求失败: %s", err.Error())}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mc.ApiKey)

	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		slog.Error("模型连通性测试失败", "id", id, "modelId", mc.ModelId, "error", err)
		return nil, &AppError{Code: common.ErrModelCallFailed, Msg: fmt.Sprintf("连接失败: %s", err.Error())}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("模型连通性测试返回错误", "id", id, "modelId", mc.ModelId, "statusCode", resp.StatusCode, "latency", elapsed)
		return nil, &AppError{Code: common.ErrModelCallFailed, Msg: fmt.Sprintf("模型返回错误(HTTP %d): %s", resp.StatusCode, string(respBody))}
	}

	slog.Info("模型连通性测试成功", "id", id, "modelId", mc.ModelId, "latency", elapsed)
	return &model.TestModelConfigResp{Latency: int(elapsed)}, nil
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	return e.Msg
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
