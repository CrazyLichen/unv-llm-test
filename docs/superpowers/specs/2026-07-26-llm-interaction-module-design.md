# LLM 交互模块设计

> 日期：2026-07-26
> 状态：已批准

---

## 1. 概述

实现一个独立的 LLM 交互模块，封装与 OpenAI 兼容大模型的通信逻辑。模块作为通用基础设施，供任务模块和模型配置模块调用，不包含业务语义解析。

### 1.1 核心职责

- 封装 `openai-go` SDK，提供简洁的调用接口
- 支持多模态消息（文本提示词 + 图片 base64）
- Client 实例懒加载缓存，模型配置变更时自动失效
- 解析 OpenAI 响应格式，原始 content 返回给调用方
- 可选重试和超时控制
- 统一错误处理，映射到项目通用 AppError

### 1.2 不做的事

- 不解析模型返回 content 的业务语义（检测框、置信度等由任务模块解析）
- 不做批量并发控制（由调用方自行管理）
- 不支持流式响应
- 不支持图片 URL 模式（仅 base64 data URI）

---

## 2. 模块结构

```
internal/llm/
├── factory.go      # ClientFactory — openai.Client 缓存管理（懒加载、失效清理）
├── client.go       # LLMClient — Analyze 方法，单次调用+响应解析
├── types.go        # LLMRequest, LLMResponse, Usage 类型定义
└── options.go      # AnalyzeOption, WithRetry(), WithTimeout() 调用选项
```

### 2.1 依赖

- `github.com/openai/openai-go/v3` — OpenAI 官方 Go SDK
- `github.com/openai/openai-go/v3/option` — SDK 选项包
- `llm-test-server/internal/common` — AppError、日志

### 2.2 依赖关系

```
common (AppError)
  ↑
llm (ClientFactory, LLMClient)
  ↑
service (ModelConfigService, TaskService...)
```

- `llm` 包依赖 `common` 包（AppError）
- `llm` 包依赖 `openai-go` SDK
- `service` 包依赖 `llm` 包（注入 factory 或 client）
- `llm` 包不依赖 `model` 包（ModelConfig 作为参数传入，不直接引用）

---

## 3. 类型定义

### 3.1 LLMRequest

```go
// LLMRequest LLM 分析请求
type LLMRequest struct {
    // Prompt 提示词（作为 User 消息发送）
    Prompt string
    // ImageBase64 图片 base64 data URI 列表（如 "data:image/jpeg;base64,..."）
    ImageBase64 []string
}
```

### 3.2 LLMResponse

```go
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

### 3.3 AnalyzeOption

```go
// AnalyzeOption 调用选项
type AnalyzeOption struct {
    // MaxRetries 重试次数（默认 0，不重试）
    MaxRetries int
    // Timeout 单次请求超时（默认 60s）
    Timeout time.Duration
}

// WithRetry 设置重试次数
func WithRetry(n int) AnalyzeOption { ... }

// WithTimeout 设置请求超时
func WithTimeout(d time.Duration) AnalyzeOption { ... }
```

---

## 4. ClientFactory

### 4.1 设计

```go
// ClientFactory openai.Client 缓存管理（懒加载、失效清理）
type ClientFactory struct {
    clients sync.Map // key: configID, value: *clientEntry
}

// clientEntry 缓存的 client 条目
type clientEntry struct {
    client   *openai.Client
    configID string
    apiKey   string  // 用于检测 config 变更
    apiUrl   string  // 用于检测 config 变更
    modelId  string  // 用于检测 config 变更
}
```

### 4.2 核心方法

#### GetOrCreateClient

```go
func (f *ClientFactory) GetOrCreateClient(configID, apiUrl, apiKey, modelId string) *openai.Client
```

流程：
1. 从 `sync.Map` 读取 configID
2. 命中 → 检查 apiKey/apiUrl/modelId 是否与传入参数一致
   - 一致 → 返回缓存 client
   - 不一致 → 删除旧 entry，创建新 client 并缓存
3. 未命中 → 创建新 client 并缓存

构造 client 时使用：
- `option.WithBaseURL(apiUrl)` — 支持自定义 API 端点（通义千问等兼容接口）
- `option.WithAPIKey(apiKey)` — API 密钥认证

#### RemoveClient

```go
func (f *ClientFactory) RemoveClient(configID string)
```

从 sync.Map 中移除指定 configID 的缓存条目。

### 4.3 缓存失效时机

- `ModelConfigService.Update` 后调用 `factory.RemoveClient(id)`
- `ModelConfigService.Delete` 后调用 `factory.RemoveClient(id)`
- `GetOrCreateClient` 的变更检测机制保证即使 RemoveClient 漏调也不会出错（下次调用时会重建）

### 4.4 懒加载

不在启动时预建 client，只在 `Analyze` 调用时触发 `GetOrCreateClient`。

---

## 5. LLMClient

### 5.1 设计

```go
// LLMClient LLM 交互客户端
type LLMClient struct {
    factory *ClientFactory
}

