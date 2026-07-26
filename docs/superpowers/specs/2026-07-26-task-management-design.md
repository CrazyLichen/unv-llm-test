# 任务管理模块设计

> 日期：2026-07-26
> 基于 `llm-test-api-docs.md` v3.0

---

## 1. 背景与目标

### 现状

模型配置管理、素材库管理、LLM 交互模块已实现。任务管理（创建、调度、执行、暂停/恢复、删除）尚未开始。

### 目标

实现任务管理的核心接口（不含结果查询和误报矫正），包括：

| 序号 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 1 | POST | `/api/tasks` | 创建任务 |
| 2 | GET | `/api/tasks` | 获取任务列表 |
| 3 | DELETE | `/api/tasks/:id` | 删除任务（级联硬删除） |
| 4 | PUT | `/api/tasks/:id` | 暂停/恢复任务 |

任务创建后自动排队执行，调度器逐个任务串行处理，每个任务内逐个素材串行调用大模型。

### 约束

- 任务调度：DB 持久化 + 内存队列混合方案
- 执行策略：单任务顺序执行，任务内逐个素材串行调用 LLM
- 视频抽帧：使用 ffmpeg-go 库，需系统安装 FFmpeg
- 重启恢复：从 DB 加载 Pending/Analyzing 状态任务重新入队

---

## 2. 数据模型

### 2.1 Task（任务）

```
tasks 表
├── Id                 string     PK, 格式: task_{uuid32}
├── Name               string     任务名称
├── Type               string     Image | Video
├── Status             string     Pending | Analyzing | Paused | Completed
├── ModelConfigId      string     FK → model_configs
├── MaterialLibraryId  string     FK → material_libraries
├── Prompt             string     下发给大模型的提示词
├── Target             string     检测目标名称
├── FrameInterval      *int32     抽帧间隔（秒），Type=Video 时有值
├── CreatedAt          time.Time
└── UpdatedAt          time.Time
```

### 2.2 Image（检测素材）

```
images 表
├── Id              string      PK, 格式: img_{uuid32}
├── TaskId          string      FK → tasks
├── AccessUrl       string      浏览器访问 URL
├── MaterialFileId  *string     关联素材文件 ID，图片集有值，视频帧为 null
├── FrameIndex      *int32      帧序号，视频帧有值
├── Status          string      Pending | Detected | NotDetected | Failed
├── Detection       *JSON       检测结果（Detection 结构）
├── FailReason      *string     失败原因
├── Correction      *string     null | "FalsePositive" | "DeletedFp"
└── CreatedAt       time.Time
```

### 2.3 Detection（检测结果，存为 JSON）

| 字段 | 类型 | 说明 |
|------|------|------|
| HasTarget | bool | 是否检测到目标 |
| Boxes | []Box | 检测框列表 |
| RawResponse | string | 大模型原始返回 |
| AnalyzedAt | string | 分析完成时间 |

### 2.4 Box（检测框）— 更新后的结构

| 字段 | 类型 | 说明 |
|------|------|------|
| X1 | int32 | 左上角 X（0-1000，基于图像宽高归一化） |
| Y1 | int32 | 左上角 Y（0-1000） |
| X2 | int32 | 右下角 X（0-1000） |
| Y2 | int32 | 右下角 Y（0-1000） |
| Confidence | string | 置信度描述（透传 LLM 的 confidence_note） |
| Label | string | 目标标签（透传 LLM 的 category） |

> **接口文档变更**：原 Box 结构的 `X, Y, Width, Height`（百分比 0-100）+ `Confidence`（float）变更为 `X1, Y1, X2, Y2`（0-1000 归一化）+ `Confidence`（string）。需同步更新 API 文档。

### 2.5 Progress（检测进度与统计）

Progress 不单独存储，通过实时查询 images 表聚合计算：

