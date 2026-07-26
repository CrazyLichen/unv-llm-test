# 任务结果领域设计

> 日期：2026-07-26

## 1. 概述

实现任务结果领域的 3 个 API 接口，补齐 API 文档中序号 20-22 的缺失功能：

| 序号 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 20 | GET | `/api/tasks/:id/images` | 获取任务分析结果 |
| 21 | POST | `/api/tasks/:id/images` | 补录漏报照片 |
| 22 | PUT | `/api/tasks/:id/images/:imageId` | 标记素材误报/删除误报/恢复 |

> 注意：API 文档中接口 22 使用 PATCH 方法，经讨论统一改为 PUT，与任务暂停/恢复接口保持风格一致。

## 2. 数据模型

### 2.1 ImageItem（响应 DTO）

将 Image 实体的 `sql.NullString` Detection 转为前端友好的结构：

```go
type ImageItem struct {
    Id             string     `json:"Id"`
    TaskId         string     `json:"TaskId"`
    AccessUrl      string     `json:"AccessUrl"`
    MaterialFileId *string    `json:"MaterialFileId"`
    FrameIndex     *int32     `json:"FrameIndex"`
    Status         string     `json:"Status"`
    Detection      *Detection `json:"Detection"`
    FailReason     *string    `json:"FailReason"`
    Correction     *string    `json:"Correction"`
}
```

- `Detection`：Image 实体中为 `sql.NullString`，响应时 JSON 反序列化为 `*Detection`；无检测结果时为 `null`
- `Correction`：`null` 表示正常（TruePositive），`"FalsePositive"` 表示误报，`"DeletedFp"` 表示已删除误报

### 2.2 UpdateCorrectionReq（矫正请求 DTO）

```go
type UpdateCorrectionReq struct {
    Correction *string `json:"Correction"`
}
```

- 不使用 `binding:"required"`，因为需要支持 `null` 值（恢复正常）
- 值域校验在 Service 层完成：`nil` / `"FalsePositive"` / `"DeletedFp"`

## 3. Repository 层

### 3.1 ListImages — 分页查询素材列表

```go
func (r *TaskRepo) ListImages(ctx context.Context, taskId string, page, pageSize int,
    status string, correction string) ([]model.Image, int, error)
```

查询逻辑：
- 基础条件：`task_id = ?`
- **默认行为**（不传 Correction 参数）：排除 `correction = 'DeletedFp'` 的素材
- `correction = "FalsePositive"` → 只查 `correction = 'FalsePositive'`
- `correction = "DeletedFp"` → 只查 `correction = 'DeletedFp'`
- `status != ""` → 追加 `status = ?` 条件
- 排序：`created_at ASC`（最早上传的在前，与素材文件列表一致）
- 分页：offset/limit

### 3.2 GetImageByIDAndTaskId — 按 ID + 任务 ID 精确查询

```go
func (r *TaskRepo) GetImageByIDAndTaskId(ctx context.Context, taskId, imageId string) (*model.Image, error)
```

- 比 `GetImageByID` 多一个 taskId 条件，确保跨任务数据隔离
- 未找到时返回 nil, nil（与 `GetByID` 风格一致）

### 3.3 复用已有方法

- `UpdateImage` — 更新 Correction 字段
- `CreateImage` — 补录照片时创建 Image 记录

## 4. Service 层

### 4.1 ListImages — 获取任务分析结果

```
输入：taskId, imageId, page, pageSize, status, correction
输出：([]ImageItem, int, error)
```

逻辑：
1. 校验任务存在性（`repo.GetByID`），不存在返回 `ErrTaskNotFound`
2. 如果 `imageId != ""`：
   - 调用 `repo.GetImageByIDAndTaskId` 精确查询
   - 不存在返回 `ErrImageNotFound`
   - 转为 ImageItem 返回（Total=1）
3. 否则：
   - 调用 `repo.ListImages` 分页查询
   - 逐条转为 ImageItem 返回

