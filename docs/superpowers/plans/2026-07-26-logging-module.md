# 日志模块实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使用 log/slog 增加日志模块，配置驱动，控制台+文件双输出，补充现有代码日志

**Architecture:** 在 common 包新增 logger.go 封装 slog 初始化，读取 config.yaml 的 log 配置（level/format/file），设置全局默认 Logger，然后逐层补充日志

**Tech Stack:** Go log/slog（标准库，零依赖）

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `server/internal/common/logger.go` | 新增 | slog 初始化：解析配置、创建 Handler、设置全局 Logger |
| `server/internal/config/config.go` | 修改 | LogConfig 新增 File 字段，Load 增加默认值和环境变量覆盖 |
| `server/config.yaml` | 修改 | log 段新增 file 字段 |
| `server/cmd/server/main.go` | 修改 | 调用 InitLogger，替换 log 为 slog |
| `server/internal/repository/db.go` | 修改 | 补充 DB 初始化日志 |
| `server/internal/repository/model_config_repo.go` | 修改 | 补充 CRUD 操作日志 |
| `server/internal/service/model_config_svc.go` | 修改 | 补充业务逻辑日志 |
| `server/internal/controller/model_config_ctrl.go` | 修改 | 补充请求入口和参数校验日志 |

---

### Task 1: 配置扩展 — LogConfig 新增 File 字段

**Files:**
- Modify: `server/internal/config/config.go`
- Modify: `server/config.yaml`

- [ ] **Step 1: 修改 LogConfig 结构体，新增 File 字段**

在 `server/internal/config/config.go` 中，将 LogConfig 改为：

```go
// LogConfig 日志配置
type LogConfig struct {
	// Level 日志级别
	Level string `yaml:"level"`
	// Format 日志格式（json/text）
	Format string `yaml:"format"`
	// File 日志文件路径，为空则仅输出到控制台
	File string `yaml:"file"`
}
```

在 Load 函数的默认值设置中添加：

```go
if cfg.Log.File == "" {
    cfg.Log.File = ""
}
```

在环境变量覆盖中添加：

```go
if v := os.Getenv("LOG_FILE"); v != "" {
    cfg.Log.File = v
}
```

- [ ] **Step 2: 修改 config.yaml，新增 file 字段**

```yaml
log:
  level: info
  format: json
  file: "./data/app.log"
```

- [ ] **Step 3: 验证编译**

Run: `cd "c:/Users/李陈/Desktop/大模型测试/server" && go build ./...`
Expected: 无错误输出

---

### Task 2: 新增 common/logger.go — slog 初始化

**Files:**
- Create: `server/internal/common/logger.go`

- [ ] **Step 1: 创建 logger.go**

```go
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
```

- [ ] **Step 2: 验证编译**

Run: `cd "c:/Users/李陈/Desktop/大模型测试/server" && go build ./...`
Expected: 无错误输出

---

### Task 3: 修改 main.go — 集成日志初始化

**Files:**
- Modify: `server/cmd/server/main.go`

- [ ] **Step 1: 重写 main.go**

```go
package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/common"
	"llm-test-server/internal/config"
	"llm-test-server/internal/controller"
	"llm-test-server/internal/repository"
	"llm-test-server/internal/service"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// main 程序入口
func main() {
	// 加载配置
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志（在 DB 之前，日志初始化失败仍用标准 log）
	if err := common.InitLogger(&cfg.Log); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	slog.Info("配置加载成功", "port", cfg.Server.Port, "mode", cfg.Server.Mode)

	// 初始化数据库
	db, err := repository.InitDB(&cfg.Database)
	if err != nil {
		slog.Error("初始化数据库失败", "error", err)
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 初始化各层
	mcRepo := repository.NewModelConfigRepo(db)
	mcSvc := service.NewModelConfigService(mcRepo)
	mcCtrl := controller.NewModelConfigController(mcSvc)

	// 注册路由
	r := gin.Default()
	controller.SetupRouter(r, mcCtrl)

	// 启动服务
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	slog.Info("服务启动", "addr", addr)
	if err := r.Run(addr); err != nil {
		slog.Error("服务启动失败", "error", err)
		log.Fatalf("服务启动失败: %v", err)
	}
}
```