```
Total       = count(Correction != 'DeletedFp')
Completed   = count(Status IN [Detected, NotDetected, Failed] AND Correction != 'DeletedFp')
Pending     = count(Status = 'Pending')
Detected    = TruePositive + FalsePositive
TruePositive  = count(Status=Detected AND Correction IS NULL)
FalsePositive = count(Status=Detected AND Correction = 'FalsePositive')
NotDetected = count(Status=NotDetected AND Correction != 'DeletedFp')
Failed      = count(Status=Failed AND Correction != 'DeletedFp')
```

约束：`Total = Completed + Pending`，`Detected = TruePositive + FalsePositive`

---

## 3. 调度器设计

### 3.1 核心结构

```go
type Scheduler struct {
    taskQueue chan *TaskItem      // 带缓冲的任务队列
    cancelMap sync.Map            // taskID → context.CancelFunc
    parentCtx context.Context     // 全局 context，用于优雅关闭
    parentCancel context.CancelFunc
    wg        sync.WaitGroup     // Worker 退出等待
    repo      *repository.TaskRepo
    llmClient *llm.LLMClient
    uploadDir string              // 文件存储根目录
}

type TaskItem struct {
    TaskID string
    Ctx    context.Context
}
```

### 3.2 入队与控制规则

**核心原则：cancelMap 统一在入队前更新，Worker 只检查 ctx 信号。**

| 操作 | 流程 |
|------|------|
| 创建任务 | DB=Pending → 创建 ctx+cancel → cancelMap.Store(id, cancel) → 入队 TaskItem{id, ctx} |
| 暂停任务 | DB=Paused → cancelMap[id]() 取消 ctx → Worker 自动跳过或退出 |
| 恢复任务 | DB=Analyzing → 新 ctx+cancel → cancelMap.Store(id, cancel) → 入队 TaskItem{id, newCtx} |
| 删除任务 | 删除 DB 记录 → cancelMap[id]() 取消 ctx → Worker 自动跳过或退出 |

### 3.3 Worker 循环

```
for {
    select {
    case item := <-s.taskQueue:
        // 执行前检查
        if item.Ctx.Err() != nil { continue }  // 已取消，跳过

        s.wg.Add(1)
        s.executeTask(item)  // 同步执行
        s.wg.Done()

    case <-s.parentCtx.Done():
        return  // 优雅关闭
    }
}
```

### 3.4 暂停/恢复并发安全

场景：快速暂停→恢复→再暂停

- 恢复入队时，cancelMap 立即更新为最新的 cancel
- 队列中可能有多个旧 TaskItem（ctx 已取消），Worker 取到后通过 `ctx.Err()` 自动跳过
- 最新的 cancel 始终在 cancelMap 中，暂停操作取消的是最新的 ctx

### 3.5 重启恢复

```
1. 从 DB 加载 Status IN (Pending, Analyzing) 的任务
2. 将 Status 更新为 Analyzing 的任务重置为 Pending
3. 为每个任务创建 ctx+cancel → cancelMap.Store → 入队
```

### 3.6 优雅关闭

```
1. 调用 parentCancel() 取消所有任务 ctx
2. 等待 wg.Wait() 让 Worker 完成当前素材检测后退出
3. 不等待整个任务完成，保留进度到 DB
```

---

## 4. 任务执行流程

### 4.1 executeTask 主流程

```
func (s *Scheduler) executeTask(item *TaskItem):
    1. 从 DB 加载任务详情
    2. 更新 Status = Analyzing
    3. if Type == Image:
         ensureImageRecords(task)  // 为素材库图片创建 Image 记录
    4. if Type == Video:
         extractFrames(item.Ctx, task)  // 抽帧 + 创建 Image 记录
    5. detectImages(item)  // 逐个素材调用 LLM
    6. 检查是否全部完成 → 更新 Status = Completed
```

### 4.2 Image 类型 — ensureImageRecords

```
1. 查询关联素材库下所有 MaterialFile（UploadStatus=Completed）
2. 为每张图片创建 Image 记录：
   - AccessUrl = MaterialFile.AccessUrl
   - MaterialFileId = MaterialFile.Id
   - FrameIndex = null
   - Status = Pending
3. 图片上传成功后 UploadStatus 已是 Completed，无需额外检查
```

