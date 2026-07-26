package llm

import "time"

// ──────────────────────────── 结构体 ────────────────────────────

// AnalyzeOption 调用选项
type AnalyzeOption struct {
	// MaxRetries 重试次数（默认 0，不重试）
	MaxRetries int
	// Timeout 单次请求超时（默认 60s）
	Timeout time.Duration
}

// ──────────────────────────── 导出函数 ────────────────────────────

// WithRetry 设置重试次数
func WithRetry(n int) AnalyzeOption {
	return AnalyzeOption{MaxRetries: n}
}

// WithTimeout 设置请求超时
func WithTimeout(d time.Duration) AnalyzeOption {
	return AnalyzeOption{Timeout: d}
}
