package llm

// ──────────────────────────── 结构体 ────────────────────────────

// AnalyzeOption 调用选项
type AnalyzeOption struct {
	// MaxRetries 重试次数（默认 0，不重试）
	MaxRetries int
	// TimeoutUs 单次请求超时（微秒，0 表示使用默认值 60000000us 即 60 秒）
	TimeoutUs int
}

// ──────────────────────────── 导出函数 ────────────────────────────

// WithRetry 设置重试次数
func WithRetry(n int) AnalyzeOption {
	return AnalyzeOption{MaxRetries: n}
}

// WithTimeoutUs 设置请求超时（微秒）
func WithTimeoutUs(us int) AnalyzeOption {
	return AnalyzeOption{TimeoutUs: us}
}
