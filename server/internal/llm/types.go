package llm

// ──────────────────────────── 结构体 ────────────────────────────

// ModelConfigParam 模型配置参数（由调用方从 ModelConfig 实体提取）
type ModelConfigParam struct {
	// ApiUrl 大模型 API 地址
	ApiUrl string
	// ApiKey API 访问密钥
	ApiKey string
	// ModelId API 模型标识（如 gpt-4o、qwen-vl-max）
	ModelId string
	// Temperature 温度参数（0-2）
	Temperature float64
	// MaxTokens 最大生成 token 数
	MaxTokens int32
}

// LLMRequest LLM 分析请求
type LLMRequest struct {
	// Prompt 提示词（作为 User 消息发送）
	Prompt string
	// ImageBase64 图片 base64 data URI 列表（如 "data:image/jpeg;base64,..."）
	ImageBase64 []string
}

// LLMResponse LLM 分析响应
type LLMResponse struct {
	// Content 模型返回的文本内容（原始 content，调用方自行解析业务语义）
	Content string
	// Model 实际使用的模型名
	Model string
	// FinishReason 结束原因（stop/length 等）
	FinishReason string
	// Usage token 用量
	Usage Usage
	// RawJSON 完整 OpenAI 响应 JSON（用于存储 RawResponse 字段）
	RawJSON string
}

// Usage token 用量
type Usage struct {
	// PromptTokens 输入 token 数
	PromptTokens int
	// CompletionTokens 输出 token 数
	CompletionTokens int
	// TotalTokens 总 token 数
	TotalTokens int
}
