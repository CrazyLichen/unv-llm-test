package common

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"llm-test-server/internal/config"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// InitLogger 初始化全局日志 Logger
func InitLogger(cfg *config.LogConfig) error {
	// 解析日志级别
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return fmt.Errorf("解析日志级别失败: %w", err)
	}

	opts := &slog.HandlerOptions{Level: level}

	// 构建 Writer
	var writer io.Writer = os.Stdout
	if cfg.File != "" {
		// 确保日志文件目录存在
		dir := filepath.Dir(cfg.File)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("创建日志目录失败: %w", err)
			}
		}

		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("打开日志文件失败: %w", err)
		}

		writer = io.MultiWriter(os.Stdout, file)
	}

	// 根据 Format 创建 Handler
	var handler slog.Handler
	switch cfg.Format {
	case "text":
		handler = slog.NewTextHandler(writer, opts)
	default:
		handler = slog.NewJSONHandler(writer, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// parseLevel 将字符串解析为 slog.Level
func parseLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("未知的日志级别: %s", level)
	}
}