### 4.3 Video 类型 — extractFrames

```
1. 使用 ffmpeg-go 按帧间隔抽帧
2. 抽帧图片保存到持久化目录: {uploadDir}/tasks/{taskId}/frames/
3. 为每帧创建 Image 记录：
   - AccessUrl = 帧图片的访问 URL
   - MaterialFileId = null
   - FrameIndex = 帧序号
   - Status = Pending
4. 抽帧过程中检查 ctx.Done()，可被取消
5. 抽帧失败 → 记录日志，已抽帧的素材继续检测，未抽帧的不创建 Image 记录，Task 保持 Analyzing
```

### 4.4 detectImages — 逐个素材检测

```
1. 查询任务下所有 Status=Pending 的 Image 记录
2. 逐个处理：
   a. 检查 item.Ctx.Done() → 如已取消则退出循环
   b. 构造 LLM 请求：Prompt + 图片（base64 或 URL）
   c. 调用 llm.Analyze()
   d. 解析结果（见 4.5）
   e. 更新 Image 记录（Status + Detection/FailReason）
3. 循环结束后检查是否全部素材已完成
```

### 4.5 LLM 返回解析

LLM 返回的 content 需解析为结构化数据。正常 JSON 格式：

```json
{
  "detected_flag": true,
  "detections": [
    {
      "category": "路面破损",
      "bbox_2d": [319, 786, 594, 850],
      "confidence_note": "道路中央存在一条明显的横向裂缝..."
    }
  ]
}
```

解析策略：

```
content → 尝试标准 JSON 解析
├── 解析成功：
│   ├── detected_flag=true  → Status=Detected
│   │   └── detections → []Box:
│   │       category → Label
│   │       bbox_2d [x1,y1,x2,y2] → X1,Y1,X2,Y2
│   │       confidence_note → Confidence
│   └── detected_flag=false → Status=NotDetected
└── 解析失败（异常 JSON）：
    ├── 记录日志（原始响应）
    ├── 正则提取 detected_flag：
    │   ├── true  → Status=Detected
    │   │   └── 正则提取 bbox_2d 和 category（能取多少取多少）
    │   └── false → Status=NotDetected
    └── 正则也提取不到 flag → Status=Failed
        └── FailReason="无法解析检测结果"
```

字段映射：

| LLM JSON 字段 | Box 字段 | 说明 |
|---------------|----------|------|
| bbox_2d[0] | X1 | 左上角 X（0-1000） |
| bbox_2d[1] | Y1 | 左上角 Y（0-1000） |
| bbox_2d[2] | X2 | 右下角 X（0-1000） |
| bbox_2d[3] | Y2 | 右下角 Y（0-1000） |
| category | Label | 目标标签 |
| confidence_note | Confidence | 置信度描述 |

---

## 5. 接口实现

### 5.1 POST /api/tasks — 创建任务

```
1. 参数校验（必填项、Type 合法性）
2. 校验 ModelConfigId 是否存在
3. 校验 MaterialLibraryId 是否存在
4. 校验素材库类型与任务类型一致（Image↔Image, Video↔Video）
5. 校验素材库未被其他任务关联（1:1 绑定）
6. 校验素材库中不存在 UploadStatus != Completed 的文件（返回 40011）
7. Type=Video 时校验 FrameInterval 必填
8. 创建 Task 记录，Status = Pending
9. Type=Image 时：为素材库下所有图片创建 Image 记录（Status=Pending）
10. Type=Video 时：不创建 Image 记录，待执行时抽帧生成
11. 调用 Scheduler.Enqueue(taskID) 入队
12. 返回成功
```

### 5.2 GET /api/tasks — 获取任务列表

```
1. 支持 Id 精确查询（忽略分页）
2. 支持 Status 筛选
3. 支持分页（Page, PageSize）
4. 每条任务附带 Progress（实时聚合查询）
5. ModelConfigName 和 MaterialLibraryName 通过 JOIN 或关联查询获取
```

