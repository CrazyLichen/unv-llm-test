# LLM 交互模块实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现独立的 LLM 交互模块，封装 openai-go SDK，支持提示词+图片分析调用，client 懒加载缓存，供后续任务模块使用。

**Architecture:** 新增 `internal/llm/` 包（factory.go / client.go / types.go / options.go），同时将 `service.AppError` 重构为 `common.AppError`（通用错误类型，错误码按领域分段），简化 controller 错误处理，改造 ModelConfigService 使用 ClientFactory。

**Tech Stack:** Go 1.24, openai-go v3 SDK, Gin, GORM

---

## 文件结构

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 创建 | `server/internal/common/apperror.go` | 通用 AppError 类型 + 预定义错误常量 + NewErrXXX 构造函数 |
| 创建 | `server/internal/llm/types.go` | LLMRequest, LLMResponse, Usage, ModelConfigParam 类型 |
| 创建 | `server/internal/llm/options.go` | AnalyzeOption, WithRetry(), WithTimeout() |
| 创建 | `server/internal/llm/factory.go` | ClientFactory — sync.Map 缓存，懒加载，RemoveClient |
| 创建 | `server/internal/llm/client.go` | LLMClient — Analyze 方法 |
| 修改 | `server/internal/common/errorcode.go` | 替换旧错误码常量为新分段常量（向后兼容） |
| 修改 | `server/internal/common/response.go` | Fail 函数增加默认错误码逻辑 |
| 修改 | `server/internal/service/model_config_svc.go` | 移除 AppError 定义，引用 common.AppError，注入 ClientFactory，改造 TestConnectivity |
| 修改 | `server/internal/controller/model_config_ctrl.go` | 简化 handleError，不再断言 AppError |
| 修改 | `server/cmd/server/main.go` | 初始化 llm 模块，注入到 ModelConfigService |
| 修改 | `server/go.mod` | 添加 openai-go 依赖 |

---

### Task 1: 安装 openai-go SDK 依赖

**Files:**
- Modify: `server/go.mod`
- Modify: `server/go.sum`

- [ ] **Step 1: 安装 openai-go SDK**

注意：项目使用 Go 1.24，需安装兼容的 SDK 版本（v3.44.0 是支持 Go 1.22-1.24 的最新版本）。

```bash
cd server && go get github.com/openai/openai-go/v3@v3.44.0
```

- [ ] **Step 2: 验证依赖安装成功**

```bash
cd server && go mod tidy && go build ./...
```

Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
git add server/go.mod server/go.sum
git commit -m "chore: 添加 openai-go SDK v3 依赖"
```

---

### Task 2: 创建通用 AppError 类型

**Files:**
- Create: `server/internal/common/apperror.go`

- [ ] **Step 1: 创建 apperror.go**

```go
package common

import "fmt"

// ──────────────────────────── 结构体 ────────────────────────────

// AppError 业务错误，携带错误码和消息
type AppError struct {
	// Code 错误码
	Code int
	// Message 错误消息
	Message string
	// Detail 详细错误信息（可选）
	Detail string
}

// ──────────────────────────── 常量 ────────────────────────────

