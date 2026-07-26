# 后台框架与模型配置实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭建 Go 1.24 + Gin + SQLite 后台框架，实现模型配置 CRUD + 连通性测试 5 个 API

**Architecture:** 经典分层 controller→service→repository→DB，common 包提供统一响应/错误码，config 包管理 YAML 配置，启动时自动建表

**Tech Stack:** Go 1.24, Gin, modernc.org/sqlite (纯Go驱动), gopkg.in/yaml.v3

---

## File Structure

| File | Responsibility |
|------|---------------|
| `llm-test-server/go.mod` | 模块定义与依赖 |
| `llm-test-server/config.yaml` | 配置文件模板 |
| `llm-test-server/cmd/server/main.go` | 入口：加载配置→初始化DB→注册路由→启动 |
| `llm-test-server/internal/config/config.go` | 配置结构体+加载逻辑（YAML+环境变量） |
| `llm-test-server/internal/common/response.go` | 统一响应结构、分页结构、辅助函数 |
| `llm-test-server/internal/common/errorcode.go` | 错误码常量 |
| `llm-test-server/internal/model/model_config.go` | ModelConfig 结构体、创建/更新 DTO |
| `llm-test-server/internal/repository/model_config_repo.go` | ModelConfig DB CRUD |
| `llm-test-server/internal/repository/db.go` | DB 初始化、建表 |
| `llm-test-server/internal/service/model_config_svc.go` | ModelConfig 业务逻辑（校验、删除检查、连通测试） |
| `llm-test-server/internal/controller/model_config_ctrl.go` | ModelConfig HTTP handler |
| `llm-test-server/internal/controller/router.go` | 总路由注册 |

---

### Task 1: 项目初始化与模块定义

**Files:**
- Create: `llm-test-server/go.mod`
- Create: `llm-test-server/config.yaml`

- [ ] **Step 1: 创建项目目录并初始化 Go module**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试"
mkdir -p llm-test-server/cmd/server
mkdir -p llm-test-server/internal/config
mkdir -p llm-test-server/internal/common
mkdir -p llm-test-server/internal/model
mkdir -p llm-test-server/internal/repository
mkdir -p llm-test-server/internal/service
mkdir -p llm-test-server/internal/controller
```

- [ ] **Step 2: 初始化 go.mod**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go mod init llm-test-server
```

- [ ] **Step 3: 创建 config.yaml**

```yaml
server:
  port: 8080
  mode: release        # debug / release

database:
  driver: sqlite
  dsn: "./data/llm-test.db"

log:
  level: info
  format: json
```

- [ ] **Step 4: Commit**

```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && git add -A && git commit -m "chore: init project structure with go.mod and config.yaml"
```

---

### Task 2: 通用包 — 错误码与统一响应

**Files:**
- Create: `llm-test-server/internal/common/errorcode.go`
- Create: `llm-test-server/internal/common/response.go`

- [ ] **Step 1: 创建 errorcode.go**

```go
package common

// 错误码常量，与接口文档 1.4 节对应
const (
	Success               = 0
	ErrParamInvalid       = 40001
	ErrTaskNotFound       = 40002
	ErrImageNotFound      = 40003
	ErrPathNotFound       = 40004
	ErrTaskStatusConflict = 40005
	ErrModelConfigNotFound = 40006
	ErrModelCallFailed    = 50001
	ErrVideoFrameFailed   = 50002
)
```

- [ ] **Step 2: 创建 response.go**

```go
package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	ErrorCode int         `json:"ErrorCode"`
	ErrorMsg  string      `json:"ErrorMsg"`
	Data      interface{} `json:"Data"`
}

// PageData 分页响应结构
type PageData struct {
	Total    int         `json:"Total"`
	Page     int         `json:"Page"`
	PageSize int         `json:"PageSize"`
	Items    interface{} `json:"Items"`
}

// OK 返回成功响应，Data 可为 nil
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		ErrorCode: Success,
		ErrorMsg:  "",
		Data:      data,
	})
}

// Fail 返回失败响应
func Fail(c *gin.Context, httpStatus int, errorCode int, errorMsg string) {
	c.JSON(httpStatus, Response{
		ErrorCode: errorCode,
		ErrorMsg:  errorMsg,
		Data:      nil,
	})
}

// MaskApiKey 对 API Key 进行脱敏，保留前3后4位，中间用 **** 替代
func MaskApiKey(key string) string {
	if len(key) <= 7 {
		return "****"
	}
	return key[:3] + "****" + key[len(key)-4:]
}
```

- [ ] **Step 3: 下载 Gin 依赖**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go get github.com/gin-gonic/gin@latest
```

- [ ] **Step 4: 验证编译**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go build ./internal/common/...
```
Expected: 无错误输出

- [ ] **Step 5: Commit**

```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && git add -A && git commit -m "feat: add common package with error codes and unified response"
```

