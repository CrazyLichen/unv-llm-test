package llm

// ──────────────────────────── 结构体 ────────────────────────────

// AnalyzeOption 调用选项
type AnalyzeOption struct {
	// MaxRetries 重试次数（默认 0，不重试）
	MaxRetries int
	// TimeoutMs 单次请求超时（毫秒，0 表示使用默认值 60000ms）
	TimeoutMs int
}

// ──────────────────────────── 导出函数 ────────────────────────────

// WithRetry 设置重试次数
func WithRetry(n int) AnalyzeOption {
	return AnalyzeOption{MaxRetries: n}
}

// WithTimeoutMs 设置请求超时（毫秒）
func WithTimeoutMs(ms int) AnalyzeOption {
	return AnalyzeOption{TimeoutMs: ms}
}
