package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
		handler = &callerHandler{inner: handler, isJSON: false}
	default:
		handler = newOrderedJSONHandler(writer, opts)
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

// ──────────────────────────── callerHandler（text 格式用） ────────────────────────────

// callerHandler 包装 slog.Handler，自动为每条日志注入源码位置
type callerHandler struct {
	inner  slog.Handler
	isJSON bool
}

// Enabled 实现 slog.Handler 接口
func (h *callerHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle 实现 slog.Handler 接口，自动添加 caller 属性
func (h *callerHandler) Handle(ctx context.Context, r slog.Record) error {
	_, file, line, ok := runtime.Caller(3)
	if ok {
		r.AddAttrs(slog.String("caller", fmt.Sprintf("%s:%d", filepath.Base(file), line)))
	}
	return h.inner.Handle(ctx, r)
}

// WithAttrs 实现 slog.Handler 接口
func (h *callerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &callerHandler{inner: h.inner.WithAttrs(attrs), isJSON: h.isJSON}
}

// WithGroup 实现 slog.Handler 接口
func (h *callerHandler) WithGroup(name string) slog.Handler {
	return &callerHandler{inner: h.inner.WithGroup(name), isJSON: h.isJSON}
}

// ──────────────────────────── orderedJSONHandler ────────────────────────────

// orderedJSONHandler 自定义 JSON Handler，字段顺序为：
// time, level, caller, msg, 其余 attrs...
// 相比标准 slog.JSONHandler，仅调整了字段输出顺序并将 caller 前置
type orderedJSONHandler struct {
	w     io.Writer
	opts  *slog.HandlerOptions
	attrs []slog.Attr
	groups []string
}

// newOrderedJSONHandler 创建字段有序的 JSON Handler
func newOrderedJSONHandler(w io.Writer, opts *slog.HandlerOptions) *orderedJSONHandler {
	return &orderedJSONHandler{w: w, opts: opts}
}

func (h *orderedJSONHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.opts != nil && h.opts.Level != nil {
		return level >= h.opts.Level.Level()
	}
	return true
}

func (h *orderedJSONHandler) Handle(ctx context.Context, r slog.Record) error {
	// 获取调用位置
	_, file, line, ok := runtime.Caller(3)
	caller := ""
	if ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	// 按目标顺序收集字段
	entries := make([]jsonEntry, 0, 4+r.NumAttrs()+len(h.attrs))

	entries = append(entries,
		jsonEntry{Key: "time", Value: r.Time.Format("2006-01-02T15:04:05.000000000Z07:00")},
		jsonEntry{Key: "level", Value: r.Level.String()},
	)
	if caller != "" {
		entries = append(entries, jsonEntry{Key: "caller", Value: caller})
	}
	entries = append(entries, jsonEntry{Key: "msg", Value: r.Message})

	// 写入 handler 上绑定的 attrs
	for _, a := range h.attrs {
		entries = append(entries, attrToEntry(a)...)
	}

	// 写入本次 Record 的 attrs
	r.Attrs(func(a slog.Attr) bool {
		entries = append(entries, attrToEntry(a)...)
		return true
	})

	// 序列化
	var buf strings.Builder
	buf.WriteByte('{')
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(e.Key)
		buf.Write(kb)
		buf.WriteByte(':')
		vb, _ := json.Marshal(e.Value)
		buf.Write(vb)
	}
	buf.WriteByte('}')
	buf.WriteByte('\n')

	_, err := fmt.Fprint(h.w, buf.String())
	return err
}

func (h *orderedJSONHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &orderedJSONHandler{
		w:      h.w,
		opts:   h.opts,
		attrs:  append(h.attrs, attrs...),
		groups: h.groups,
	}
}

func (h *orderedJSONHandler) WithGroup(name string) slog.Handler {
	return &orderedJSONHandler{
		w:      h.w,
		opts:   h.opts,
		attrs:  h.attrs,
		groups: append(h.groups, name),
	}
}

// ──────────────────────────── JSON 序列化辅助 ────────────────────────────

type jsonEntry struct {
	Key   string
	Value interface{}
}

func attrToEntry(a slog.Attr) []jsonEntry {
	if a.Key == "" {
		return nil
	}
	// 常见类型直接转换，避免 json.Marshal 产生多余转义
	switch v := a.Value.Any().(type) {
	case string:
		return []jsonEntry{{Key: a.Key, Value: v}}
	case int:
		return []jsonEntry{{Key: a.Key, Value: v}}
	case int64:
		return []jsonEntry{{Key: a.Key, Value: v}}
	case int32:
		return []jsonEntry{{Key: a.Key, Value: v}}
	case float64:
		return []jsonEntry{{Key: a.Key, Value: v}}
	case bool:
		return []jsonEntry{{Key: a.Key, Value: v}}
	case nil:
		return []jsonEntry{{Key: a.Key, Value: nil}}
	default:
		// 其他类型（如 json.RawMessage、结构体等）交给 json.Marshal
		return []jsonEntry{{Key: a.Key, Value: v}}
	}
}