---

### Task 3: 配置加载模块

**Files:**
- Create: `llm-test-server/internal/config/config.go`

- [ ] **Step 1: 创建 config.go**

```go
package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Log      LogConfig      `yaml:"log"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Load 从 YAML 文件加载配置，环境变量可覆盖
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// 设置默认值
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "release"
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "sqlite"
	}
	if cfg.Database.DSN == "" {
		cfg.Database.DSN = "./data/llm-test.db"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}

	// 环境变量覆盖
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("SERVER_MODE"); v != "" {
		cfg.Server.Mode = v
	}
	if v := os.Getenv("DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}

	return &cfg, nil
}
```

- [ ] **Step 2: 下载 YAML 依赖**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go get gopkg.in/yaml.v3@latest
```

- [ ] **Step 3: 验证编译**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go build ./internal/config/...
```
Expected: 无错误输出

- [ ] **Step 4: Commit**

```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && git add -A && git commit -m "feat: add config package with YAML loading and env override"
```

---

### Task 4: 数据模型定义

**Files:**
- Create: `llm-test-server/internal/model/model_config.go`

- [ ] **Step 1: 创建 model_config.go**

```go
package model

import "time"

// ModelConfig 模型配置实体（对应 model_configs 表）
type ModelConfig struct {
	Id          string    `json:"Id"`
	ModelName   string    `json:"ModelName"`
	ModelId     string    `json:"ModelId"`
	ApiUrl      string    `json:"ApiUrl"`
	ApiKey      string    `json:"ApiKey"`
	Temperature float64   `json:"Temperature"`
	MaxTokens   int32     `json:"MaxTokens"`
	CreatedAt   string    `json:"CreatedAt"`
	UpdatedAt   string    `json:"UpdatedAt"`
}

// CreateModelConfigReq 创建模型配置请求
type CreateModelConfigReq struct {
	ModelName   string  `json:"ModelName" binding:"required"`
	ModelId     string  `json:"ModelId" binding:"required"`
	ApiUrl      string  `json:"ApiUrl" binding:"required"`
	ApiKey      string  `json:"ApiKey" binding:"required"`
	Temperature *float64 `json:"Temperature"`
	MaxTokens   *int32   `json:"MaxTokens"`
}

// UpdateModelConfigReq 更新模型配置请求（指针类型区分未传与零值）
type UpdateModelConfigReq struct {
	ModelName   *string  `json:"ModelName"`
	ModelId     *string  `json:"ModelId"`
	ApiUrl      *string  `json:"ApiUrl"`
	ApiKey      *string  `json:"ApiKey"`
	Temperature *float64 `json:"Temperature"`
	MaxTokens   *int32   `json:"MaxTokens"`
}

// TestModelConfigResp 连通性测试响应
type TestModelConfigResp struct {
	Latency int `json:"Latency"`
}

// TimeFormat 统一时间格式
const TimeFormat = "2006-01-02 15:04:05"

// NowFormatted 返回当前时间的格式化字符串
func NowFormatted() string {
	return time.Now().Format(TimeFormat)
}
```

- [ ] **Step 2: 验证编译**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go build ./internal/model/...
```
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && git add -A && git commit -m "feat: add ModelConfig data model and DTOs"
```

---

### Task 5: 数据库初始化与 Repository 层

**Files:**
- Create: `llm-test-server/internal/repository/db.go`
- Create: `llm-test-server/internal/repository/model_config_repo.go`

- [ ] **Step 1: 下载 SQLite 驱动依赖**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go get modernc.org/sqlite@latest
```

- [ ] **Step 2: 创建 db.go — 数据库初始化与建表**

```go
package repository

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"llm-test-server/internal/config"
)

// InitDB 初始化数据库连接并建表
func InitDB(cfg *config.DatabaseConfig) (*sql.DB, error) {
	// 确保 DSN 的父目录存在
	dir := filepath.Dir(cfg.DSN)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}

	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite 写入是单写锁，限制最大打开连接数
	db.SetMaxOpenConns(1)

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return db, nil
}

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

- [ ] **Step 3: 创建 model_config_repo.go — ModelConfig 数据访问**

```go
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"llm-test-server/internal/model"
)

type ModelConfigRepo struct {
	db *sql.DB
}

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
		return fmt.Errorf("insert model_config: %w", err)
	}
	return nil
}