- [ ] **Step 2: 验证编译**

Run: `cd "c:/Users/李陈/Desktop/大模型测试/server" && go build ./...`
Expected: 无错误输出

---

### Task 4: 补充 repository 层日志

**Files:**
- Modify: `server/internal/repository/db.go`
- Modify: `server/internal/repository/model_config_repo.go`

- [ ] **Step 1: 修改 db.go，补充日志**

在 `db.go` 的 import 中添加 `"log/slog"`，在 InitDB 函数的关键位置添加日志：

```go
package repository

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"llm-test-server/internal/config"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// InitDB 初始化数据库连接并建表
func InitDB(cfg *config.DatabaseConfig) (*sql.DB, error) {
	// 确保 DSN 的父目录存在
	dir := filepath.Dir(cfg.DSN)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// SQLite 写入是单写锁，限制最大打开连接数
	db.SetMaxOpenConns(1)

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("创建数据表失败: %w", err)
	}

	slog.Info("数据库初始化成功", "driver", cfg.Driver, "dsn", cfg.DSN)
	return db, nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// createTables 执行建表 SQL
func createTables(db *sql.DB) error {
	const sql = `
	CREATE TABLE IF NOT EXISTS model_configs (
		id          TEXT PRIMARY KEY,
		model_name  TEXT NOT NULL,
		model_id    TEXT NOT NULL,
		api_url     TEXT NOT NULL,
		api_key     TEXT NOT NULL,
		temperature REAL NOT NULL DEFAULT 0.7,
		max_tokens  INTEGER NOT NULL DEFAULT 4096,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	);`

	_, err := db.Exec(sql)
	return err
}
```

- [ ] **Step 2: 修改 model_config_repo.go，补充日志**

在 import 中添加 `"log/slog"`，在每个方法的关键位置添加日志：

```go
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
)

// ──────────────────────────── 结构体 ────────────────────────────

// ModelConfigRepo 模型配置数据访问层
type ModelConfigRepo struct {
	db *sql.DB
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewModelConfigRepo 创建模型配置仓储实例
func NewModelConfigRepo(db *sql.DB) *ModelConfigRepo {
	return &ModelConfigRepo{db: db}
}

// Create 插入一条模型配置
func (r *ModelConfigRepo) Create(ctx context.Context, mc *model.ModelConfig) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO model_configs (id, model_name, model_id, api_url, api_key, temperature, max_tokens, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mc.Id, mc.ModelName, mc.ModelId, mc.ApiUrl, mc.ApiKey, mc.Temperature, mc.MaxTokens, mc.CreatedAt, mc.UpdatedAt,
	)
	if err != nil {
		slog.Error("插入模型配置失败", "id", mc.Id, "error", err)
		return fmt.Errorf("插入模型配置失败: %w", err)
	}
	slog.Info("插入模型配置成功", "id", mc.Id, "modelId", mc.ModelId)
	return nil
}

// GetByID 根据 ID 查询单条模型配置
func (r *ModelConfigRepo) GetByID(ctx context.Context, id string) (*model.ModelConfig, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, model_name, model_id, api_url, api_key, temperature, max_tokens, created_at, updated_at
		 FROM model_configs WHERE id = ?`, id)

	var mc model.ModelConfig
	err := row.Scan(&mc.Id, &mc.ModelName, &mc.ModelId, &mc.ApiUrl, &mc.ApiKey, &mc.Temperature, &mc.MaxTokens, &mc.CreatedAt, &mc.UpdatedAt)
	if err == sql.ErrNoRows {
		slog.Warn("模型配置不存在", "id", id)
		return nil, nil
	}
	if err != nil {
		slog.Error("按ID查询模型配置失败", "id", id, "error", err)
		return nil, fmt.Errorf("按ID查询模型配置失败: %w", err)
	}
	return &mc, nil
}

