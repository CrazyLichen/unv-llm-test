package llm

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"

	"llm-test-server/internal/common"
)

// ──────────────────────────── 常量 ────────────────────────────

const ()

// ──────────────────────────── 结构体 ────────────────────────────

// LLMClient LLM 交互客户端
type LLMClient struct {
	factory *ClientFactory
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewLLMClient 创建 LLM 客户端实例
func NewLLMClient(factory *ClientFactory) *LLMClient {
	return &LLMClient{factory: factory}
}

// Analyze 调用大模型进行图片分析
func (c *LLMClient) Analyze(
	ctx context.Context,
	configID string,
	config ModelConfigParam,
	req LLMRequest,
	opts ...AnalyzeOption,
) (*LLMResponse, error) {
	// 参数校验
	if req.Prompt == "" {
		return nil, common.NewErrParamValidation("提示词不能为空")
	}
	if len(req.ImageBase64) == 0 {
		return nil, common.NewErrParamValidation("图片数据不能为空")
	}

	// 合并选项
	opt := mergeOptions(opts)

	// 构造请求选项
	reqOpts := []option.RequestOption{
		option.WithMaxRetries(opt.MaxRetries),
	}
	if opt.TimeoutUs > 0 {
		reqOpts = append(reqOpts, option.WithRequestTimeout(time.Duration(opt.TimeoutUs)*time.Microsecond))
	}

	// 获取 client
	client := c.factory.GetOrCreateClient(configID, config.ApiUrl, config.ApiKey, config.ModelId)

	// 构造消息内容（文本 + 图片）
	parts := make([]openai.ChatCompletionContentPartUnionParam, 0, 1+len(req.ImageBase64))
	parts = append(parts, openai.TextContentPart(req.Prompt))
	for _, img := range req.ImageBase64 {
		parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL: img,
		}))
	}

	slog.Info("LLM调用开始", "configID", configID, "model", config.ModelId, "promptLen", len(req.Prompt), "imageCount", len(req.ImageBase64))

	start := time.Now()

	// 发送请求
	completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:       shared.ChatModel(config.ModelId),
		Messages:    []openai.ChatCompletionMessageParamUnion{openai.UserMessage(parts)},
		Temperature: openai.Float(config.Temperature),
		MaxTokens:   openai.Int(int64(config.MaxTokens)),
	}, reqOpts...)

	elapsed := time.Since(start)

	if err != nil {
		slog.Error("LLM调用失败", "configID", configID, "model", config.ModelId, "error", err, "latencyMs", elapsed.Milliseconds())
		return nil, common.NewErrLLMCallFailed(err.Error())
	}

	// 解析响应
	if len(completion.Choices) == 0 {
		slog.Error("LLM返回空选择", "configID", configID, "model", config.ModelId)
		return nil, common.NewErrLLMCallFailed("模型返回空响应")
	}

	choice := completion.Choices[0]
	content := choice.Message.Content

	// 序列化完整响应为 RawJSON
	rawJSON, err := json.Marshal(completion)
	if err != nil {
		rawJSON = []byte("{}")
	}

	resp := &LLMResponse{
		Content:      content,
		Model:        completion.Model,
		FinishReason: string(choice.FinishReason),
		Usage: Usage{
			PromptTokens:     int(completion.Usage.PromptTokens),
			CompletionTokens: int(completion.Usage.CompletionTokens),
			TotalTokens:      int(completion.Usage.TotalTokens),
		},
		RawJSON: string(rawJSON),
	}

	slog.Info("LLM调用完成", "configID", configID, "model", resp.Model, "latencyMs", elapsed.Milliseconds(),
		"promptTokens", resp.Usage.PromptTokens, "completionTokens", resp.Usage.CompletionTokens)

	return resp, nil
}

// TestConnectivity 测试模型连通性，返回延迟（毫秒）
func (c *LLMClient) TestConnectivity(ctx context.Context, configID string, config ModelConfigParam) (int, error) {
	client := c.factory.GetOrCreateClient(configID, config.ApiUrl, config.ApiKey, config.ModelId)

	start := time.Now()
	_, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     shared.ChatModel(config.ModelId),
		Messages:  []openai.ChatCompletionMessageParamUnion{openai.UserMessage("Hello")},
		MaxTokens: openai.Int(5),
	})
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return 0, common.NewErrLLMCallFailed(err.Error())
	}

	return int(elapsed), nil
}

// RemoveClient 移除指定配置的缓存 client（代理 factory.RemoveClient）
func (c *LLMClient) RemoveClient(configID string) {
	c.factory.RemoveClient(configID)
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// mergeOptions 合并调用选项
func mergeOptions(opts []AnalyzeOption) AnalyzeOption {
	result := AnalyzeOption{TimeoutUs: 60000000} // 默认 60 秒（微秒）
	for _, o := range opts {
		if o.MaxRetries > 0 {
			result.MaxRetries = o.MaxRetries
		}
		if o.TimeoutUs > 0 {
			result.TimeoutUs = o.TimeoutUs
		}
	}
	return result
}
