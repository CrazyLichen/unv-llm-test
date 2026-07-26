# Go 编码规范

## 规范 1：所有注释使用中文

所有 Go 源码文件中的注释必须使用中文，包括：

- 包注释（// Package xxx 提供...）
- 函数/方法注释（// FuncName 执行...）
- 结构体/接口注释（// User 用户信息）
- 字段注释（// ID 唯一标识）
- 常量/枚举注释
- 行内注释
- TODO/FIXME 标注

**正确示例：**

```go
// 模型客户端接口
type BaseModelClient interface {
    // 调用模型并返回完整响应
    Invoke(ctx context.Context, messages []Message, opts ...Option) (*AssistantMessage, error)
    // 流式调用模型，返回消息块通道
    Stream(ctx context.Context, messages []Message, opts ...Option) (<-chan AssistantMessageChunk, error)
}
```

**错误示例（禁止）：**

```go
// Model client interface
type BaseModelClient interface {
    // Invoke model and return full response
    Invoke(ctx context.Context, messages []Message, opts ...Option) (*AssistantMessage, error)
}
```

## 规范 2：源码声明排列顺序

每个 Go 源码文件内部，必须严格按以下顺序排列声明：

1. 结构体 (type struct / type interface) — 接口排在结构体之前
2. 枚举 (type iota)
3. 常量 (const)
4. 全局变量 (var)
5. 导出函数 (func Xxx) — 大写开头
6. 非导出函数 (func xxx) — 小写开头

各类声明之间用分隔注释区分：

```go
// ──────────────────────────── 结构体 ────────────────────────────

// ──────────────────────────── 枚举 ────────────────────────────

// ──────────────────────────── 常量 ────────────────────────────

// ──────────────────────────── 全局变量 ────────────────────────────

// ──────────────────────────── 导出函数 ────────────────────────────

// ──────────────────────────── 非导出函数 ────────────────────────────
```

### 完整示例

```go
package agent

// ──────────────────────────── 结构体 ────────────────────────────

// AgentCard Agent 身份元数据
type AgentCard struct {
    // ID 唯一标识
    ID string `json:"id"`
    // Name 名称
    Name string `json:"name"`
    // Description 描述
    Description string `json:"description"`
}

// AgentResult Agent 执行结果
type AgentResult struct {
    // TaskID 任务标识
    TaskID string `json:"task_id"`
    // Status 任务状态
    Status TaskStatus `json:"status"`
}

// ──────────────────────────── 枚举 ────────────────────────────

// TaskStatus 任务状态枚举
type TaskStatus int

const (
    // TaskStatusPending 待执行
    TaskStatusPending TaskStatus = iota
    // TaskStatusRunning 执行中
    TaskStatusRunning
    // TaskStatusCompleted 已完成
    TaskStatusCompleted
    // TaskStatusFailed 已失败
    TaskStatusFailed
)

// ──────────────────────────── 常量 ────────────────────────────

const (
    // MaxIterations ReAct 循环最大迭代次数
    MaxIterations = 10
    // DefaultModelName 默认模型名称
    DefaultModelName = "qwen-max"
)

// ──────────────────────────── 全局变量 ────────────────────────────

var (
    // globalRegistry 全局 Agent 注册表
    globalRegistry = make(map[string]*AgentCard)
)

// ──────────────────────────── 导出函数 ────────────────────────────

// NewAgent 创建新的 Agent 实例
func NewAgent(card *AgentCard) *Agent {
    return &Agent{card: card}
}

// Register 向全局注册表注册 Agent
func Register(card *AgentCard) error {
    globalRegistry[card.ID] = card
    return nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// buildSystemPrompt 构建系统提示词
func buildSystemPrompt(card *AgentCard) string {
    return ""
}

// validateCard 校验 AgentCard 字段
func validateCard(card *AgentCard) error {
    return nil
}
```

### 特例说明

- 接口 (type interface) 归类到结构体区块，排在结构体之前
- 类型别名 (type X = Y) 归类到枚举区块
- init() 函数归类到非导出函数区块
- 文件头只保留 package 和 import，不要在 import 后直接写 const/var