// NewLLMClient 创建 LLM 客户端实例
func NewLLMClient(factory *ClientFactory) *LLMClient
```

### 5.2 核心方法

#### Analyze

```go
func (c *LLMClient) Analyze(
    ctx context.Context,
    configID string,
    config ModelConfigParam,
    req LLMRequest,
    opts ...AnalyzeOption,
) (*LLMResponse, error)
```

其中 `ModelConfigParam` 为轻量参数结构（避免直接依赖 model 包）：

```go
// ModelConfigParam 模型配置参数（由调用方从 ModelConfig 实体提取）
type ModelConfigParam struct {
    ApiUrl      string
    ApiKey      string
    ModelId     string
    Temperature float64
    MaxTokens   int32
}
```

流程：
1. 参数校验（Prompt 非空、ImageBase64 非空）
2. 从 factory 获取/创建 openai.Client
3. 构造 ChatCompletion 请求：
   - `model = config.ModelId`
   - `temperature = config.Temperature`
   - `max_tokens = config.MaxTokens`
   - `messages = [UserMessage(TextPart(req.Prompt), ImagePart(base64)...)]`
4. 发送请求（应用 AnalyzeOption 中的重试和超时）
5. 解析响应，构造 LLMResponse 返回
6. 记录日志（请求前/后/错误）

### 5.3 调用示例

```go
// 任务模块调用 LLM
resp, err := llmClient.Analyze(ctx, "mc_001", llm.ModelConfigParam{
    ApiUrl:      mc.ApiUrl,
    ApiKey:      mc.ApiKey,
    ModelId:     mc.ModelId,
    Temperature: mc.Temperature,
    MaxTokens:   mc.MaxTokens,
}, llm.LLMRequest{
    Prompt:      "请检测图片中是否有沿街摆摊行为，返回JSON格式...",
    ImageBase64: []string{"data:image/jpeg;base64,/9j/4AAQ..."},
}, llm.WithRetry(2), llm.WithTimeout(30*time.Second))

if err != nil {
    // 处理错误，记录 FailReason
    var appErr common.AppError
    if errors.As(err, &appErr) {
        image.FailReason = appErr.Error()
    }
    return
}

// resp.Content 是模型返回的原始文本，任务模块自行解析检测框
boxes := parseDetectionResult(resp.Content)
```

---

## 6. 通用 AppError 重构

### 6.1 目标

将现有 `service.AppError` 提升到 `common` 包，成为项目通用的错误类型，所有模块共用。

### 6.2 AppError 类型

```go
// AppError 业务错误，携带错误码和消息
type AppError struct {
    // Code 错误码
    Code int
    // Message 错误消息
    Message string
    // Detail 详细错误信息（可选）
    Detail string
}

func (e AppError) Error() string {
    if e.Detail != "" {
        return fmt.Sprintf("%s（%s）", e.Message, e.Detail)
    }
    return e.Message
}
```

注意：`AppError` 改为值类型（非指针），`Error()` 方法的接收者也改为值接收者。调用方使用 `errors.As` 而非类型断言。

### 6.3 错误码分段

按领域 1000 递增分段，每个领域有独立编号前缀：

```
0001-0999  通用
1001-1999  模型配置领域
2001-2999  素材库领域
3001-3999  任务领域
5001-5999  LLM 领域
6001-6999  文件处理领域
```

### 6.4 预定义错误常量

```go
// ──────── 通用 (0xxx) ────────
var ErrParamValidation = AppError{Code: 1, Message: "参数校验失败"}

// ──────── 模型配置领域 (1xxx) ────────
var ErrModelConfigNotFound    = AppError{Code: 1001, Message: "模型配置不存在"}
var ErrModelConfigBoundByTask = AppError{Code: 1002, Message: "模型配置已被任务关联"}

// ──────── 素材库领域 (2xxx) ────────
var ErrMaterialLibNotFound    = AppError{Code: 2001, Message: "素材库不存在"}
var ErrMaterialLibBound       = AppError{Code: 2002, Message: "素材库已被任务关联"}
var ErrLibTypeMismatch        = AppError{Code: 2003, Message: "素材库类型与任务类型不匹配"}
var ErrMaterialFileNotFound   = AppError{Code: 2004, Message: "素材文件不存在"}
var ErrFileUploadIncomplete   = AppError{Code: 2005, Message: "文件上传未完成"}

