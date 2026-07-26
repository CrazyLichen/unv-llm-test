# 大模型检测效果评估平台 — 后台框架与模型配置设计

> 日期：2026-07-26
> 范围：后台框架搭建、配置管理、数据库连接、模型配置 CRUD + 连通性测试

---

## 1. 技术选型

| 项目 | 选择 | 理由 |
|------|------|------|
| 语言 | Go 1.24 | 用户指定 |
| Web 框架 | Gin | 轻量高性能，路由分组/中间件完善 |
| 数据库 | SQLite | 单机 PC 部署，零配置 |
| SQLite 驱动 | modernc.org/sqlite | 纯 Go，无需 CGO |
| 配置格式 | YAML + 环境变量覆盖 | 简洁易读 |

## 2. 项目结构

```
llm-test-server/
├── cmd/
│   └── server/
│       └── main.go              # 入口：加载配置、初始化DB、注册路由、启动服务
├── internal/
│   ├── config/
│   │   └── config.go            # 配置结构体 + 加载逻辑
│   ├── model/
│   │   ├── model_config.go      # ModelConfig 结构体
│   │   ├── task.go              # Task, Progress 等结构体（后续）
│   │   └── image.go             # Image, Detection, Box 等结构体（后续）
│   ├── repository/
│   │   ├── model_config_repo.go # ModelConfig DB 操作
│   │   ├── task_repo.go         # Task DB 操作（后续）
│   │   └── image_repo.go        # Image DB 操作（后续）
│   ├── service/
│   │   └── model_config_svc.go  # ModelConfig 业务逻辑
│   ├── controller/
│   │   ├── model_config_ctrl.go # ModelConfig HTTP handler
│   │   └── router.go            # 总路由入口
│   └── common/
│       ├── response.go          # 统一响应结构、分页结构
│       └── errorcode.go         # 错误码常量
├── config.yaml                  # 配置文件模板
├── go.mod
└── go.sum
```

## 3. 配置设计

### config.yaml

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

### 配置加载逻辑

- 优先从 `config.yaml` 读取
- 环境变量覆盖：`SERVER_PORT`、`DB_DSN`、`LOG_LEVEL` 等
- 配置结构体：

```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Log      LogConfig
}
```

## 4. 数据库设计

### 4.1 连接初始化

- 启动时自动创建 `./data/` 目录（如不存在）
- 使用 `modernc.org/sqlite` 驱动打开 SQLite
- `SetMaxOpenConns(1)` — SQLite 写入单写锁，避免并发写冲突
- 自动执行建表 SQL（`CREATE TABLE IF NOT EXISTS`），无需迁移工具

### 4.2 模型配置表（model_configs）

> 对文档 v2.1 的调整：去掉 Name 字段（与 ModelName 冗余），新增 ModelId 字段（API 请求中的 model 标识）

```sql
CREATE TABLE IF NOT EXISTS model_configs (
    id          TEXT PRIMARY KEY,          -- mc_{ModelId}_{uuid32}
    model_name  TEXT NOT NULL,             -- 用户自定义名称/显示名称
    model_id    TEXT NOT NULL,             -- API model 标识（如 gpt-4o、qwen-vl-max）
    api_url     TEXT NOT NULL,             -- 大模型 API 地址
    api_key     TEXT NOT NULL,             -- API 访问密钥
    temperature REAL NOT NULL DEFAULT 0.7, -- 温度参数
    max_tokens  INTEGER NOT NULL DEFAULT 4096, -- 最大生成 token 数
    created_at  TEXT NOT NULL,             -- 创建时间 yyyy-MM-dd HH:mm:ss
    updated_at  TEXT NOT NULL              -- 更新时间 yyyy-MM-dd HH:mm:ss
);
```

### 4.3 ID 生成规则

格式：`mc_{ModelId}_{uuid32}`

示例：`mc_gpt-4o_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`

使用 Go 标准库 `crypto/rand` 生成 32 位十六进制随机串。

### 4.4 时间格式

统一使用 `yyyy-MM-dd HH:mm:ss`，与接口文档约定一致，以 TEXT 类型存储。

## 5. 统一响应结构

### 5.1 通用响应

```go
type Response struct {
    ErrorCode int         `json:"ErrorCode"`
    ErrorMsg  string      `json:"ErrorMsg"`
    Data      interface{} `json:"Data"`
}
```

成功时 `ErrorCode=0, ErrorMsg="", Data=业务数据或null`。
失败时 `ErrorCode=错误码, ErrorMsg=描述, Data=null`。

### 5.2 分页响应

```go
type PageData struct {
    Total    int         `json:"Total"`
    Page     int         `json:"Page"`
    PageSize int         `json:"PageSize"`
    Items    interface{} `json:"Items"`
}
```

### 5.3 错误码

