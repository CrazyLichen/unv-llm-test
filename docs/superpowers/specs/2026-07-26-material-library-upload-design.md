# 素材库与文件上传改造设计

> 日期：2026-07-26
> 基于 `llm-test-api-docs.md` v2.1 的增量改造

---

## 1. 背景与目标

### 问题

当前 API 设计假设前后端同机部署，素材通过本地路径（`ImagePaths` / `VideoPath`）指定。在浏览器环境下，前端无法获取本地文件路径，导致该设计不可行。

### 目标

1. 新增**素材库（MaterialLibrary）**领域，支持用户独立管理图片和视频素材
2. 提供文件上传接口：图片批量上传、视频分片上传
3. 任务创建时通过素材库 ID 关联素材，不再使用本地路径
4. 结果查询时返回浏览器可访问的 URL，替代本地路径

### 约束

- 支持远程部署（前后端可不在同一机器）
- 文件存储在后端本地磁盘
- 前端通过静态文件 URL 访问图片/视频
- 素材库与任务 1:1 绑定，创建时关联，不可更换

---

## 2. 数据模型

### 2.1 MaterialLibrary（素材库）

| 字段 | 类型 | 说明 |
|------|------|------|
| Id | string | 素材库唯一标识 |
| Name | string | 素材库名称 |
| Type | string | 素材类型：`Image` 图片集 / `Video` 视频集 |
| Description | string \| null | 描述 |
| FileCount | int32 | 文件数量 |
| TotalSize | int64 | 文件总大小（字节） |
| CreatedAt | string | 创建时间 |
| UpdatedAt | string | 最后更新时间 |

### 2.2 MaterialFile（素材文件）

| 字段 | 类型 | 说明 |
|------|------|------|
| Id | string | 文件唯一标识 |
| LibraryId | string | 所属素材库 ID |
| FileName | string | 原始文件名 |
| StoragePath | string | 后端存储路径（相对路径，程序内部管理） |
| AccessUrl | string | 浏览器访问 URL |
| FileSize | int64 | 文件大小（字节） |
| MimeType | string | MIME 类型 |
| UploadStatus | string | 上传状态：`Uploading` / `Merging` / `Completed` / `Failed` |
| FailReason | string \| null | 失败原因（UploadStatus=Failed 时有值） |
| Progress | float | 上传进度（0-1），Uploading 时表示分片上传进度，Merging 时固定为 1，Completed/Failed 时为 1 |
| TotalChunks | int32 \| null | 分片总数（仅视频文件有值） |
| UploadedChunks | int32 \| null | 已上传分片数（仅视频文件有值） |
| CreatedAt | string | 创建时间 |

### 2.3 Task 模型变更

| 变更 | 字段 | 说明 |
|------|------|------|
| 移除 | `VideoPath` | 不再使用本地路径 |
| 新增 | `MaterialLibraryId` | 关联素材库 ID |
| 保留 | `FrameInterval` | Type=Video 时必填，任务分析时按此间隔抽帧 |

### 2.4 Image 模型变更

| 变更 | 字段 | 说明 |
|------|------|------|
| 移除 | `Path` | 不再返回本地路径 |
| 新增 | `AccessUrl` | 浏览器可访问的图片 URL |
| 新增 | `MaterialFileId` | 关联原始素材文件 ID（图片集任务有值，视频帧为 null） |
| 保留 | `FrameIndex` | 视频抽帧素材有值 |

---

## 3. 存储设计

### 3.1 目录结构

```
data/
├── uploads/
│   ├── images/
│   │   └── {library_id}/
│   │       ├── {file_id}.jpg
│   │       └── {file_id}.png
│   └── videos/
│       └── {library_id}/
│           ├── {file_id}.mp4
│           └── chunks/               # 分片临时目录
│               ├── {file_id}.part.0
│               └── {file_id}.part.1
├── tasks/
│   └── {task_id}/
│       └── frames/                   # 任务抽帧目录
│           ├── frame_0001.jpg
│           └── frame_0002.jpg
```

### 3.2 静态文件 URL 规则

Gin 配置 `StaticFS` 将 URL 路径映射到磁盘目录：

| URL 前缀 | 磁盘路径 | 说明 |
|----------|----------|------|
| `/uploads/images/` | `data/uploads/images/` | 图片素材 |
| `/uploads/videos/` | `data/uploads/videos/` | 视频素材 |
| `/tasks/` | `data/tasks/` | 任务抽帧 |

- 图片素材：`/uploads/images/{library_id}/{file_id}.ext`
- 视频素材：`/uploads/videos/{library_id}/{file_id}.ext`
- 任务抽帧：`/tasks/{task_id}/frames/frame_XXXX.jpg`