// 错误码按领域 1000 递增分段：
// 0xxx 通用
// 1xxx 模型配置领域
// 2xxx 素材库领域
// 3xxx 任务领域
// 5xxx LLM 领域
// 6xxx 文件处理领域
const (
	// ──────── 通用 (0xxx) ────────
	// ErrCodeParamValidation 参数校验失败错误码
	ErrCodeParamValidation = 1
	// ErrCodeServerInternal 内部服务错误码
	ErrCodeServerInternal = 2

	// ──────── 模型配置领域 (1xxx) ────────
	// ErrCodeModelConfigNotFound 模型配置不存在错误码
	ErrCodeModelConfigNotFound = 1001
	// ErrCodeModelConfigBoundByTask 模型配置已被任务关联错误码
	ErrCodeModelConfigBoundByTask = 1002

	// ──────── 素材库领域 (2xxx) ────────
	// ErrCodeMaterialLibNotFound 素材库不存在错误码
	ErrCodeMaterialLibNotFound = 2001
	// ErrCodeMaterialLibBound 素材库已被任务关联错误码
	ErrCodeMaterialLibBound = 2002
	// ErrCodeLibTypeMismatch 素材库类型不匹配错误码
	ErrCodeLibTypeMismatch = 2003
	// ErrCodeMaterialFileNotFound 素材文件不存在错误码
	ErrCodeMaterialFileNotFound = 2004
	// ErrCodeFileUploadIncomplete 文件上传未完成错误码
	ErrCodeFileUploadIncomplete = 2005

	// ──────── 任务领域 (3xxx) ────────
	// ErrCodeTaskNotFound 任务不存在错误码
	ErrCodeTaskNotFound = 3001
	// ErrCodeTaskStatusInvalid 任务状态不允许此操作错误码
	ErrCodeTaskStatusInvalid = 3002

	// ──────── LLM 领域 (5xxx) ────────
	// ErrCodeLLMCallFailed 大模型调用失败错误码
	ErrCodeLLMCallFailed = 5001

	// ──────── 文件处理领域 (6xxx) ────────
	// ErrCodeVideoFrameFailed 视频抽帧失败错误码
	ErrCodeVideoFrameFailed = 6001
	// ErrCodeFileUploadFailed 文件上传失败错误码
	ErrCodeFileUploadFailed = 6002
	// ErrCodeChunkUploadFailed 分片上传异常错误码
	ErrCodeChunkUploadFailed = 6003
)

// ──────────────────────────── 全局变量 ────────────────────────────

var (
	// ──────── 通用 (0xxx) ────────
	// ErrParamValidation 参数校验失败
	ErrParamValidation = AppError{Code: ErrCodeParamValidation, Message: "参数校验失败"}

	// ──────── 模型配置领域 (1xxx) ────────
	// ErrModelConfigNotFound 模型配置不存在
	ErrModelConfigNotFound = AppError{Code: ErrCodeModelConfigNotFound, Message: "模型配置不存在"}
	// ErrModelConfigBoundByTask 模型配置已被任务关联
	ErrModelConfigBoundByTask = AppError{Code: ErrCodeModelConfigBoundByTask, Message: "模型配置已被任务关联"}

	// ──────── 素材库领域 (2xxx) ────────
	// ErrMaterialLibNotFound 素材库不存在
	ErrMaterialLibNotFound = AppError{Code: ErrCodeMaterialLibNotFound, Message: "素材库不存在"}
	// ErrMaterialLibBound 素材库已被任务关联
	ErrMaterialLibBound = AppError{Code: ErrCodeMaterialLibBound, Message: "素材库已被任务关联"}
	// ErrLibTypeMismatch 素材库类型不匹配
	ErrLibTypeMismatch = AppError{Code: ErrCodeLibTypeMismatch, Message: "素材库类型与任务类型不匹配"}
	// ErrMaterialFileNotFound 素材文件不存在
	ErrMaterialFileNotFound = AppError{Code: ErrCodeMaterialFileNotFound, Message: "素材文件不存在"}
	// ErrFileUploadIncomplete 文件上传未完成
	ErrFileUploadIncomplete = AppError{Code: ErrCodeFileUploadIncomplete, Message: "文件上传未完成"}

	// ──────── 任务领域 (3xxx) ────────
	// ErrTaskNotFound 任务不存在
	ErrTaskNotFound = AppError{Code: ErrCodeTaskNotFound, Message: "任务不存在"}
	// ErrTaskStatusInvalid 任务状态不允许此操作
	ErrTaskStatusInvalid = AppError{Code: ErrCodeTaskStatusInvalid, Message: "任务状态不允许此操作"}

	// ──────── LLM 领域 (5xxx) ────────
	// ErrLLMCallFailed 大模型调用失败
	ErrLLMCallFailed = AppError{Code: ErrCodeLLMCallFailed, Message: "大模型调用失败"}

	// ──────── 文件处理领域 (6xxx) ────────
	// ErrVideoFrameFailed 视频抽帧失败
	ErrVideoFrameFailed = AppError{Code: ErrCodeVideoFrameFailed, Message: "视频抽帧失败"}
	// ErrFileUploadFailed 文件上传失败
	ErrFileUploadFailed = AppError{Code: ErrCodeFileUploadFailed, Message: "文件上传失败"}
	// ErrChunkUploadFailed 分片上传异常
	ErrChunkUploadFailed = AppError{Code: ErrCodeChunkUploadFailed, Message: "分片上传异常"}
)