// ──────── 任务领域 (3xxx) ────────
var ErrTaskNotFound           = AppError{Code: 3001, Message: "任务不存在"}
var ErrTaskStatusInvalid      = AppError{Code: 3002, Message: "任务状态不允许此操作"}

// ──────── LLM 领域 (5xxx) ────────
var ErrLLMCallFailed          = AppError{Code: 5001, Message: "大模型调用失败"}

// ──────── 文件处理领域 (6xxx) ────────
var ErrVideoFrameFailed       = AppError{Code: 6001, Message: "视频抽帧失败"}
var ErrFileUploadFailed       = AppError{Code: 6002, Message: "文件上传失败"}
var ErrChunkUploadFailed      = AppError{Code: 6003, Message: "分片上传异常"}
```

### 6.5 带 Detail 的构造函数

对于需要携带详细信息的错误类型，提供 `NewErrXXX(detail)` 构造函数：

```go
// NewErrLLMCallFailed 创建大模型调用失败错误（带详细信息）
func NewErrLLMCallFailed(detail string) AppError {
    return AppError{Code: 5001, Message: "大模型调用失败", Detail: detail}
}

// NewErrParamValidation 创建参数校验失败错误（带详细信息）
func NewErrParamValidation(detail string) AppError {
    return AppError{Code: 1, Message: "参数校验失败", Detail: detail}
}

// NewErrVideoFrameFailed 创建视频抽帧失败错误（带详细信息）
func NewErrVideoFrameFailed(detail string) AppError {
    return AppError{Code: 6001, Message: "视频抽帧失败", Detail: detail}
}
```

### 6.6 迁移影响

1. **`service.AppError`** → 移至 `common.AppError`，service 包引用 `common.AppError`
2. **`common/errorcode.go`** 中的纯常量 → 替换为 `AppError` 值常量，保留旧常量做兼容过渡
3. **`controller/handleError`** → 简化：不再断言错误类型，`err != nil` 就统一返回 HTTP 500，`ErrorMsg` 用 `err.Error()`，`ErrorCode` 用通用服务端错误码。AppError 的 Code/Message 供 service 层内部传递和日志使用

---

## 7. ModelConfigService 改造

### 7.1 注入 ClientFactory

```go
type ModelConfigService struct {
    repo    *repository.ModelConfigRepo
    factory *llm.ClientFactory
}

func NewModelConfigService(repo *repository.ModelConfigRepo, factory *llm.ClientFactory) *ModelConfigService
```

### 7.2 Update/Delete 时清理缓存

```go
func (s *ModelConfigService) Update(...) error {
    // ...existing logic...
    s.factory.RemoveClient(id)  // 配置变更，清理缓存
    // ...
}

func (s *ModelConfigService) Delete(...) error {
    // ...existing logic...
    s.factory.RemoveClient(id)  // 配置删除，清理缓存
    // ...
}
```

### 7.3 TestConnectivity 改用 SDK

将现有的手写 HTTP 请求替换为 `llmClient.Analyze` 或直接使用 factory 获取 client 发送测试请求：

```go
func (s *ModelConfigService) TestConnectivity(ctx context.Context, id string) (*model.TestModelConfigResp, error) {
    mc, err := s.repo.GetByID(ctx, id)
    if mc == nil {
        return nil, common.ErrModelConfigNotFound
    }

    client := s.factory.GetOrCreateClient(mc.Id, mc.ApiUrl, mc.ApiKey, mc.ModelId)
    start := time.Now()
    _, err = client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
        Model:    mc.ModelId,
        Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("Hello")},
        MaxTokens: opt.Int(5),
    })
    elapsed := time.Since(start).Milliseconds()

    if err != nil {
        return nil, common.NewErrLLMCallFailed(err.Error())
    }

    return &model.TestModelConfigResp{Latency: int(elapsed)}, nil
}
```

---

## 8. 日志规范

### 8.1 请求前

```
slog.Info("LLM调用开始", "configID", configID, "model", modelId, "promptLen", len(prompt), "imageCount", len(imageBase64))
```

### 8.2 请求后

```
slog.Info("LLM调用完成", "configID", configID, "model", resp.Model, "latency", elapsed, "promptTokens", usage.PromptTokens, "completionTokens", usage.CompletionTokens)
```

### 8.3 错误

```
slog.Error("LLM调用失败", "configID", configID, "model", modelId, "error", err)
```

---

## 9. main.go 初始化流程

```go
// 初始化 LLM 模块
llmFactory := llm.NewClientFactory()
llmClient  := llm.NewLLMClient(llmFactory)

// 注入到服务层
mcSvc := service.NewModelConfigService(mcRepo, llmFactory)  // 用于配置变更时清理缓存
// taskSvc := service.NewTaskService(taskRepo, llmClient)   // 后续任务模块
```
