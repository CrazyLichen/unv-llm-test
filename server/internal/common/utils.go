package common

import "time"

// ──────────────────────────── 常量 ────────────────────────────

const (
	// TimeFormat 统一时间格式
	TimeFormat = "2006-01-02 15:04:05"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// NowFormatted 返回当前时间的格式化字符串
func NowFormatted() string {
	return time.Now().Format(TimeFormat)
}