// GetByID 根据 ID 查询单条
func (r *ModelConfigRepo) GetByID(ctx context.Context, id string) (*model.ModelConfig, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, model_name, model_id, api_url, api_key, temperature, max_tokens, created_at, updated_at
		 FROM model_configs WHERE id = ?`, id)

	var mc model.ModelConfig
	err := row.Scan(&mc.Id, &mc.ModelName, &mc.ModelId, &mc.ApiUrl, &mc.ApiKey, &mc.Temperature, &mc.MaxTokens, &mc.CreatedAt, &mc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query model_config by id: %w", err)
	}
	return &mc, nil
}

// List 分页查询，返回列表和总数
func (r *ModelConfigRepo) List(ctx context.Context, page, pageSize int) ([]model.ModelConfig, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_configs`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count model_configs: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, model_name, model_id, api_url, api_key, temperature, max_tokens, created_at, updated_at
		 FROM model_configs ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query model_configs: %w", err)
	}
	defer rows.Close()

	var items []model.ModelConfig
	for rows.Next() {
		var mc model.ModelConfig
		if err := rows.Scan(&mc.Id, &mc.ModelName, &mc.ModelId, &mc.ApiUrl, &mc.ApiKey, &mc.Temperature, &mc.MaxTokens, &mc.CreatedAt, &mc.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan model_config: %w", err)
		}
		items = append(items, mc)
	}
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

	if len(sets) == 0 {
		return nil // 无字段需要更新
	}

	sets = append(sets, "updated_at = ?")
	args = append(args, model.NowFormatted())
	args = append(args, id)

	query := fmt.Sprintf("UPDATE model_configs SET %s WHERE id = ?", joinSets(sets))
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update model_config: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete 删除模型配置
func (r *ModelConfigRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM model_configs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete model_config: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// HasRelatedTasks 检查是否有关联任务（tasks 表尚未创建，预留接口）
func (r *ModelConfigRepo) HasRelatedTasks(ctx context.Context, modelConfigId string) (bool, error) {
	// tasks 表尚未创建，返回 false
	// 后续创建 tasks 表后实现：SELECT COUNT(*) FROM tasks WHERE model_config_id = ?
	return false, nil
}

// joinSets 拼接 SET 子句
func joinSets(sets []string) string {
	result := sets[0]
	for i := 1; i < len(sets); i++ {
		result += ", " + sets[i]
	}
	return result
}
```

- [ ] **Step 4: 验证编译**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go build ./internal/repository/...
```
Expected: 无错误输出

- [ ] **Step 5: Commit**

```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && git add -A && git commit -m "feat: add database init and ModelConfig repository layer"
```

---

### Task 6: Service 层 — 模型配置业务逻辑

**Files:**
- Create: `llm-test-server/internal/service/model_config_svc.go`

- [ ] **Step 1: 创建 model_config_svc.go**

```go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)

type ModelConfigService struct {
	repo *repository.ModelConfigRepo
}

func NewModelConfigService(repo *repository.ModelConfigRepo) *ModelConfigService {
	return &ModelConfigService{repo: repo}
}

// Create 创建模型配置
func (s *ModelConfigService) Create(ctx context.Context, req *model.CreateModelConfigReq) error {
	id, err := generateID(req.ModelId)
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}

	temp := 0.7
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	maxTokens := int32(4096)
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	now := model.NowFormatted()
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

	return s.repo.Create(ctx, mc)
}

// GetByID 按 ID 查询
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

// List 分页查询
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

	err = s.repo.Update(ctx, id, req)
	if err != nil {
		return err
	}
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
		return &AppError{Code: common.ErrTaskStatusConflict, Msg: "该模型配置下存在关联任务，无法删除"}
	}

	return s.repo.Delete(ctx, id)
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
		return nil, &AppError{Code: common.ErrModelCallFailed, Msg: fmt.Sprintf("创建请求失败: %s", err.Error())}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mc.ApiKey)

	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return nil, &AppError{Code: common.ErrModelCallFailed, Msg: fmt.Sprintf("连接失败: %s", err.Error())}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &AppError{Code: common.ErrModelCallFailed, Msg: fmt.Sprintf("模型返回错误(HTTP %d): %s", resp.StatusCode, string(respBody))}
	}

	return &model.TestModelConfigResp{Latency: int(elapsed)}, nil
}

// generateID 生成 mc_{ModelId}_{uuid32} 格式的 ID
func generateID(modelId string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "mc_" + modelId + "_" + hex.EncodeToString(bytes), nil
}

// AppError 业务错误，携带错误码和消息
type AppError struct {
	Code int
	Msg  string
}

func (e *AppError) Error() string {
	return e.Msg
}
```

- [ ] **Step 2: 验证编译**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go build ./internal/service/...
```
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && git add -A && git commit -m "feat: add ModelConfig service with CRUD and connectivity test"
```

---

### Task 7: Controller 层与路由注册

**Files:**
- Create: `llm-test-server/internal/controller/model_config_ctrl.go`
- Create: `llm-test-server/internal/controller/router.go`

- [ ] **Step 1: 创建 model_config_ctrl.go**

```go
package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
	"llm-test-server/internal/service"
)

type ModelConfigController struct {
	svc *service.ModelConfigService
}

func NewModelConfigController(svc *service.ModelConfigService) *ModelConfigController {
	return &ModelConfigController{svc: svc}
}

// Create 创建模型配置
func (ctrl *ModelConfigController) Create(c *gin.Context) {
	var req model.CreateModelConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
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

	var req model.UpdateModelConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
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

	if err := ctrl.svc.Delete(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// Test 测试模型连通性
func (ctrl *ModelConfigController) Test(c *gin.Context) {
	id := c.Param("id")

	result, err := ctrl.svc.TestConnectivity(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, result)
}

// handleError 统一错误处理
func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*service.AppError); ok {
		httpStatus := http.StatusBadRequest
		if appErr.Code >= 50000 {
			httpStatus = http.StatusInternalServerError
		}
		common.Fail(c, httpStatus, appErr.Code, appErr.Msg)
		return
	}
	common.Fail(c, http.StatusInternalServerError, common.ErrModelCallFailed, err.Error())
}
```

- [ ] **Step 2: 创建 router.go**

```go
package controller

import (
	"github.com/gin-gonic/gin"
)

// SetupRouter 注册所有路由
func SetupRouter(r *gin.Engine, mcCtrl *ModelConfigController) {
	api := r.Group("/api")
	{
		mc := api.Group("/model-configs")
		{
			mc.POST("", mcCtrl.Create)
			mc.GET("", mcCtrl.List)
			mc.PATCH("/:id", mcCtrl.Update)
			mc.DELETE("/:id", mcCtrl.Delete)
			mc.POST("/:id/test", mcCtrl.Test)
		}
	}
}
```

- [ ] **Step 3: 验证编译**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go build ./internal/controller/...
```
Expected: 无错误输出

- [ ] **Step 4: Commit**

```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && git add -A && git commit -m "feat: add ModelConfig controller and router"
```

---

### Task 8: 主入口与集成

**Files:**
- Create: `llm-test-server/cmd/server/main.go`

- [ ] **Step 1: 创建 main.go**

```go
package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/config"
	"llm-test-server/internal/controller"
	"llm-test-server/internal/repository"
	"llm-test-server/internal/service"
)