---

## 4. 接口设计

### 4.1 素材库管理

#### 4.1.1 创建素材库

- **URL**：`POST /api/material-libraries`
- **Content-Type**：`application/json`

##### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Name | string | 是 | 素材库名称 |
| Type | string | 是 | 素材类型：`Image` / `Video` |
| Description | string | 否 | 描述 |

##### Request Example

```json
{
  "Name": "沿街摆摊图片集",
  "Type": "Image",
  "Description": "第一批采集的沿街摆摊图片"
}
```

##### Response

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "Id": "ml_001",
    "Name": "沿街摆摊图片集",
    "Type": "Image",
    "Description": "第一批采集的沿街摆摊图片",
    "FileCount": 0,
    "TotalSize": 0,
    "CreatedAt": "2026-07-26 10:00:00",
    "UpdatedAt": "2026-07-26 10:00:00"
  }
}
```

---

#### 4.1.2 获取素材库列表

- **URL**：`GET /api/material-libraries`

##### Query Parameters

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Id | string | 否 | 按 ID 精确查询，忽略分页，返回单条 |
| Page | int32 | 否 | 页码，默认 1 |
| PageSize | int32 | 否 | 每页数量，默认 20 |
| Type | string | 否 | 按素材类型筛选：`Image` / `Video` |

##### Response

分页结构，`Items` 每项结构见 MaterialLibrary。

---

#### 4.1.3 更新素材库

- **URL**：`PUT /api/material-libraries/:id`
- **Content-Type**：`application/json`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |

##### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Name | string | 是 | 素材库名称 |
| Description | string | 否 | 描述 |

##### Response

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

---

#### 4.1.4 删除素材库

- **URL**：`DELETE /api/material-libraries/:id`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |

##### Response

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

##### 后端行为说明

- 级联删除：同时删除素材库下所有文件记录和磁盘文件
- 若素材库已关联到任务，则拒绝删除（返回 ErrorCode=40007）

---

#### 4.1.5 批量上传图片

- **URL**：`POST /api/material-libraries/:id/images`
- **Content-Type**：`multipart/form-data`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |

##### Request

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| files | file[] | 是 | 图片文件列表 |

##### 上传限制

| 限制项 | 值 |
|--------|-----|
| 单个文件最大 | 10MB |
| 单次最多上传 | 20 张 |
| 请求总体积最大 | 50MB |

##### Response

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "UploadedCount": 5,
    "Files": [
      {
        "Id": "mf_001",
        "FileName": "img_001.jpg",
        "AccessUrl": "/uploads/images/ml_001/mf_001.jpg",
        "FileSize": 204800,
        "MimeType": "image/jpeg"
      }
    ]
  }
}
```

##### 后端行为说明

- 逐个遍历上传文件，用 `io.Copy` 从上传流直接写入磁盘，不在内存中缓存全部文件
- 每个文件处理完立即释放，内存占用约等于单个文件大小
- 前端分批调用：如用户选了 100 张图片，前端自动分 5 批（每批 20 张）调用

---

#### 4.1.6 上传视频（分片）

分三步：初始化 → 上传分片 → 合并完成。

##### 4.1.6.1 初始化视频上传

- **URL**：`POST /api/material-libraries/:id/videos/init`
- **Content-Type**：`application/json`

###### 后端行为说明

1. 校验素材库是否存在且类型为 Video
2. 校验素材库下是否已存在同名且 UploadStatus 为 Completed 或 Merging 的文件，若存在则拒绝（返回 ErrorCode=40001）
3. 检查素材库下是否已存在同名且 UploadStatus 为 Uploading 的文件（断点续传）：
   - 若存在，返回已有的 UploadId 和 ChunkCount，前端可继续上传未完成的分片
   - 若不存在，创建新的 MaterialFile 记录，UploadStatus 设为 `Uploading`，TotalChunks 设为计算出的分片数，UploadedChunks 设为 0，创建分片临时目录
4. 返回 UploadId 和 ChunkCount

###### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| FileName | string | 是 | 视频文件名 |
| FileSize | int64 | 是 | 文件总大小（字节） |
| ChunkSize | int32 | 是 | 分片大小（字节） |

###### Request Example

```json
{
  "FileName": "cam_01.mp4",
  "FileSize": 52428800,
  "ChunkSize": 5242880
}
```