| 常量名 | 值 | 说明 |
|--------|-----|------|
| Success | 0 | 成功 |
| ErrParamInvalid | 40001 | 参数校验失败 |
| ErrTaskNotFound | 40002 | 任务不存在 |
| ErrImageNotFound | 40003 | 素材不存在 |
| ErrPathNotFound | 40004 | 路径不存在 |
| ErrTaskStatusConflict | 40005 | 任务状态不允许此操作 |
| ErrModelConfigNotFound | 40006 | 模型配置不存在 |
| ErrModelCallFailed | 50001 | 大模型调用失败 |
| ErrVideoFrameFailed | 50002 | 视频抽帧失败 |

## 6. 模型配置 API

### 6.1 创建模型配置 `POST /api/model-configs`

**请求体**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ModelName | string | 是 | 用户自定义名称 |
| ModelId | string | 是 | API 模型标识 |
| ApiUrl | string | 是 | API 地址 |
| ApiKey | string | 是 | API 密钥 |
| Temperature | float | 否 | 默认 0.7 |
| MaxTokens | int32 | 否 | 默认 4096 |

**后端逻辑**：
1. 校验必填字段
2. 生成 ID：`mc_{ModelId}_{uuid32}`
3. 设置默认值
4. 插入数据库
5. 返回 `{ErrorCode:0, ErrorMsg:"", Data:null}`

### 6.2 获取模型配置列表 `GET /api/model-configs`

**Query 参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Id | string | 否 | 按 ID 精确查询，忽略分页 |
| Page | int32 | 否 | 默认 1 |
| PageSize | int32 | 否 | 默认 20 |

**后端逻辑**：
- 传 Id 时查询单条，包裹在分页结构中返回（Total=1, Page=1, PageSize=1）
- 不传 Id 时按分页查询全部
- **ApiKey 脱敏**：返回时只显示前3后4位，中间用 `****` 替代

### 6.3 更新模型配置 `PATCH /api/model-configs/:id`

**请求体**：仅传需要更新的字段，未传保持不变。使用指针类型区分"未传"与"传了零值"。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ModelName | *string | 否 | 用户自定义名称 |
| ModelId | *string | 否 | API 模型标识 |
| ApiUrl | *string | 否 | API 地址 |
| ApiKey | *string | 否 | API 密钥 |
| Temperature | *float64 | 否 | 温度参数（指针，可区分未传与0） |
| MaxTokens | *int32 | 否 | 最大 token 数（指针，可区分未传与0） |

**后端逻辑**：
1. 查询配置是否存在（不存在返回 40006）
2. 仅更新指针非 nil 的字段
3. 更新 UpdatedAt
4. ModelId 变更不影响已有 ID（ID 在创建时确定）

### 6.4 删除模型配置 `DELETE /api/model-configs/:id`

**后端逻辑**：
1. 查询配置是否存在
2. 检查 tasks 表中是否有关联记录（model_config_id = :id）
3. 存在关联 → 返回 ErrorCode=40005
4. 无关联 → 硬删除

### 6.5 测试模型连通性 `POST /api/model-configs/:id/test`

**后端逻辑**：
1. 查询模型配置
2. 构造 OpenAI 兼容请求：
   ```json
   {
     "model": "{ModelId}",
     "messages": [{"role": "user", "content": "Hello"}],
     "max_tokens": 5
   }
   ```
3. 发送到 ApiUrl，记录耗时
4. 成功：`{ErrorCode:0, Data:{Latency: 毫秒数}}`
5. 失败：`{ErrorCode:50001, ErrorMsg: "具体原因", Data:null}`

## 7. 接口文档调整记录

与原文档 v2.1 的差异：

| 位置 | 原文档 | 调整后 | 原因 |
|------|--------|--------|------|
| ModelConfig 数据模型 | Name + ModelName | ModelName + ModelId | Name 与 ModelName 冗余；需单独字段存放 API model 标识 |
| ModelConfig.Id | `mc_001` 示例 | `mc_{ModelId}_{uuid32}` | 用户指定格式，更具唯一性 |
| 创建模型配置请求 | Name + ModelName | ModelName + ModelId | 同上 |
| 更新模型配置请求 | Name + ModelName 可选更新 | ModelName + ModelId 可选更新 | 同上 |
| 获取模型配置列表 | ApiKey 原样返回 | ApiKey 脱敏返回 | 安全性 |

## 8. 后续扩展（本次不实现）

以下模块在框架搭建完成后逐步添加：

- **任务管理**：task_repo.go, task_svc.go, task_ctrl.go + tasks 表
- **素材管理**：image_repo.go, image_svc.go, image_ctrl.go + images 表
- **任务调度**：后台 goroutine 轮询 Pending/Analyzing 任务，调用大模型检测
- **视频抽帧**：FFmpeg 集成，Type=Video 时自动抽帧