// List 分页查询模型配置列表，返回列表和总数
func (r *ModelConfigRepo) List(ctx context.Context, page, pageSize int) ([]model.ModelConfig, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_configs`).Scan(&total)
	if err != nil {
		slog.Error("统计模型配置数量失败", "error", err)
		return nil, 0, fmt.Errorf("统计模型配置数量失败: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, model_name, model_id, api_url, api_key, temperature, max_tokens, created_at, updated_at
		 FROM model_configs ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, offset)
	if err != nil {
		slog.Error("查询模型配置列表失败", "page", page, "pageSize", pageSize, "error", err)
		return nil, 0, fmt.Errorf("查询模型配置列表失败: %w", err)
	}
	defer rows.Close()

	var items []model.ModelConfig
	for rows.Next() {
		var mc model.ModelConfig
		if err := rows.Scan(&mc.Id, &mc.ModelName, &mc.ModelId, &mc.ApiUrl, &mc.ApiKey, &mc.Temperature, &mc.MaxTokens, &mc.CreatedAt, &mc.UpdatedAt); err != nil {
			slog.Error("扫描模型配置记录失败", "error", err)
			return nil, 0, fmt.Errorf("扫描模型配置记录失败: %w", err)
		}
		items = append(items, mc)
	}

	slog.Info("查询模型配置列表", "page", page, "pageSize", pageSize, "total", total, "count", len(items))
	return items, total, nil
}

// Update 更新模型配置（仅更新非 nil 字段）
func (r *ModelConfigRepo) Update(ctx context.Context, id string, req *model.UpdateModelConfigReq) error {
	var sets []string
	var args []interface{}

	if req.ModelName != nil {
		sets = append(sets, "model_name = ?")
		args = append(args, *req.ModelName)
	}
	if req.ModelId != nil {
		sets = append(sets, "model_id = ?")
		args = append(args, *req.ModelId)
	}
	if req.ApiUrl != nil {
		sets = append(sets, "api_url = ?")
		args = append(args, *req.ApiUrl)
	}
	if req.ApiKey != nil {
		sets = append(sets, "api_key = ?")
		args = append(args, *req.ApiKey)
	}
	if req.Temperature != nil {
		sets = append(sets, "temperature = ?")
		args = append(args, *req.Temperature)
	}
	if req.MaxTokens != nil {
		sets = append(sets, "max_tokens = ?")
		args = append(args, *req.MaxTokens)
	}

	// 无字段需要更新
	if len(sets) == 0 {
		slog.Warn("更新模型配置无字段变更", "id", id)
		return nil
	}

	sets = append(sets, "updated_at = ?")
	args = append(args, common.NowFormatted())
	args = append(args, id)

	query := fmt.Sprintf("UPDATE model_configs SET %s WHERE id = ?", joinSets(sets))
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		slog.Error("更新模型配置失败", "id", id, "error", err)
		return fmt.Errorf("更新模型配置失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	slog.Info("更新模型配置成功", "id", id, "fields", len(sets)-1)
	return nil
}

// Delete 删除模型配置
func (r *ModelConfigRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM model_configs WHERE id = ?`, id)
	if err != nil {
		slog.Error("删除模型配置失败", "id", id, "error", err)
		return fmt.Errorf("删除模型配置失败: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	slog.Info("删除模型配置成功", "id", id)
	return nil
}

// HasRelatedTasks 检查是否有关联任务（tasks 表尚未创建，预留接口）
func (r *ModelConfigRepo) HasRelatedTasks(ctx context.Context, modelConfigId string) (bool, error) {
	// tasks 表尚未创建，返回 false
	// 后续创建 tasks 表后实现：SELECT COUNT(*) FROM tasks WHERE model_config_id = ?
	return false, nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// joinSets 拼接 SET 子句
func joinSets(sets []string) string {
	result := sets[0]
	for i := 1; i < len(sets); i++ {
		result += ", " + sets[i]
	}
	return result
}
```

- [ ] **Step 3: 验证编译**

Run: `cd "c:/Users/李陈/Desktop/大模型测试/server" && go build ./...`
Expected: 无错误输出

---

### Task 5: 补充 service 层日志

**Files:**
- Modify: `server/internal/service/model_config_svc.go`

- [ ] **Step 1: 修改 model_config_svc.go，补充日志**

在 import 中添加 `"log/slog"`，在各方法关键位置添加日志。完整文件内容：

```go
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
```

- [ ] **Step 2: 验证编译**

Run: `cd "c:/Users/李陈/Desktop/大模型测试/server" && go build ./...`
Expected: 无错误输出

---

### Task 6: 补充 controller 层日志

**Files:**
- Modify: `server/internal/controller/model_config_ctrl.go`

- [ ] **Step 1: 修改 model_config_ctrl.go，补充日志**

在 import 中添加 `"log/slog"`，在每个 handler 入口和参数校验失败处添加日志：

```go
package controller

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
	"llm-test-server/internal/service"
)

// ──────────────────────────── 结构体 ────────────────────────────

// ModelConfigController 模型配置 HTTP 处理器
type ModelConfigController struct {
	svc *service.ModelConfigService
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewModelConfigController 创建模型配置控制器实例
func NewModelConfigController(svc *service.ModelConfigService) *ModelConfigController {
	return &ModelConfigController{svc: svc}
}

// Create 创建模型配置
func (ctrl *ModelConfigController) Create(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	var req model.CreateModelConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.Create(c.Request.Context(), &req); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// List 获取模型配置列表
func (ctrl *ModelConfigController) List(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	// 按 ID 精确查询
	if id := c.Query("Id"); id != "" {
		mc, err := ctrl.svc.GetByID(c.Request.Context(), id)
		if err != nil {
			handleError(c, err)
			return
		}
		common.OK(c, common.PageData{
			Total:    1,
			Page:     1,
			PageSize: 1,
			Items:    []model.ModelConfig{*mc},
		})
		return
	}

	// 分页查询
	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	items, total, err := ctrl.svc.List(c.Request.Context(), page, pageSize)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, common.PageData{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	})
}

// Update 更新模型配置
func (ctrl *ModelConfigController) Update(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.UpdateModelConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.Update(c.Request.Context(), id, &req); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// Delete 删除模型配置
func (ctrl *ModelConfigController) Delete(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	if err := ctrl.svc.Delete(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// Test 测试模型连通性
func (ctrl *ModelConfigController) Test(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	result, err := ctrl.svc.TestConnectivity(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, result)
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// handleError 统一错误处理
func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*service.AppError); ok {
		httpStatus := http.StatusBadRequest
		if appErr.Code >= 50000 {
			httpStatus = http.StatusInternalServerError
		}
		slog.Error("请求处理失败", "errorCode", appErr.Code, "errorMsg", appErr.Msg, "path", c.Request.URL.Path)
		common.Fail(c, httpStatus, appErr.Code, appErr.Msg)
		return
	}
	slog.Error("请求处理未知错误", "error", err.Error(), "path", c.Request.URL.Path)
	common.Fail(c, http.StatusInternalServerError, common.ErrModelCallFailed, err.Error())
}
```

- [ ] **Step 2: 验证编译**

Run: `cd "c:/Users/李陈/Desktop/大模型测试/server" && go build ./...`
Expected: 无错误输出

---

### Task 7: 冒烟测试

**Files:** 无新文件

- [ ] **Step 1: 清理旧数据并启动服务**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/server"
rm -f data/llm-test.db data/app.log
go run ./cmd/server/
```
Expected: 控制台输出 JSON 格式日志，包含 `"msg":"配置加载成功"` 和 `"msg":"数据库初始化成功"` 和 `"msg":"服务启动"`

- [ ] **Step 2: 测试创建模型配置**

```bash
curl -s -X POST http://localhost:8080/api/model-configs -H "Content-Type: application/json" -d '{"ModelName":"GPT-4o 测试","ModelId":"gpt-4o","ApiUrl":"https://api.openai.com/v1/chat/completions","ApiKey":"sk-testkey1234567890abcdef","Temperature":0.7,"MaxTokens":4096}'
```
Expected: `{"ErrorCode":0,"ErrorMsg":"","Data":null}`，控制台输出创建日志

- [ ] **Step 3: 检查日志文件**

```bash
cat data/app.log
```
Expected: 包含 JSON 格式日志记录

- [ ] **Step 4: 停止服务**