// ──────────────────────────── 导出函数 ────────────────────────────

// Error 实现 error 接口
func (e AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s（%s）", e.Message, e.Detail)
	}
	return e.Message
}

// NewErrParamValidation 创建参数校验失败错误（带详细信息）
func NewErrParamValidation(detail string) AppError {
	return AppError{Code: ErrCodeParamValidation, Message: "参数校验失败", Detail: detail}
}

// NewErrLLMCallFailed 创建大模型调用失败错误（带详细信息）
func NewErrLLMCallFailed(detail string) AppError {
	return AppError{Code: ErrCodeLLMCallFailed, Message: "大模型调用失败", Detail: detail}
}

// NewErrVideoFrameFailed 创建视频抽帧失败错误（带详细信息）
func NewErrVideoFrameFailed(detail string) AppError {
	return AppError{Code: ErrCodeVideoFrameFailed, Message: "视频抽帧失败", Detail: detail}
}

// NewErrFileUploadFailed 创建文件上传失败错误（带详细信息）
func NewErrFileUploadFailed(detail string) AppError {
	return AppError{Code: ErrCodeFileUploadFailed, Message: "文件上传失败", Detail: detail}
}

// NewErrChunkUploadFailed 创建分片上传异常错误（带详细信息）
func NewErrChunkUploadFailed(detail string) AppError {
	return AppError{Code: ErrCodeChunkUploadFailed, Message: "分片上传异常", Detail: detail}
}
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./internal/common/...
```

Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add server/internal/common/apperror.go
git commit -m "feat: 添加通用 AppError 类型及错误码常量"
```

---

### Task 3: 更新 errorcode.go 保持向后兼容

**Files:**
- Modify: `server/internal/common/errorcode.go`

- [ ] **Step 1: 更新 errorcode.go，旧常量指向新错误码**

将现有的纯整数常量替换为引用新分段错误码，保持已引用这些常量的代码（如 model_config_svc.go、model_config_ctrl.go）继续编译。

```go
package common

// ──────────────────────────── 常量 ────────────────────────────

const (
	// Success 成功
	Success = 0

	// 以下为旧版错误码常量，映射到新版分段错误码，保持向后兼容
	// 新代码应使用 apperror.go 中的 ErrCodeXXX 常量和 ErrXXX 变量

	// ErrParamInvalid 参数校验失败（旧版，= ErrCodeParamValidation）
	ErrParamInvalid = ErrCodeParamValidation
	// ErrTaskNotFound 任务不存在（旧版，= ErrCodeTaskNotFound）
	ErrTaskNotFound = ErrCodeTaskNotFound
	// ErrImageNotFound 素材不存在（旧版，= ErrCodeMaterialFileNotFound）
	ErrImageNotFound = ErrCodeMaterialFileNotFound
	// ErrTaskStatusConflict 任务状态不允许此操作（旧版，= ErrCodeTaskStatusInvalid）
	ErrTaskStatusConflict = ErrCodeTaskStatusInvalid
	// ErrModelConfigNotFound 模型配置不存在（旧版，= ErrCodeModelConfigNotFound）
	ErrModelConfigNotFound = ErrCodeModelConfigNotFound
	// ErrLibraryNotFound 素材库不存在（旧版，= ErrCodeMaterialLibNotFound）
	ErrLibraryNotFound = ErrCodeMaterialLibNotFound
	// ErrLibraryAlreadyBound 素材库已被任务关联（旧版，= ErrCodeMaterialLibBound）
	ErrLibraryAlreadyBound = ErrCodeMaterialLibBound
	// ErrLibraryTypeMismatch 素材库类型不匹配（旧版，= ErrCodeLibTypeMismatch）
	ErrLibraryTypeMismatch = ErrCodeLibTypeMismatch
	// ErrFileNotFound 文件不存在（旧版，= ErrCodeMaterialFileNotFound）
	ErrFileNotFound = ErrCodeMaterialFileNotFound
	// ErrFileUploadIncomplete 文件上传未完成（旧版，= ErrCodeFileUploadIncomplete）
	ErrFileUploadIncomplete = ErrCodeFileUploadIncomplete
	// ErrModelCallFailed 大模型调用失败（旧版，= ErrCodeLLMCallFailed）
	ErrModelCallFailed = ErrCodeLLMCallFailed
	// ErrVideoFrameFailed 视频抽帧失败（旧版，= ErrCodeVideoFrameFailed）
	ErrVideoFrameFailed = ErrCodeVideoFrameFailed
	// ErrFileUploadFailed 文件上传失败（旧版，= ErrCodeFileUploadFailed）
	ErrFileUploadFailed = ErrCodeFileUploadFailed
	// ErrChunkUploadFailed 分片上传异常（旧版，= ErrCodeChunkUploadFailed）
	ErrChunkUploadFailed = ErrCodeChunkUploadFailed
)
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./...
```