func main() {
	// 加载配置
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	db, err := repository.InitDB(&cfg.Database)
	if err != nil {
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
	log.Printf("服务启动于 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
```

- [ ] **Step 2: 整体编译**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go mod tidy && go build ./cmd/server/...
```
Expected: 无错误输出

- [ ] **Step 3: Commit**

```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && git add -A && git commit -m "feat: add main entry point, wire all layers together"
```

---

### Task 9: 冒烟测试验证

**Files:** 无新文件

- [ ] **Step 1: 启动服务**

Run:
```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && go run ./cmd/server/
```
Expected: 输出 `服务启动于 :8080`，服务运行中

- [ ] **Step 2: 测试创建模型配置**

在另一个终端执行：

```bash
curl -X POST http://localhost:8080/api/model-configs \
  -H "Content-Type: application/json" \
  -d '{"ModelName":"GPT-4o 测试","ModelId":"gpt-4o","ApiUrl":"https://api.openai.com/v1/chat/completions","ApiKey":"sk-testkey1234567890abcdef","Temperature":0.7,"MaxTokens":4096}'
```
Expected: `{"ErrorCode":0,"ErrorMsg":"","Data":null}`

- [ ] **Step 3: 测试查询模型配置列表**

```bash
curl http://localhost:8080/api/model-configs
```
Expected: 返回分页结构，Items 中包含刚创建的配置，ApiKey 已脱敏为 `sk-****cdef`

- [ ] **Step 4: 记录返回的 Id，测试更新**

```bash
curl -X PATCH http://localhost:8080/api/model-configs/{Id} \
  -H "Content-Type: application/json" \
  -d '{"ModelName":"GPT-4o 测试-V2","Temperature":0.5}'
```
Expected: `{"ErrorCode":0,"ErrorMsg":"","Data":null}`

- [ ] **Step 5: 测试按 ID 查询**

```bash
curl "http://localhost:8080/api/model-configs?Id={Id}"
```
Expected: 返回单条配置，ModelName 已更新

- [ ] **Step 6: 测试删除**

```bash
curl -X DELETE http://localhost:8080/api/model-configs/{Id}
```
Expected: `{"ErrorCode":0,"ErrorMsg":"","Data":null}`

- [ ] **Step 7: 测试删除不存在的配置**

```bash
curl -X DELETE http://localhost:8080/api/model-configs/not_exist
```
Expected: `{"ErrorCode":40006,"ErrorMsg":"模型配置不存在","Data":null}`

- [ ] **Step 8: 停止服务，最终 commit**

按 Ctrl+C 停止服务

```bash
cd "c:/Users/李陈/Desktop/大模型测试/llm-test-server" && git add -A && git commit -m "chore: smoke test passed"
```