###### Response

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "UploadId": "upload_abc123",
    "ChunkCount": 10
  }
}
```

---

##### 4.1.6.2 上传分片

- **URL**：`POST /api/material-libraries/:id/videos/chunk`
- **Content-Type**：`multipart/form-data`

###### Request

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| UploadId | string | 是 | 上传标识 |
| ChunkIndex | int32 | 是 | 分片序号（从 0 开始） |
| file | file | 是 | 分片数据 |

###### Response

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

---

##### 4.1.6.3 完成上传（合并分片）

- **URL**：`POST /api/material-libraries/:id/videos/complete`
- **Content-Type**：`application/json`

###### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| UploadId | string | 是 | 上传标识 |

###### Response

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

> 异步合并：接口立即返回，合并进度通过素材库详情接口查询。

---

##### 4.1.6.3 完成上传 — 后端行为说明

1. 校验所有分片是否已上传完毕
2. 将 MaterialFile 的 UploadStatus 设为 `Merging`，更新 UploadedChunks 等于 TotalChunks
3. 立即返回响应，不等待合并完成
4. 后端异步执行合并：按序合并分片为完整视频文件
5. 合并成功：删除分片临时文件，更新 MaterialFile UploadStatus 为 `Completed`，清除 TotalChunks/UploadedChunks
6. 合并失败：更新 MaterialFile UploadStatus 为 `Failed`，FailReason 记录失败原因
7. 前端通过查询素材库文件列表获取各文件的上传状态和分片进度

---

#### 4.1.7 获取素材库文件列表

- **URL**：`GET /api/material-libraries/:id/files`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |

##### Query Parameters

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Page | int32 | 否 | 页码，默认 1 |
| PageSize | int32 | 否 | 每页数量，默认 24 |
| UploadStatus | string | 否 | 按上传状态筛选 |

##### 排序规则

默认按创建时间升序排列（最早上传的在前）。

##### Response

分页结构，`Items` 每项结构见 MaterialFile。

---

#### 4.1.8 删除素材文件

- **URL**：`DELETE /api/material-libraries/:id/files/:fileId`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |
| fileId | string | 文件 ID |

##### Response

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

##### 后端行为说明

- 删除文件记录和磁盘文件
- 素材库的 FileCount / TotalSize 自动调整

---

### 4.2 任务接口变更

#### 4.2.1 创建任务 — 变更

- **URL**：`POST /api/tasks`

##### Request Body 变更

| 字段 | 变更 | 必填 | 说明 |
|------|------|------|------|
| ~~ImagePaths~~ | 移除 | — | 不再传本地路径 |
| ~~VideoPath~~ | 移除 | — | 不再传本地路径 |
| MaterialLibraryId | 新增 | 是 | 关联素材库 ID |
| FrameInterval | 保留 | Type=Video 时必填 | 任务分析时按此间隔抽帧 |

##### Request Example — 图片集任务

```json
{
  "Name": "沿街摆摊检测-批次1",
  "Type": "Image",
  "ModelConfigId": "mc_001",
  "MaterialLibraryId": "ml_001",
  "Prompt": "请检测图片中是否有沿街摆摊行为，返回JSON格式...",
  "Target": "沿街摆摊"
}
```

##### Request Example — 视频集任务

```json
{
  "Name": "消防通道占用-小区夜查",
  "Type": "Video",
  "ModelConfigId": "mc_002",
  "MaterialLibraryId": "ml_002",
  "Prompt": "检测消防通道是否被杂物、电动车、纸箱等物品占用，若存在占用物请标注其位置边界。",
  "Target": "消防通道占用物",
  "FrameInterval": 2
}
```

##### 后端行为变更

1. 校验 MaterialLibraryId 是否存在
2. 校验素材库类型是否与 Task.Type 一致
3. 校验素材库是否已被其他任务关联（1:1 绑定）
4. 校验素材库中是否存在 UploadStatus 不为 Completed 的文件，若存在则拒绝创建（返回 ErrorCode=40011）
5. **Type=Image**：为素材库下所有图片创建 Image 记录
6. **Type=Video**：不立即创建 Image 记录，任务开始分析后按 FrameInterval 抽帧生成
7. 其余逻辑不变

---

#### 4.2.2 获取任务列表 — 变更

TaskItem 变更：

| 字段 | 变更 | 说明 |
|------|------|------|
| MaterialLibraryId | 新增 | 关联素材库 ID |
| ~~VideoPath~~ | 移除 | 不再返回本地路径 |
| FrameInterval | 保留 | 视频任务抽帧间隔 |

---

#### 4.2.3 更新任务 — 变更

- **URL**：`PUT /api/tasks/:id`

接口行为不变，仅方法从 PATCH 改为 PUT。

---

#### 4.2.4 获取任务分析结果 — 变更

- **URL**：`GET /api/tasks/:id/images`

Image 响应变更：

| 字段 | 变更 | 说明 |
|------|------|------|
| ~~Path~~ | 移除 | 不再返回本地路径 |
| AccessUrl | 新增 | 浏览器可访问的图片 URL |
| MaterialFileId | 新增 | 关联原始素材文件 ID（图片集有值，视频帧为 null） |

##### Response Example — 图片集素材

```json
{
  "Id": "img_001",
  "TaskId": "task_20260726_001",
  "AccessUrl": "/uploads/images/ml_001/mf_001.jpg",
  "MaterialFileId": "mf_001",
  "FrameIndex": null,
  "Status": "Detected",
  "Detection": { ... },
  "FailReason": null,
  "Correction": null
}
```

##### Response Example — 视频帧

```json
{
  "Id": "img_050",
  "TaskId": "task_20260726_002",
  "AccessUrl": "/tasks/task_20260726_002/frames/frame_0001.jpg",
  "MaterialFileId": null,
  "FrameIndex": 1,
  "Status": "Detected",
  "Detection": { ... },
  "FailReason": null,
  "Correction": null
}
```

---

#### 4.2.5 补录漏报照片 — 变更

- **URL**：`POST /api/tasks/:id/images`
- **Content-Type**：`multipart/form-data`（变更）

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 任务 ID |

##### Request

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 图片文件 |

##### Response

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

##### 后端行为说明

- 上传图片到任务帧目录（`data/tasks/{task_id}/frames/`）
- 创建 Image 记录，AccessUrl 指向该文件

---

#### 4.2.6 其他任务接口（无变更）

- **删除任务** `DELETE /api/tasks/:id`：级联删除 Image 记录 + 任务帧目录
- **标记素材误报** `PATCH /api/tasks/:id/images/:imageId`：行为不变

---

### 4.3 模型配置接口变更

所有更新接口方法从 PATCH 改为 PUT：

- `PUT /api/model-configs/:id`（原 PATCH）

---

## 5. 错误码补充

| ErrorCode | ErrorMsg | 说明 |
|-----------|----------|------|
| 40007 | 素材库不存在 | MaterialLibraryId 无效 |
| 40008 | 素材库已被任务关联 | 素材库 1:1 绑定，不可重复关联 |
| 40009 | 素材库类型与任务类型不匹配 | Image 素材库只能关联 Image 任务 |
| 40010 | 文件不存在 | MaterialFileId 无效 |
| 40011 | 文件上传未完成 | 素材库中有未完成上传的文件 |
| 50003 | 文件上传失败 | 服务端写入文件异常 |
| 50004 | 分片上传异常 | 分片校验失败、合并异常等 |

---

## 6. 接口总览

| 序号 | 方法 | 路径 | 说明 |
|------|------|------|------|
| | | | **模型配置管理** |
| 1 | POST | `/api/model-configs` | 创建模型配置 |
| 2 | GET | `/api/model-configs` | 获取模型配置列表 |
| 3 | PUT | `/api/model-configs/:id` | 更新模型配置 |
| 4 | DELETE | `/api/model-configs/:id` | 删除模型配置 |
| 5 | POST | `/api/model-configs/:id/test` | 测试模型连通性 |
| | | | **素材库管理** |
| 6 | POST | `/api/material-libraries` | 创建素材库 |
| 7 | GET | `/api/material-libraries` | 获取素材库列表 |
| 8 | PUT | `/api/material-libraries/:id` | 更新素材库 |
| 9 | DELETE | `/api/material-libraries/:id` | 删除素材库 |
| 10 | POST | `/api/material-libraries/:id/images` | 批量上传图片 |
| 11 | POST | `/api/material-libraries/:id/videos/init` | 初始化视频上传 |
| 12 | POST | `/api/material-libraries/:id/videos/chunk` | 上传视频分片 |
| 13 | POST | `/api/material-libraries/:id/videos/complete` | 完成视频上传 |
| 14 | GET | `/api/material-libraries/:id/files` | 获取素材文件列表 |
| 15 | DELETE | `/api/material-libraries/:id/files/:fileId` | 删除素材文件 |
| | | | **任务管理** |
| 16 | POST | `/api/tasks` | 创建任务 |
| 17 | GET | `/api/tasks` | 获取任务列表 |
| 18 | DELETE | `/api/tasks/:id` | 删除任务 |
| 19 | PUT | `/api/tasks/:id` | 暂停/恢复任务 |
| 20 | GET | `/api/tasks/:id/images` | 获取任务分析结果 |
| 21 | POST | `/api/tasks/:id/images` | 补录漏报照片 |
| 22 | PATCH | `/api/tasks/:id/images/:imageId` | 标记素材误报 |