Expected: 编译成功，所有引用旧常量的代码仍能工作

- [ ] **Step 3: 提交**

```bash
git add server/internal/common/errorcode.go
git commit -m "refactor: 旧错误码常量映射到新分段错误码"
```

---

### Task 4: 简化 controller 错误处理

**Files:**
- Modify: `server/internal/controller/model_config_ctrl.go`

- [ ] **Step 1: 简化 handleError 函数**

将 `handleError` 从断言 `*service.AppError` 改为只判断 `err != nil`，统一返回 HTTP 500。

```go
// handleError 统一错误处理，err != nil 时返回 HTTP 500
func handleError(c *gin.Context, err error) {
	slog.Error("请求处理失败", "error", err.Error(), "path", c.Request.URL.Path)
	common.Fail(c, http.StatusInternalServerError, common.ErrCodeServerInternal, err.Error())
}
```

- [ ] **Step 2: 移除 controller 中对 service 包的直接引用**

由于 handleError 不再断言 `*service.AppError`，检查 controller 的 import 是否还需要 `service` 包。当前 `model_config_ctrl.go` 的 import 中 `service` 包仅用于 `*service.ModelConfigService` 的类型引用，仍然需要保留。

无需修改 import，只需确保 handleError 函数替换完毕。

- [ ] **Step 3: 验证编译**

```bash
cd server && go build ./...
```

Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add server/internal/controller/model_config_ctrl.go
git commit -m "refactor: 简化 controller 错误处理，err != nil 统一返回 500"
```

---

### Task 5: 重构 ModelConfigService，移除旧 AppError

**Files:**
- Modify: `server/internal/service/model_config_svc.go`

- [ ] **Step 1: 移除 service 包中的 AppError 定义，改用 common.AppError**

修改要点：
1. 删除 `service.AppError` 结构体定义和 `Error()` 方法
2. 将所有 `&AppError{Code: ..., Msg: ...}` 替换为 `common.AppError{Code: ..., Message: ...}` 或直接使用预定义的 `common.ErrXXX` 变量
3. 注意：`common.AppError` 是值类型，返回时不需要取地址

替换对照表：

| 旧代码 | 新代码 |
|--------|--------|
| `&AppError{Code: common.ErrModelConfigNotFound, Msg: "模型配置不存在"}` | `common.ErrModelConfigNotFound` |
| `&AppError{Code: common.ErrTaskStatusConflict, Msg: "该模型配置下存在关联任务，无法删除"}` | `common.ErrModelConfigBoundByTask` |
| `&AppError{Code: common.ErrModelCallFailed, Msg: fmt.Sprintf("创建请求失败: %s", err.Error())}` | `common.NewErrLLMCallFailed(fmt.Sprintf("创建请求失败: %s", err.Error()))` |
| `&AppError{Code: common.ErrModelCallFailed, Msg: fmt.Sprintf("连接失败: %s", err.Error())}` | `common.NewErrLLMCallFailed(fmt.Sprintf("连接失败: %s", err.Error()))` |
| `&AppError{Code: common.ErrModelCallFailed, Msg: fmt.Sprintf("模型返回错误(HTTP %d): %s", resp.StatusCode, string(respBody))}` | `common.NewErrLLMCallFailed(fmt.Sprintf("模型返回错误(HTTP %d): %s", resp.StatusCode, string(respBody)))` |

完整替换后的 `model_config_svc.go` 中，删除以下内容：

```go
// AppError 业务错误，携带错误码和消息
type AppError struct {
	// Code 错误码
	Code int
	// Msg 错误消息
	Msg string
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	return e.Msg
}
```

替换所有 `&AppError{...}` 为对应的 `common.AppError{...}` 或 `common.ErrXXX` / `common.NewErrXXX(...)`。

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./...
```

Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add server/internal/service/model_config_svc.go
git commit -m "refactor: 移除 service.AppError，改用 common.AppError"
```

---

### Task 6: 创建 LLM 类型定义

**Files:**
- Create: `server/internal/llm/types.go`

- [ ] **Step 1: 创建 types.go**

```go
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
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./internal/llm/...
```

Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add server/internal/llm/types.go
git commit -m "feat: 添加 LLM 模块类型定义"
```