**Image → ImageItem 转换**：
- `Detection`：`sql.NullString` → JSON 反序列化为 `*Detection`；`Valid=false` 或反序列化失败时设为 `nil`
- 其余字段直接映射

### 4.2 UploadMissedPhoto — 补录漏报照片

```
输入：taskId, file (*multipart.FileHeader)
输出：error
```

逻辑：
1. 校验任务存在性
2. 生成 `img_` 前缀 ID
3. 保存文件到 `{uploadDir}/tasks/{taskId}/frames/{imageId}{ext}`
4. 创建 Image 记录：
   - Status = `"Detected"`
   - Detection = null（`sql.NullString{Valid: false}`）
   - MaterialFileId = nil
   - Correction = nil
   - AccessUrl = `/uploads/tasks/{taskId}/frames/{imageId}{ext}`

### 4.3 MarkCorrection — 标记素材误报/恢复

```
输入：taskId, imageId, correction (*string)
输出：error
```

逻辑：
1. 校验任务存在性
2. 查询 Image（`repo.GetImageByIDAndTaskId`），不存在返回 `ErrImageNotFound`
3. 校验 Correction 值域：
   - `nil` → 恢复正常
   - `"FalsePositive"` → 标记误报
   - `"DeletedFp"` → 删除误报
   - 其他值 → 返回 `ErrParamValidation`
4. 调用 `repo.UpdateImage` 更新 Correction 字段
5. Progress 自动生效（`CountImagesByStatus` 实时聚合，无需额外处理）

Correction 状态流转：任意状态之间可直接切换，不需要逐步恢复。

## 5. Controller 层

### 5.1 ListImages

```
GET /api/tasks/:id/images?ImageId=&Page=1&PageSize=24&Status=&Correction=
```

- 解析 path param `id`
- 解析 query params：`ImageId`, `Page`(默认1), `PageSize`(默认24), `Status`, `Correction`
- 按 ImageId 精确查询模式：与任务列表的 Id 精确查询模式一致，在 Controller 层判断后调用不同 Service 方法

### 5.2 UploadImage

```
POST /api/tasks/:id/images (multipart/form-data)
```

- 解析 path param `id`
- 读取 `file` 字段（单文件）
- 调用 `svc.UploadMissedPhoto(ctx, taskId, file)`

### 5.3 UpdateCorrection

```
PUT /api/tasks/:id/images/:imageId
```

- 解析 path params：`id`, `imageId`
- 绑定 JSON body 到 `UpdateCorrectionReq`
- 调用 `svc.MarkCorrection(ctx, taskId, imageId, req.Correction)`

## 6. 路由注册

在 `router.go` 的 tasks group 中添加：

```go
tasks.GET("/:id/images", taskCtrl.ListImages)
tasks.POST("/:id/images", taskCtrl.UploadImage)
tasks.PUT("/:id/images/:imageId", taskCtrl.UpdateCorrection)
```

## 7. 错误码

| 错误码 | 名称 | 说明 |
|--------|------|------|
| 40003 | ErrImageNotFound | 素材不存在（ImageId 无效或不在该任务下） |

复用已有错误码，不新增。`ErrTaskNotFound`(40002) 和 `ErrParamValidation`(40001) 也需复用。

## 8. 关键设计决策

| 决策 | 选择 | 原因 |
|------|------|------|
| 矫正接口 HTTP 方法 | PUT（非文档的 PATCH） | 与任务暂停/恢复接口风格一致 |
| 补录照片 Status | Detected | 补录即视为已检出目标 |
| 补录照片 Detection | null | 未经过大模型，无检测结果 |
| Correction 值域 | null / "FalsePositive" / "DeletedFp" | 保持 API 文档原样 |
| Correction 恢复 | 任意状态可直接恢复 | 不要求逐步恢复，简化操作 |
| 默认排除 DeletedFp | 是 | 列表默认不显示已删除误报，需显式传 Correction=DeletedFp 查看 |
| 素材排序 | created_at ASC | 与素材文件列表一致，最早上传的在前 |