### 5.3 DELETE /api/tasks/:id — 删除任务

```
1. 校验任务是否存在
2. 调用 Scheduler.PauseTask(id) 取消执行中的任务
3. 级联硬删除：images 表 → tasks 表
4. 删除抽帧图片文件（如存在）
5. 注意：不删除素材库文件，只删除任务产生的 Image 记录和抽帧文件
```

### 5.4 PUT /api/tasks/:id — 暂停/恢复任务

```
Status="Paused":
  1. 校验任务存在且当前状态为 Pending 或 Analyzing
  2. 更新 DB Status = Paused
  3. 调用 cancelMap[taskID]() 取消 ctx

Status="Analyzing":
  1. 校验任务存在且当前状态为 Paused
  2. 更新 DB Status = Analyzing
  3. 创建新 ctx+cancel → cancelMap.Store(taskID, cancel)
  4. 入队 TaskItem{taskID, newCtx}
  5. Worker 执行时跳过已完成的素材，从中断处继续
```

---

## 6. 代码分层与文件组织

```
internal/
├── model/
│   └── task.go                      # Task, Image 实体 + DTO
├── repository/
│   ├── db.go                        # AutoMigrate 加入 Task, Image
│   └── task_repo.go                 # Task + Image 数据访问 + Progress 聚合
├── service/
│   ├── task_svc.go                  # 任务业务逻辑（CRUD + 暂停/恢复 + 与 Scheduler 交互）
│   └── task_executor.go             # Scheduler：队列管理、Worker、抽帧、LLM 调用、结果解析
├── controller/
│   ├── router.go                    # 新增任务路由
│   └── task_ctrl.go                 # 任务 HTTP handler
└── llm/                             # 已有，Analyze 方法复用
```

### 各层职责

| 层 | 文件 | 职责 |
|---|---|---|
| model | task.go | Task/Image 实体定义、请求/响应 DTO、枚举常量 |
| repository | task_repo.go | Task/Image CRUD、Progress 聚合查询、关联校验 |
| service | task_svc.go | 业务逻辑编排、参数校验、调用 Scheduler |
| service | task_executor.go | Scheduler 核心：队列、Worker、抽帧、LLM 调用、结果解析 |
| controller | task_ctrl.go | HTTP 参数绑定、调用 service、返回响应 |

---

## 7. 接口文档变更记录

### Box 结构变更

| 原字段 | 原类型 | 新字段 | 新类型 | 说明 |
|--------|--------|--------|--------|------|
| X | int32 (0-100) | X1 | int32 (0-1000) | 左上角 X，归一化值 |
| Y | int32 (0-100) | Y1 | int32 (0-1000) | 左上角 Y，归一化值 |
| Width | int32 (百分比) | X2 | int32 (0-1000) | 右下角 X，归一化值 |
| Height | int32 (百分比) | Y2 | int32 (0-1000) | 右下角 Y，归一化值 |
| Confidence | float | Confidence | string | 置信度描述文字 |

### bbox_2d 映射说明

LLM 返回的 `bbox_2d: [x1, y1, x2, y2]`，值为 0-1000 归一化坐标，直接映射到 Box 的 X1/Y1/X2/Y2 字段存储。

---

## 8. 错误处理

| 场景 | 处理方式 |
|------|----------|
| 创建任务时素材库有未完成文件 | 返回 ErrorCode=40011 |
| 创建任务时素材库已被关联 | 返回 ErrorCode=40008 |
| 创建任务时素材库类型不匹配 | 返回 ErrorCode=40009 |
| 视频抽帧失败 | 记录日志，已抽帧的素材继续检测，未抽帧的不创建 Image 记录，Task 保持 Analyzing |
| LLM 调用失败 | 单个素材 Status=Failed + FailReason，不影响其他素材 |
| JSON 解析失败 | 先记录日志，正则降级解析，全部失败则 Status=Failed |
| 暂停/恢复状态不合法 | 返回 ErrorCode=40005 |