---

### Task 7: 创建 LLM 调用选项

**Files:**
- Create: `server/internal/llm/options.go`

- [ ] **Step 1: 创建 options.go**

```go
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
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./internal/llm/...
```

Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add server/internal/llm/options.go
git commit -m "feat: 添加 LLM 调用选项（WithRetry/WithTimeout）"
```

---

### Task 8: 创建 ClientFactory

**Files:**
- Create: `server/internal/llm/factory.go`

- [ ] **Step 1: 创建 factory.go**

```go
package llm

import (
	"sync"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// ──────────────────────────── 结构体 ────────────────────────────

// ClientFactory openai.Client 缓存管理（懒加载、失效清理）
type ClientFactory struct {
	clients sync.Map // key: configID, value: *clientEntry
}

// clientEntry 缓存的 client 条目
type clientEntry struct {
	client  *openai.Client
	apiKey  string // 用于检测 config 变更
	apiUrl  string // 用于检测 config 变更
	modelId string // 用于检测 config 变更
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewClientFactory 创建 ClientFactory 实例
func NewClientFactory() *ClientFactory {
	return &ClientFactory{}
}

// GetOrCreateClient 获取或创建 openai.Client（懒加载，缓存命中时检测配置变更）
func (f *ClientFactory) GetOrCreateClient(configID, apiUrl, apiKey, modelId string) *openai.Client {
	// 检查缓存
	if val, ok := f.clients.Load(configID); ok {
		entry := val.(*clientEntry)
		// 配置未变更，返回缓存 client
		if entry.apiKey == apiKey && entry.apiUrl == apiUrl && entry.modelId == modelId {
			return entry.client
		}
		// 配置已变更，删除旧缓存
		f.clients.Delete(configID)
	}

	// 创建新 client
	client := openai.NewClient(
		option.WithBaseURL(apiUrl),
		option.WithAPIKey(apiKey),
	)

	// 缓存
	f.clients.Store(configID, &clientEntry{
		client:  &client,
		apiKey:  apiKey,
		apiUrl:  apiUrl,
		modelId: modelId,
	})

	return &client
}

// RemoveClient 移除指定配置的缓存 client
func (f *ClientFactory) RemoveClient(configID string) {
	f.clients.Delete(configID)
}
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./internal/llm/...
```

Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add server/internal/llm/factory.go
git commit -m "feat: 添加 ClientFactory（懒加载缓存+失效清理）"
```

---

### Task 9: 创建 LLMClient

**Files:**
- Create: `server/internal/llm/client.go`

- [ ] **Step 1: 创建 client.go**

```go
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"llm-test-server/internal/common"
)

// ──────────────────────────── 常量 ────────────────────────────

const (
	// defaultTimeout 默认请求超时
	defaultTimeout = 60 * time.Second
)

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
	if opt.Timeout > 0 {
		reqOpts = append(reqOpts, option.WithRequestTimeout(opt.Timeout))
	}

	// 获取 client
	client := c.factory.GetOrCreateClient(configID, config.ApiUrl, config.ApiKey, config.ModelId)

	// 构造消息内容
	parts := make([]openai.ChatCompletionContentPartUnionParam, 0, 1+len(req.ImageBase64))
	parts = append(parts, openai.TextPart(req.Prompt))
	for _, img := range req.ImageBase64 {
		parts = append(parts, openai.ImagePart(img))
	}

	slog.Info("LLM调用开始", "configID", configID, "model", config.ModelId, "promptLen", len(req.Prompt), "imageCount", len(req.ImageBase64))

	start := time.Now()

	// 发送请求
	completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:       config.ModelId,
		Messages:    []openai.ChatCompletionMessageParamUnion{openai.UserMessage(parts...)},
		Temperature: openai.Float(config.Temperature),
		MaxTokens:   openai.Int(int64(config.MaxTokens)),
	}, reqOpts...)

	elapsed := time.Since(start)

	if err != nil {
		slog.Error("LLM调用失败", "configID", configID, "model", config.ModelId, "error", err, "latency", elapsed)
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

	slog.Info("LLM调用完成", "configID", configID, "model", resp.Model, "latency", elapsed,
		"promptTokens", resp.Usage.PromptTokens, "completionTokens", resp.Usage.CompletionTokens)

	return resp, nil
}

// TestConnectivity 测试模型连通性，返回延迟（毫秒）
func (c *LLMClient) TestConnectivity(ctx context.Context, configID string, config ModelConfigParam) (int, error) {
	client := c.factory.GetOrCreateClient(configID, config.ApiUrl, config.ApiKey, config.ModelId)

	start := time.Now()
	_, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    config.ModelId,
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("Hello")},
		MaxTokens: openai.Int(5),
	})
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return 0, common.NewErrLLMCallFailed(err.Error())
	}

	return int(elapsed), nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// mergeOptions 合并调用选项
func mergeOptions(opts []AnalyzeOption) AnalyzeOption {
	result := AnalyzeOption{Timeout: defaultTimeout}
	for _, o := range opts {
		if o.MaxRetries > 0 {
			result.MaxRetries = o.MaxRetries
		}
		if o.Timeout > 0 {
			result.Timeout = o.Timeout
		}
	}
	return result
}
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./internal/llm/...
```

Expected: 编译成功（可能需要根据 openai-go SDK 实际 API 微调类型签名）

- [ ] **Step 3: 提交**

```bash
git add server/internal/llm/client.go
git commit -m "feat: 添加 LLMClient（Analyze + TestConnectivity）"
```

---

### Task 10: 改造 ModelConfigService，注入 LLMClient

**Files:**
- Modify: `server/internal/service/model_config_svc.go`

- [ ] **Step 1: 修改 ModelConfigService 结构体，注入 LLMClient**

在 `model_config_svc.go` 中：

1. 添加 `llmClient` 字段到 `ModelConfigService` 结构体
2. 修改 `NewModelConfigService` 构造函数，接受 `*llm.LLMClient` 参数
3. 替换 `TestConnectivity` 实现，使用 `llmClient.TestConnectivity()`
4. 在 `Update` 和 `Delete` 成功后调用 client factory 的 `RemoveClient`（通过 llmClient 暴露，或直接注入 factory）

由于 LLMClient 内部持有 factory，我们在 LLMClient 上暴露 `RemoveClient` 代理方法：

修改 `ModelConfigService` 结构体：

```go
// ModelConfigService 模型配置业务逻辑层
type ModelConfigService struct {
	repo      *repository.ModelConfigRepo
	llmClient *llm.LLMClient
}

// NewModelConfigService 创建模型配置服务实例
func NewModelConfigService(repo *repository.ModelConfigRepo, llmClient *llm.LLMClient) *ModelConfigService {
	return &ModelConfigService{repo: repo, llmClient: llmClient}
}
```

需要先在 `llm/client.go` 中添加 `RemoveClient` 代理方法：

```go
// RemoveClient 移除指定配置的缓存 client（代理 factory.RemoveClient）
func (c *LLMClient) RemoveClient(configID string) {
	c.factory.RemoveClient(configID)
}
```

然后修改 `Update` 方法，在成功后清理缓存：

```go
func (s *ModelConfigService) Update(ctx context.Context, id string, req *model.UpdateModelConfigReq) error {
	// ...existing existence check...
	if err := s.repo.Update(ctx, id, req); err != nil {
		slog.Error("更新模型配置失败", "id", id, "error", err)
		return err
	}
	// 配置变更，清理 LLM client 缓存
	s.llmClient.RemoveClient(id)
	slog.Info("更新模型配置成功", "id", id)
	return nil
}
```

修改 `Delete` 方法，在成功后清理缓存：

```go
func (s *ModelConfigService) Delete(ctx context.Context, id string) error {
	// ...existing existence check and related tasks check...
	if err := s.repo.Delete(ctx, id); err != nil {
		slog.Error("删除模型配置失败", "id", id, "error", err)
		return err
	}
	// 配置删除，清理 LLM client 缓存
	s.llmClient.RemoveClient(id)
	slog.Info("删除模型配置成功", "id", id)
	return nil
}
```

替换 `TestConnectivity` 方法：

```go
// TestConnectivity 测试模型连通性
func (s *ModelConfigService) TestConnectivity(ctx context.Context, id string) (*model.TestModelConfigResp, error) {
	mc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if mc == nil {
		return nil, common.ErrModelConfigNotFound
	}

	latency, err := s.llmClient.TestConnectivity(ctx, mc.Id, llm.ModelConfigParam{
		ApiUrl:      mc.ApiUrl,
		ApiKey:      mc.ApiKey,
		ModelId:     mc.ModelId,
		Temperature: mc.Temperature,
		MaxTokens:   mc.MaxTokens,
	})
	if err != nil {
		return nil, err
	}

	slog.Info("模型连通性测试成功", "id", id, "modelId", mc.ModelId, "latency", latency)
	return &model.TestModelConfigResp{Latency: latency}, nil
}
```

同时删除 import 中不再需要的包：`crypto/rand`、`encoding/hex` 保留（generateID 用），`fmt` 保留，`io` 可删除，`net/http` 可删除，`strings` 可删除，`time` 可删除。

最终 import：

```go
import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"llm-test-server/internal/common"
	"llm-test-server/internal/llm"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)
```

- [ ] **Step 2: 验证编译**

```bash
cd server && go build ./...
```

Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add server/internal/llm/client.go server/internal/service/model_config_svc.go
git commit -m "refactor: ModelConfigService 注入 LLMClient，改造 TestConnectivity"
```

---

### Task 11: 更新 main.go 初始化流程

**Files:**
- Modify: `server/cmd/server/main.go`

- [ ] **Step 1: 添加 llm 模块初始化和注入**

在 main.go 中：

1. 添加 import `"llm-test-server/internal/llm"`
2. 在初始化各层部分，创建 ClientFactory 和 LLMClient
3. 修改 `NewModelConfigService` 调用，传入 llmClient

修改初始化部分：

```go
	// 初始化各层
	mcRepo := repository.NewModelConfigRepo(db)

	// 初始化 LLM 模块
	llmFactory := llm.NewClientFactory()
	llmClient := llm.NewLLMClient(llmFactory)

	mcSvc := service.NewModelConfigService(mcRepo, llmClient)
	mcCtrl := controller.NewModelConfigController(mcSvc)
```

- [ ] **Step 2: 验证编译和运行**

```bash
cd server && go build ./...
```

Expected: 编译成功

```bash
cd server && go run cmd/server/main.go
```

Expected: 服务正常启动，无报错

- [ ] **Step 3: 提交**

```bash
git add server/cmd/server/main.go
git commit -m "feat: main.go 初始化 LLM 模块并注入 ModelConfigService"
```

---

### Task 12: 全量编译验证与测试

**Files:**
- All modified files

- [ ] **Step 1: 全量编译**

```bash
cd server && go build ./...
```

Expected: 编译成功

- [ ] **Step 2: go vet 检查**

```bash
cd server && go vet ./...
```

Expected: 无警告

- [ ] **Step 3: 启动服务进行冒�烟测试**

```bash
cd server && go run cmd/server/main.go
```

在另一个终端测试现有接口是否正常：

```bash
# 测试创建模型配置
curl -X POST http://localhost:8080/api/model-configs \
  -H "Content-Type: application/json" \
  -d '{"Name":"测试配置","ModelId":"gpt-4o","ApiUrl":"https://api.openai.com/v1","ApiKey":"sk-test","Temperature":0.7,"MaxTokens":100}'

# 测试获取列表
curl http://localhost:8080/api/model-configs
```

Expected: 接口返回正常，错误码使用新分段值

- [ ] **Step 4: 最终提交（如有修复）**

```bash
git add -A && git commit -m "fix: 修复集成问题"
```

如果没有修复则跳过此步骤。
