# 大模型检测效果评估平台 — API 接口文档

> 版本：v3.0
> 更新日期：2026-07-26

---

## 1. 概述

### 1.1 约定

| 项目 | 说明 |
|------|------|
| 协议 | HTTP/1.1 |
| 基础路径 | `/api` |
| 字段命名 | PascalCase（首字母大写） |
| 字符编码 | UTF-8 |
| 时间格式 | `yyyy-MM-dd HH:mm:ss`，示例：`2025-10-28 12:00:00` |
| 整数类型 | 统一使用 `int32` |
| 前后端部署 | 支持远程访问，素材通过文件上传接口提交 |

### 1.2 统一响应结构

所有接口统一返回以下 JSON 结构：

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": { ... }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码，`0` 表示成功 |
| ErrorMsg | string | 错误描述，成功时为空字符串 |
| Data | object \| null | 具体业务数据，失败时为 `null` |

### 1.3 分页响应结构

列表接口的 `Data` 统一为分页结构：

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "Total": 15,
    "Page": 1,
    "PageSize": 20,
    "Items": [ ... ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| Total | int32 | 总记录数 |
| Page | int32 | 当前页码 |
| PageSize | int32 | 每页数量 |
| Items | array | 数据列表 |

### 1.4 错误码定义

| ErrorCode | ErrorMsg | 说明 |
|-----------|----------|------|
| 0 | — | 成功 |
| 40001 | 参数校验失败 | 请求参数不合法 |
| 40002 | 任务不存在 | TaskId 无效 |
| 40003 | 素材不存在 | ImageId 无效 |
| 40005 | 任务状态不允许此操作 | 如对已完成任务执行不合理的操作 |
| 40006 | 模型配置不存在 | ModelConfigId 无效 |
| 40007 | 素材库不存在 | MaterialLibraryId 无效 |
| 40008 | 素材库已被任务关联 | 素材库关联任务时，不允许删除文件 |
| 40009 | 素材库类型与任务类型不匹配 | Image 素材库只能关联 Image 任务 |
| 40010 | 文件不存在 | MaterialFileId 无效 |
| 40011 | 文件上传未完成 | 素材库中有未完成上传的文件 |
| 50001 | 大模型调用失败 | 检测过程中模型调用异常 |
| 50002 | 视频抽帧失败 | 视频文件无法解析或抽帧异常 |
| 50003 | 文件上传失败 | 服务端写入文件异常 |
| 50004 | 分片上传异常 | 分片校验失败、合并异常等 |

### 1.5 枚举值定义

#### TaskType — 任务类型

| 值 | 中文名称 | 说明 |
|----|----------|------|
| Image | 图片集 | 素材为一批图片 |
| Video | 视频集 | 素材为一段视频，后端自动抽帧 |

#### TaskStatus — 任务状态

| 值 | 中文名称 | 说明 |
|----|----------|------|
| Pending | 等待中 | 任务已创建，排队等待执行 |
| Analyzing | 检测中 | 任务正在执行检测（含视频抽帧） |
| Paused | 暂停中 | 任务已暂停，可恢复继续检测 |
| Completed | 已完成 | 全部素材检测完毕 |
| Failed | 已失败 | 任务执行失败（如视频抽帧失败、无待检测素材、模型配置不存在），附带 FailReason，可通过 PUT 恢复 |

#### ImageStatus — 素材检测状态

| 值 | 中文名称 | 说明 |
|----|----------|------|
| Pending | 待检测 | 等待调用大模型 |
| Detected | 已检出 | 大模型检测到目标 |
| NotDetected | 未检出 | 大模型未检测到目标 |
| Failed | 检测失败 | 大模型调用异常，附带失败原因 |

#### LibraryType — 素材库类型

| 值 | 中文名称 | 说明 |
|----|----------|------|
| Image | 图片集 | 素材库包含一批图片 |
| Video | 视频集 | 素材库包含一段视频 |

#### UploadStatus — 文件上传状态

| 值 | 中文名称 | 说明 |
|----|----------|------|
| Uploading | 上传中 | 文件正在上传（含分片上传中） |
| Merging | 合并中 | 分片上传完毕，后端正在异步合并分片 |
| Completed | 已完成 | 文件上传完成（含合并完成） |
| Failed | 上传失败 | 文件上传或合并异常，附带失败原因 |

---

## 2. 数据模型

### 2.1 ModelConfig（模型配置）

| 字段 | 类型 | 说明 |
|------|------|------|
| Id | string | 配置唯一标识 |
| Name | string | 用户自定义配置名称 |
| ApiUrl | string | 大模型 API 地址 |
| ModelName | string | 模型名称，如 `gpt-4o`、`qwen-vl-max` |
| ApiKey | string | API 访问密钥 |
| Temperature | float | 温度参数（0-2），控制生成随机性 |
| MaxTokens | int32 | 最大生成 token 数 |
| CreatedAt | string | 创建时间 |
| UpdatedAt | string | 最后更新时间 |

### 2.2 MaterialLibrary（素材库）

| 字段 | 类型 | 说明 |
|------|------|------|
| Id | string | 素材库唯一标识 |
| Name | string | 素材库名称 |
| Type | string | 素材类型，见 [LibraryType](#librarytype--素材库类型) |
| Description | string \| null | 描述 |
| FileCount | int32 | 文件数量 |
| TotalSize | int64 | 文件总大小（字节） |
| CreatedAt | string | 创建时间 |
| UpdatedAt | string | 最后更新时间 |

### 2.3 MaterialFile（素材文件）

| 字段 | 类型 | 说明 |
|------|------|------|
| Id | string | 文件唯一标识 |
| LibraryId | string | 所属素材库 ID |
| FileName | string | 原始文件名 |
| StoragePath | string | 后端存储路径（相对路径，程序内部管理） |
| AccessUrl | string | 浏览器访问 URL |
| FileSize | int64 | 文件大小（字节） |
| MimeType | string | MIME 类型 |
| UploadStatus | string | 上传状态，见 [UploadStatus](#uploadstatus--文件上传状态) |
| FailReason | string \| null | 失败原因（UploadStatus=Failed 时有值） |
| Progress | float | 上传进度（0-1），Uploading 时表示分片上传进度，Merging 时固定为 1，Completed/Failed 时为 1 |
| TotalChunks | int32 \| null | 分片总数（仅视频文件有值） |
| UploadedChunks | int32 \| null | 已上传分片数（仅视频文件有值） |
| CreatedAt | string | 创建时间 |

### 2.4 Task（分析任务）

| 字段 | 类型 | 说明 |
|------|------|------|
| Id | string | 任务唯一标识 |
| Name | string | 任务名称 |
| Type | string | 任务类型，见 [TaskType](#tasktype--任务类型) |
| Status | string | 任务状态，见 [TaskStatus](#taskstatus--任务状态) |
| ModelConfigId | string | 使用的模型配置 ID |
| ModelConfigName | string | 使用的模型配置名称 |
| MaterialLibraryId | string | 关联素材库 ID |
| MaterialLibraryName | string | 关联素材库名称 |
| Prompt | string | 下发给大模型的提示词 |
| Target | string | 检测目标名称 |
| FrameInterval | int32 \| null | 抽帧间隔，单位秒（Type=Video 时有值） |
| FailReason | string \| null | 失败原因（Status=Failed 时有值） |
| Progress | object | 检测进度与统计，见 [Progress](#25-progress检测进度与统计) |
| CreatedAt | string | 创建时间 |

### 2.5 Progress（检测进度与统计）

结构化描述检测进度与结果分布：

```
Total: 48
├── Completed: 23
│   ├── Detected: 15
│   │   ├── TruePositive: 12  (正报)
│   │   └── FalsePositive: 3  (误报)
│   ├── NotDetected: 6
│   └── Failed: 2
└── Pending: 25
```

| 字段 | 类型 | 说明 |
|------|------|------|
| Total | int32 | 素材总数 |
| Completed | int32 | 已完成检测数（含成功和失败） |
| CompletedDetail | object | 已完成明细，见 [CompletedDetail](#251-completeeddetail已完成明细) |
| Pending | int32 | 待检测数 |

#### 2.5.1 CompletedDetail（已完成明细）

| 字段 | 类型 | 说明 |
|------|------|------|
| Detected | int32 | 已检出目标数 |
| DetectedDetail | object | 检出明细，见 [DetectedDetail](#252-detecteddetail检出明细) |
| NotDetected | int32 | 未检出目标数 |
| Failed | int32 | 检测失败数 |

#### 2.5.2 DetectedDetail（检出明细）

| 字段 | 类型 | 说明 |
|------|------|------|
| TruePositive | int32 | 正报数（检出且未被标记误报） |
| FalsePositive | int32 | 误报数（检出且被标记为误报） |

> 约束：
> - `Detected = DetectedDetail.TruePositive + DetectedDetail.FalsePositive`
> - `Completed = CompletedDetail.Detected + CompletedDetail.NotDetected + CompletedDetail.Failed`
> - `Total = Completed + Pending`

### 2.6 Image（检测素材）

| 字段 | 类型 | 说明 |
|------|------|------|
| Id | string | 素材唯一标识 |
| TaskId | string | 所属任务 ID |
| AccessUrl | string | 图片/帧浏览器访问 URL |
| MaterialFileId | string \| null | 关联原始素材文件 ID（图片集任务有值，视频帧为 null） |
| FrameIndex | int32 \| null | 视频帧序号（视频抽帧素材有值） |
| Status | string | 检测状态，见 [ImageStatus](#imagestatus--素材检测状态) |
| Detection | object \| null | 检测结果，见 [Detection](#27-detection检测结果) |
| FailReason | string \| null | 失败原因（Status=Failed 时有值） |
| Correction | string \| null | 矫正标记：`null` 正常 \| `"FalsePositive"` 误报 \| `"DeletedFp"` 已删除误报 |

### 2.7 Detection（检测结果）

| 字段 | 类型 | 说明 |
|------|------|------|
| HasTarget | bool | 是否检测到目标 |
| Boxes | array | 检测框列表，见 [Box](#28-box检测框) |
| RawResponse | string | 大模型原始 JSON 返回 |
| AnalyzedAt | string | 分析完成时间 |

### 2.8 Box（检测框）

| 字段 | 类型 | 说明 |
|------|------|------|
| X1 | int32 | 左上角 X 坐标（0-1000 归一化） |
| Y1 | int32 | 左上角 Y 坐标（0-1000 归一化） |
| X2 | int32 | 右下角 X 坐标（0-1000 归一化） |
| Y2 | int32 | 右下角 Y 坐标（0-1000 归一化） |
| Confidence | string | 置信度描述（如 "high"、"medium"、"low"，来自大模型返回的 confidence_note） |
| Label | string | 目标标签 |

---

## 3. 接口详情

### 3.1 模型配置管理

#### 3.1.1 创建模型配置

- **URL**：`POST /api/model-configs`
- **Content-Type**：`application/json`

##### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Name | string | 是 | 用户自定义配置名称 |
| ApiUrl | string | 是 | 大模型 API 地址 |
| ModelName | string | 是 | 模型名称 |
| ApiKey | string | 是 | API 访问密钥 |
| Temperature | float | 否 | 温度参数，默认 0.7 |
| MaxTokens | int32 | 否 | 最大生成 token 数，默认 4096 |

##### Request Example

```json
{
  "Name": "GPT-4o 视觉检测",
  "ApiUrl": "https://api.openai.com/v1/chat/completions",
  "ModelName": "gpt-4o",
  "ApiKey": "sk-xxxxx",
  "Temperature": 0.7,
  "MaxTokens": 4096
}
```

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

---

#### 3.1.2 获取模型配置列表

- **URL**：`GET /api/model-configs`

##### Query Parameters

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Id | string | 否 | 配置 ID，传此参数时按 ID 精确查询，忽略分页参数，返回单条结果 |
| Page | int32 | 否 | 页码，默认 1 |
| PageSize | int32 | 否 | 每页数量，默认 20 |

##### Request Example

```
GET /api/model-configs?Page=1&PageSize=20
```

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | object \| null | 分页数据；配置不存在时为 `null` |
| Data.Total | int32 | 总记录数 |
| Data.Page | int32 | 当前页码 |
| Data.PageSize | int32 | 每页数量 |
| Data.Items | array | 配置列表，每项结构见 [ModelConfig](#21-modelconfig模型配置) |

##### Response Example

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "Total": 3,
    "Page": 1,
    "PageSize": 20,
    "Items": [
      {
        "Id": "mc_001",
        "Name": "GPT-4o 视觉检测",
        "ApiUrl": "https://api.openai.com/v1/chat/completions",
        "ModelName": "gpt-4o",
        "ApiKey": "sk-****",
        "Temperature": 0.7,
        "MaxTokens": 4096,
        "CreatedAt": "2026-07-26 09:00:00",
        "UpdatedAt": "2026-07-26 09:00:00"
      },
      {
        "Id": "mc_002",
        "Name": "通义千问-VL",
        "ApiUrl": "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
        "ModelName": "qwen-vl-max",
        "ApiKey": "sk-****",
        "Temperature": 0.5,
        "MaxTokens": 2048,
        "CreatedAt": "2026-07-26 09:10:00",
        "UpdatedAt": "2026-07-26 09:10:00"
      }
    ]
  }
}
```

---

#### 3.1.3 更新模型配置

- **URL**：`PUT /api/model-configs/:id`
- **Content-Type**：`application/json`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 配置 ID |

##### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Name | string | 否 | 用户自定义配置名称 |
| ApiUrl | string | 否 | 大模型 API 地址 |
| ModelName | string | 否 | 模型名称 |
| ApiKey | string | 否 | API 访问密钥 |
| Temperature | float | 否 | 温度参数 |
| MaxTokens | int32 | 否 | 最大生成 token 数 |

> 仅传需要更新的字段，未传的字段保持不变。

##### Request Example

```json
{
  "Name": "GPT-4o 视觉检测-V2",
  "Temperature": 0.5
}
```

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

---

#### 3.1.4 删除模型配置

- **URL**：`DELETE /api/model-configs/:id`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 配置 ID |

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

##### 后端行为说明

- 删除模型配置前需检查是否有关联任务正在使用，若存在关联任务则拒绝删除（返回 ErrorCode=40005）

---

#### 3.1.5 测试模型连通性

使用指定模型配置发送一次简单的测试请求，验证模型 API 的连通性和可用性。

- **URL**：`POST /api/model-configs/:id/test`
- **Content-Type**：`application/json`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 配置 ID |

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码，`0` 表示连通成功 |
| ErrorMsg | string | 错误描述，连通成功时为空字符串；失败时返回具体失败原因 |
| Data | object \| null | 连通测试结果 |
| Data.Latency | int32 | 响应耗时，单位毫秒（仅连通成功时有值） |

##### Response Example — 连通成功

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "Latency": 523
  }
}
```

##### Response Example — 连通失败

```json
{
  "ErrorCode": 50001,
  "ErrorMsg": "Connection refused: 无法连接到 https://api.openai.com/v1/chat/completions",
  "Data": null
}
```

##### 后端行为说明

1. 根据配置 ID 查询模型配置（ApiUrl、ModelName、ApiKey 等）
2. 使用该配置向大模型 API 发送一条简短的测试请求（如 "Hello"），验证网络连通性和 API 可用性
3. 连通成功：返回响应耗时 Latency
4. 连通失败：返回 ErrorCode=50001（大模型调用失败），ErrorMsg 包含具体失败原因（如网络不可达、API Key 无效、模型不存在等）

---

### 3.2 素材库管理

#### 3.2.1 创建素材库

- **URL**：`POST /api/material-libraries`
- **Content-Type**：`application/json`

##### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Name | string | 是 | 素材库名称 |
| Type | string | 是 | 素材类型，见 [LibraryType](#librarytype--素材库类型) |
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

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | object \| null | 创建的素材库信息 |

##### Response Example

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

#### 3.2.2 获取素材库列表

- **URL**：`GET /api/material-libraries`

##### Query Parameters

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Id | string | 否 | 按 ID 精确查询，忽略分页参数，返回单条结果 |
| Page | int32 | 否 | 页码，默认 1 |
| PageSize | int32 | 否 | 每页数量，默认 20 |
| Type | string | 否 | 按素材类型筛选，见 [LibraryType](#librarytype--素材库类型) |

##### Request Example

```
GET /api/material-libraries?Page=1&PageSize=20&Type=Image
```

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | object \| null | 分页数据；素材库不存在时为 `null` |
| Data.Total | int32 | 总记录数 |
| Data.Page | int32 | 当前页码 |
| Data.PageSize | int32 | 每页数量 |
| Data.Items | array | 素材库列表，每项结构见 [MaterialLibrary](#22-materiallibrary素材库) |

##### Response Example

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "Total": 2,
    "Page": 1,
    "PageSize": 20,
    "Items": [
      {
        "Id": "ml_001",
        "Name": "沿街摆摊图片集",
        "Type": "Image",
        "Description": "第一批采集的沿街摆摊图片",
        "FileCount": 48,
        "TotalSize": 104857600,
        "CreatedAt": "2026-07-26 10:00:00",
        "UpdatedAt": "2026-07-26 11:00:00"
      },
      {
        "Id": "ml_002",
        "Name": "消防通道视频",
        "Type": "Video",
        "Description": null,
        "FileCount": 1,
        "TotalSize": 52428800,
        "CreatedAt": "2026-07-26 10:05:00",
        "UpdatedAt": "2026-07-26 10:10:00"
      }
    ]
  }
}
```

---

#### 3.2.3 更新素材库

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

##### Request Example

```json
{
  "Name": "沿街摆摊图片集-第二批",
  "Description": "更新后的描述"
}
```

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

---

#### 3.2.4 删除素材库

- **URL**：`DELETE /api/material-libraries/:id`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

##### 后端行为说明

- 级联删除：同时删除素材库下所有文件记录和磁盘文件
- 若素材库已关联到任务，则拒绝删除（返回 ErrorCode=40008）

---

#### 3.2.5 批量上传图片

向指定素材库批量上传图片文件。前端可分批调用，每批上传不超过 20 张图片。

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

##### Request Example

```bash
curl -X POST http://localhost:8080/api/material-libraries/ml_001/images \
  -F "files=@img_001.jpg" \
  -F "files=@img_002.jpg" \
  -F "files=@img_003.png"
```

##### 上传限制

| 限制项 | 值 |
|--------|-----|
| 单个文件最大 | 10MB |
| 单次最多上传 | 20 张 |
| 请求总体积最大 | 50MB |

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | object \| null | 上传结果 |
| Data.UploadedCount | int32 | 成功上传数量 |
| Data.Files | array | 上传文件列表，每项结构见 [MaterialFile](#23-materialfile素材文件) |

##### Response Example

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "UploadedCount": 3,
    "Files": [
      {
        "Id": "mf_001",
        "LibraryId": "ml_001",
        "FileName": "img_001.jpg",
        "StoragePath": "images/ml_001/mf_001.jpg",
        "AccessUrl": "/uploads/images/ml_001/mf_001.jpg",
        "FileSize": 204800,
        "MimeType": "image/jpeg",
        "UploadStatus": "Completed",
        "FailReason": null,
        "Progress": 1,
        "TotalChunks": null,
        "UploadedChunks": null,
        "CreatedAt": "2026-07-26 10:01:00"
      },
      {
        "Id": "mf_002",
        "LibraryId": "ml_001",
        "FileName": "img_002.jpg",
        "StoragePath": "images/ml_001/mf_002.jpg",
        "AccessUrl": "/uploads/images/ml_001/mf_002.jpg",
        "FileSize": 153600,
        "MimeType": "image/jpeg",
        "UploadStatus": "Completed",
        "FailReason": null,
        "Progress": 1,
        "TotalChunks": null,
        "UploadedChunks": null,
        "CreatedAt": "2026-07-26 10:01:00"
      },
      {
        "Id": "mf_003",
        "LibraryId": "ml_001",
        "FileName": "img_003.png",
        "StoragePath": "images/ml_001/mf_003.png",
        "AccessUrl": "/uploads/images/ml_001/mf_003.png",
        "FileSize": 307200,
        "MimeType": "image/png",
        "UploadStatus": "Completed",
        "FailReason": null,
        "Progress": 1,
        "TotalChunks": null,
        "UploadedChunks": null,
        "CreatedAt": "2026-07-26 10:01:00"
      }
    ]
  }
}
```

##### 后端行为说明

1. 校验素材库是否存在且类型为 Image
2. 校验上传限制（单文件大小、文件数量、请求总体积）
3. 逐个遍历上传文件，用 `io.Copy` 从上传流直接写入磁盘，不在内存中缓存全部文件
4. 每个文件处理完立即释放，内存占用约等于单个文件大小
5. 为每个文件创建 MaterialFile 记录，UploadStatus 设为 `Completed`
6. 更新素材库的 FileCount 和 TotalSize
7. 前端分批调用：如用户选了 100 张图片，前端自动分 5 批（每批 20 张）调用

---

#### 3.2.6 上传视频（分片）

视频文件采用分片上传，分三步：初始化 → 上传分片 → 合并完成。

##### 3.2.6.1 初始化视频上传

- **URL**：`POST /api/material-libraries/:id/videos/init`
- **Content-Type**：`application/json`

###### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |

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

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | object \| null | 初始化结果 |
| Data.UploadId | string | 上传标识，后续上传分片和完成时使用 |
| Data.ChunkCount | int32 | 分片总数 |

###### Response Example

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

###### 后端行为说明

1. 校验素材库是否存在且类型为 Video
2. 校验素材库下是否已存在同名且 UploadStatus 为 Completed 或 Merging 的文件，若存在则拒绝（返回 ErrorCode=40001）
3. 检查素材库下是否已存在同名且 UploadStatus 为 Uploading 的文件（断点续传）：
   - 若存在，返回已有的 UploadId 和 ChunkCount，前端可继续上传未完成的分片
   - 若不存在，创建新的 MaterialFile 记录，UploadStatus 设为 `Uploading`，TotalChunks 设为计算出的分片数，UploadedChunks 设为 0，创建分片临时目录
4. 返回 UploadId 和 ChunkCount

---

##### 3.2.6.2 上传分片

- **URL**：`POST /api/material-libraries/:id/videos/chunk`
- **Content-Type**：`multipart/form-data`

###### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |

###### Request

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| UploadId | string | 是 | 上传标识 |
| ChunkIndex | int32 | 是 | 分片序号（从 0 开始） |
| file | file | 是 | 分片数据 |

###### Request Example

```bash
curl -X POST http://localhost:8080/api/material-libraries/ml_002/videos/chunk \
  -F "UploadId=upload_abc123" \
  -F "ChunkIndex=0" \
  -F "file=@cam_01.part.0"
```

###### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

###### 后端行为说明

1. 校验 UploadId 是否有效
2. 将分片数据写入临时目录
3. 更新 MaterialFile 的 UploadedChunks

---

##### 3.2.6.3 完成上传（合并分片）

- **URL**：`POST /api/material-libraries/:id/videos/complete`
- **Content-Type**：`application/json`

###### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |

###### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| UploadId | string | 是 | 上传标识 |

###### Request Example

```json
{
  "UploadId": "upload_abc123"
}
```

###### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回，合并异步执行 |

###### Response Example

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

###### 后端行为说明

1. 校验所有分片是否已上传完毕
2. 将 MaterialFile 的 UploadStatus 设为 `Merging`，更新 UploadedChunks 等于 TotalChunks
3. 立即返回响应，不等待合并完成
4. 后端异步执行合并：按序合并分片为完整视频文件
5. 合并成功：
   - 删除分片临时文件
   - 更新 MaterialFile 的 UploadStatus 为 `Completed`，清除 TotalChunks 和 UploadedChunks
   - 更新素材库的 FileCount 和 TotalSize
6. 合并失败：
   - 更新 MaterialFile 的 UploadStatus 为 `Failed`，FailReason 记录失败原因
7. 前端通过查询素材库文件列表获取各文件的上传状态和分片进度

---

#### 3.2.7 获取素材库文件列表

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
| UploadStatus | string | 否 | 按上传状态筛选，见 [UploadStatus](#uploadstatus--文件上传状态) |

##### 排序规则

默认按创建时间升序排列（最早上传的在前）。

##### Request Example

```
GET /api/material-libraries/ml_001/files?Page=1&PageSize=24
```

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | object \| null | 分页数据；素材库不存在时为 `null` |
| Data.Total | int32 | 总记录数 |
| Data.Page | int32 | 当前页码 |
| Data.PageSize | int32 | 每页数量 |
| Data.Items | array | 文件列表，每项结构见 [MaterialFile](#23-materialfile素材文件) |

##### Response Example

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "Total": 3,
    "Page": 1,
    "PageSize": 24,
    "Items": [
      {
        "Id": "mf_001",
        "LibraryId": "ml_001",
        "FileName": "img_001.jpg",
        "StoragePath": "images/ml_001/mf_001.jpg",
        "AccessUrl": "/uploads/images/ml_001/mf_001.jpg",
        "FileSize": 204800,
        "MimeType": "image/jpeg",
        "UploadStatus": "Completed",
        "FailReason": null,
        "Progress": 1,
        "TotalChunks": null,
        "UploadedChunks": null,
        "CreatedAt": "2026-07-26 10:01:00"
      },
      {
        "Id": "mf_002",
        "LibraryId": "ml_001",
        "FileName": "img_002.jpg",
        "StoragePath": "images/ml_001/mf_002.jpg",
        "AccessUrl": "/uploads/images/ml_001/mf_002.jpg",
        "FileSize": 153600,
        "MimeType": "image/jpeg",
        "UploadStatus": "Completed",
        "FailReason": null,
        "Progress": 1,
        "TotalChunks": null,
        "UploadedChunks": null,
        "CreatedAt": "2026-07-26 10:01:00"
      },
      {
        "Id": "mf_010",
        "LibraryId": "ml_002",
        "FileName": "cam_01.mp4",
        "StoragePath": "videos/ml_002/mf_010.mp4",
        "AccessUrl": "/uploads/videos/ml_002/mf_010.mp4",
        "FileSize": 52428800,
        "MimeType": "video/mp4",
        "UploadStatus": "Merging",
        "FailReason": null,
        "Progress": 1,
        "TotalChunks": 10,
        "UploadedChunks": 10,
        "CreatedAt": "2026-07-26 10:10:00"
      }
    ]
  }
}
```

---

#### 3.2.8 删除素材文件

- **URL**：`DELETE /api/material-libraries/:id/files/:fileId`

##### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 素材库 ID |
| fileId | string | 文件 ID |

##### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

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
- 若素材库已关联到任务，则不允许删除文件（返回 ErrorCode=40008）

---

### 3.3 创建任务

创建分析任务并进入后端排队队列，等待调度执行。创建任务时关联一个素材库，素材库类型需与任务类型一致。

- **URL**：`POST /api/tasks`
- **Content-Type**：`application/json`

#### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Name | string | 是 | 任务名称 |
| Type | string | 是 | 任务类型，见 [TaskType](#tasktype--任务类型) |
| ModelConfigId | string | 是 | 模型配置 ID，见 [ModelConfig](#21-modelconfig模型配置) |
| MaterialLibraryId | string | 是 | 关联素材库 ID，见 [MaterialLibrary](#22-materiallibrary素材库) |
| Prompt | string | 是 | 下发给大模型的提示词 |
| Target | string | 是 | 检测目标名称 |
| FrameInterval | int32 | Type=Video 时必填 | 抽帧间隔（秒） |

#### Request Example — 图片集任务

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

#### Request Example — 视频集任务

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

#### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码，`0` 表示创建成功 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

#### 后端行为说明

1. 校验参数（ModelConfigId 是否存在、MaterialLibraryId 是否存在、素材库类型是否与任务类型一致、必填项是否完整）
2. 校验素材库中是否存在 UploadStatus 不为 Completed 的文件，若存在则拒绝创建（返回 ErrorCode=40011）
3. 创建任务记录，关联指定的模型配置和素材库，状态设为 `Pending`（等待中），进入后端任务排队队列
4. **Type=Image**：为素材库下所有图片创建 Image 记录，状态设为 `Pending`（待检测）
5. **Type=Video**：不立即创建 Image 记录，待任务开始执行后再按 FrameInterval 异步抽帧生成帧 Image 记录
6. 后端按排队顺序依次执行任务，任务开始执行时状态变为 `Analyzing`（检测中），使用关联的模型配置调用大模型
7. 立即返回，不等待任务开始执行

---

### 3.4 获取任务列表

查询任务列表，支持按 ID 查询单条记录或按状态分页筛选。前端通过轮询此接口获取检测进度。

- **URL**：`GET /api/tasks`

#### Query Parameters

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Id | string | 否 | 任务 ID，传此参数时按 ID 精确查询，忽略分页参数，返回单条结果 |
| Page | int32 | 否 | 页码，默认 1（Id 参数存在时忽略） |
| PageSize | int32 | 否 | 每页数量，默认 20（Id 参数存在时忽略） |
| Status | string | 否 | 任务状态筛选，见 [TaskStatus](#taskstatus--任务状态) |

#### Request Example — 按状态分页查询

```
GET /api/tasks?Page=1&PageSize=20&Status=Completed
```

#### Request Example — 按 ID 查询单条

```
GET /api/tasks?Id=task_20260726_001
```

#### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | object \| null | 分页数据（Id 查询时 Total=1，Items 含单条）；任务不存在时为 `null` |
| Data.Total | int32 | 总记录数 |
| Data.Page | int32 | 当前页码 |
| Data.PageSize | int32 | 每页数量 |
| Data.Items | array | 任务列表，每项结构见 [TaskItem](#taskitem) |

##### TaskItem

| 字段 | 类型 | 说明 |
|------|------|------|
| Id | string | 任务唯一标识 |
| Name | string | 任务名称 |
| Type | string | 任务类型，见 [TaskType](#tasktype--任务类型) |
| Status | string | 任务状态，见 [TaskStatus](#taskstatus--任务状态) |
| ModelConfigId | string | 使用的模型配置 ID |
| ModelConfigName | string | 使用的模型配置名称 |
| MaterialLibraryId | string | 关联素材库 ID |
| MaterialLibraryName | string | 关联素材库名称 |
| Prompt | string | 下发给大模型的提示词 |
| Target | string | 检测目标名称 |
| FrameInterval | int32 \| null | 抽帧间隔，单位秒（Type=Video 时有值） |
| FailReason | string \| null | 失败原因（Status=Failed 时有值） |
| Progress | object | 检测进度与统计，见 [Progress](#25-progress检测进度与统计) |
| CreatedAt | string | 创建时间 |

#### Response Example — 分页查询

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "Total": 15,
    "Page": 1,
    "PageSize": 20,
    "Items": [
      {
        "Id": "task_20260726_001",
        "Name": "沿街摆摊检测-批次1",
        "Type": "Image",
        "Status": "Completed",
        "ModelConfigId": "mc_001",
        "ModelConfigName": "GPT-4o 视觉检测",
        "MaterialLibraryId": "ml_001",
        "MaterialLibraryName": "沿街摆摊图片集",
        "Prompt": "请检测图片中是否有沿街摆摊行为，返回JSON格式...",
        "Target": "沿街摆摊",
        "FrameInterval": null,
        "FailReason": null,
        "Progress": {
          "Total": 48,
          "Completed": 48,
          "CompletedDetail": {
            "Detected": 32,
            "DetectedDetail": {
              "TruePositive": 29,
              "FalsePositive": 3
            },
            "NotDetected": 15,
            "Failed": 1
          },
          "Pending": 0
        },
        "CreatedAt": "2026-07-26 10:30:00"
      }
    ]
  }
}
```

#### 错误响应示例（按 ID 查询，任务不存在）

```json
{
  "ErrorCode": 40002,
  "ErrorMsg": "任务不存在",
  "Data": null
}
```

---

### 3.5 删除任务

删除任务及其关联的所有素材记录和检测结果（级联硬删除）。

- **URL**：`DELETE /api/tasks/:id`

#### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 任务 ID |

#### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

---

### 3.6 暂停/恢复任务

暂停排队中或检测中的任务，或恢复已暂停的任务。

- **URL**：`PUT /api/tasks/:id`
- **Content-Type**：`application/json`

#### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 任务 ID |

#### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Status | string | 是 | `"Paused"` 暂停任务；`"Analyzing"` 恢复任务 |

#### Request Example — 暂停任务

```json
{
  "Status": "Paused"
}
```

#### Request Example — 恢复任务

```json
{
  "Status": "Analyzing"
}
```

#### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

#### 后端行为说明

- **Status = "Paused"**：暂停任务，保留已完成进度，任务状态变为 `Paused`（暂停中）
  - 当前状态为 `Pending`（等待中）时可暂停，暂停后不再参与调度排队
  - 当前状态为 `Analyzing`（检测中）时可暂停，暂停后停止检测队列
- **Status = "Analyzing"**：恢复任务，任务状态变为 `Analyzing`（检测中），继续从中断处执行检测
  - 当前状态为 `Paused`（暂停中）时可恢复
  - 当前状态为 `Failed`（已失败）时可恢复，恢复后清除 FailReason 并重新入队执行

---

### 3.7 获取任务分析结果

查询任务下所有素材的检测结果。图片集任务和视频集任务的结果结构一致：图片集的素材来自关联素材库中已上传的图片，视频集的素材由后端根据抽帧间隔自动抽帧生成，最终都是逐张图片调大模型检测，结果统一呈现。支持按素材 ID 查询单条或按检测状态分页筛选。

- **URL**：`GET /api/tasks/:id/images`

#### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 任务 ID |

#### Query Parameters

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ImageId | string | 否 | 素材 ID，传此参数时按 ID 精确查询，忽略分页参数，返回单条结果 |
| Page | int32 | 否 | 页码，默认 1（ImageId 参数存在时忽略） |
| PageSize | int32 | 否 | 每页数量，默认 24（ImageId 参数存在时忽略） |
| Status | string | 否 | 检测状态筛选，见 [ImageStatus](#imagestatus--素材检测状态) |
| Correction | string | 否 | 矫正标记筛选：默认返回全部可见素材；传 `"FalsePositive"` 只看误报素材；传 `"DeletedFp"` 只看已删除误报素材 |

#### Request Example — 按状态分页查询

```
GET /api/tasks/task_20260726_001/images?Page=1&PageSize=24&Status=Detected
```

#### Request Example — 按素材 ID 查询单条

```
GET /api/tasks/task_20260726_001/images?ImageId=img_001
```

#### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | object \| null | 分页数据（ImageId 查询时 Total=1，Items 含单条）；素材不存在时为 `null` |
| Data.Total | int32 | 总记录数 |
| Data.Page | int32 | 当前页码 |
| Data.PageSize | int32 | 每页数量 |
| Data.Items | array | 分析结果列表，每项结构见 [Image](#24-image检测素材) |

#### Response Example — 分页查询

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": {
    "Total": 48,
    "Page": 1,
    "PageSize": 24,
    "Items": [
      {
        "Id": "img_001",
        "TaskId": "task_20260726_001",
        "AccessUrl": "/uploads/images/ml_001/mf_001.jpg",
        "MaterialFileId": "mf_001",
        "FrameIndex": null,
        "Status": "Detected",
        "Detection": {
          "HasTarget": true,
          "Boxes": [
            {
              "X1": 120,
              "Y1": 85,
              "X2": 320,
              "Y2": 245,
              "Confidence": "high",
              "Label": "沿街摆摊"
            }
          ],
          "RawResponse": "{ \"detected_flag\": true, \"detections\": [{\"bbox_2d\": [120, 85, 320, 245], \"category\": \"沿街摆摊\", \"confidence_note\": \"high\"}] }",
          "AnalyzedAt": "2026-07-26 10:30:15"
        },
        "FailReason": null,
        "Correction": null
      },
      {
        "Id": "img_005",
        "TaskId": "task_20260726_001",
        "AccessUrl": "/uploads/images/ml_001/mf_005.jpg",
        "MaterialFileId": "mf_005",
        "FrameIndex": null,
        "Status": "Detected",
        "Detection": {
          "HasTarget": true,
          "Boxes": [
            {
              "X1": 80,
              "Y1": 60,
              "X2": 200,
              "Y2": 160,
              "Confidence": "medium",
              "Label": "沿街摆摊"
            }
          ],
          "RawResponse": "{ \"detected_flag\": true, \"detections\": [{\"bbox_2d\": [80, 60, 200, 160], \"category\": \"沿街摆摊\", \"confidence_note\": \"medium\"}] }",
          "AnalyzedAt": "2026-07-26 10:30:25"
        },
        "FailReason": null,
        "Correction": "FalsePositive"
      }
    ]
  }
}
```

---

### 3.8 补录漏报照片

将一张新照片上传并添加到任务的素材集中，后续可通过任务详情查看。

- **URL**：`POST /api/tasks/:id/images`
- **Content-Type**：`multipart/form-data`

#### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 任务 ID |

#### Request

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 图片文件 |

#### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

#### 后端行为说明

1. 上传图片到任务帧目录（`data/tasks/{task_id}/frames/`）
2. 创建 Image 记录并加入任务的素材集，后端内部标记来源为补录
3. 素材列表查询时自动体现（后端内部过滤已删除误报的素材）

---

### 3.9 标记素材误报

将素材标记为误报，素材仍可见但标注为误报状态，配合误报删除和恢复接口可动态调整任务下的误报率。

- **URL**：`PUT /api/tasks/:id/images/:imageId`
- **Content-Type**：`application/json`

#### Path Parameters

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 任务 ID |
| imageId | string | 素材 ID |

#### Request Body

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Correction | string \| null | 是 | `"FalsePositive"` 标记误报；`"DeletedFp"` 删除误报；`null` 恢复正常 |

#### Request Example — 标记误报

```json
{
  "Correction": "FalsePositive"
}
```

#### Request Example — 删除误报

```json
{
  "Correction": "DeletedFp"
}
```

#### Request Example — 恢复正常

```json
{
  "Correction": null
}
```

#### Response

| 字段 | 类型 | 说明 |
|------|------|------|
| ErrorCode | int32 | 错误码 |
| ErrorMsg | string | 错误描述 |
| Data | null | 无业务数据返回 |

```json
{
  "ErrorCode": 0,
  "ErrorMsg": "",
  "Data": null
}
```

#### 后端行为说明

- **Correction = "FalsePositive"**：标记该素材为误报，素材仍在分析结果列表中可见，但标注为误报状态，Progress 统计自动调整（误报率变化）
- **Correction = "DeletedFp"**：删除误报，后端内部软删除该素材，分析结果列表默认不返回此类素材，Progress 统计自动调整
- **Correction = null**：恢复正常，清除所有矫正标记，素材恢复为正常检测结果，Progress 统计自动调整

---

## 4. 接口总览

| 序号 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 1 | POST | `/api/model-configs` | 创建模型配置 |
| 2 | GET | `/api/model-configs` | 获取模型配置列表（支持按 ID 查询单条） |
| 3 | PUT | `/api/model-configs/:id` | 更新模型配置 |
| 4 | DELETE | `/api/model-configs/:id` | 删除模型配置 |
| 5 | POST | `/api/model-configs/:id/test` | 测试模型连通性 |
| 6 | POST | `/api/material-libraries` | 创建素材库 |
| 7 | GET | `/api/material-libraries` | 获取素材库列表（支持按 ID 查询单条） |
| 8 | PUT | `/api/material-libraries/:id` | 更新素材库 |
| 9 | DELETE | `/api/material-libraries/:id` | 删除素材库 |
| 10 | POST | `/api/material-libraries/:id/images` | 批量上传图片 |
| 11 | POST | `/api/material-libraries/:id/videos/init` | 初始化视频上传 |
| 12 | POST | `/api/material-libraries/:id/videos/chunk` | 上传视频分片 |
| 13 | POST | `/api/material-libraries/:id/videos/complete` | 完成视频上传 |
| 14 | GET | `/api/material-libraries/:id/files` | 获取素材文件列表 |
| 15 | DELETE | `/api/material-libraries/:id/files/:fileId` | 删除素材文件 |
| 16 | POST | `/api/tasks` | 创建任务（排队等待执行） |
| 17 | GET | `/api/tasks` | 获取任务列表（支持按 ID 查询单条） |
| 18 | DELETE | `/api/tasks/:id` | 删除任务（级联硬删除） |
| 19 | PUT | `/api/tasks/:id` | 暂停/恢复任务 |
| 20 | GET | `/api/tasks/:id/images` | 获取任务分析结果（支持按素材 ID 查询单条） |
| 21 | POST | `/api/tasks/:id/images` | 补录漏报照片 |
| 22 | PUT | `/api/tasks/:id/images/:imageId` | 标记素材误报/删除误报/恢复 |

---

## 5. 状态流转

### 5.1 任务状态

```
  POST /api/tasks
        │
        ▼
   ┌──────────┐
   │ Pending  │ ◄── 等待中（排队等待执行）
   └──┬───┬───┘
      │   │
      │   PUT { Status: "Paused" }
      │        │
      │        ▼
      │   ┌──────────┐
      │   │  Paused  │ ◄── 暂停中
      │   │  暂停中   │
      │   └────┬─────┘
      │        │
      │   PUT { Status: "Analyzing" }  (暂停→可恢复)
      │        │
      │        ▼
   后端调度执行 ─────────────────────────────────┐
      │                                         │
      ▼                                         │
   ┌──────────┐   PUT { Status: "Paused" }    │
   │ Analyzing │ ──────────────────────────▶ ┌──────────┐
   │  检测中   │ ◄────────────────────────── │  Paused  │
   └──┬───┬───┘   PUT { Status: "Analyzing" }  暂停中   │
      │   │                                    └────┬─────┘
      │   │  执行失败（抽帧失败/无素材/模型配置缺失）     │
      │   │        │                           恢复后
      │   │        ▼                           继续检测
      │   │   ┌──────────┐                        │
      │   │   │  Failed  │ ◄── 已失败              │
      │   │   │  已失败   │   FailReason 有失败原因   │
      │   │   └────┬─────┘                        │
      │   │        │                              │
      │   │   PUT { Status: "Analyzing" }         │
      │   │   (Failed→可恢复，清除FailReason)       │
      │   │        │                              │
   全部素材检测完毕  └────────────────────────────────┘
      │
      ▼
  ┌───────────┐
  │ Completed │
  │  已完成    │
  └───────────┘
```

### 5.2 素材状态

```
  创建任务 / 补录漏报（上传图片）
        │
        ▼
  ┌─────────┐         调用失败           ┌──────────┐
  │ Pending  │ ─────────────────────────▶ │  Failed  │
  │ 待检测   │                            │ 检测失败  │
  └────┬────┘                            └──────────┘
       │                                  FailReason
  大模型返回结果                            有失败原因
       │
  ┌────┴─────────┐
  │              │
  ▼              ▼
Detected      NotDetected
 已检出         未检出
```

---

## 6. 数据流总览

```
┌──────────────────────────────────────────────────────────────────┐
│                         数据流                                    │
│                                                                  │
│  ── 模型配置 ──                                                   │
│                                                                  │
│  POST   /api/model-configs  → 创建模型配置                       │
│  GET    /api/model-configs  → 查询配置列表                       │
│  PUT    /api/model-configs/:id  → 更新配置                       │
│  DELETE /api/model-configs/:id  → 删除配置                       │
│  POST   /api/model-configs/:id/test  → 测试连通性                │
│                                                                  │
│  ── 素材库管理 ──                                                 │
│                                                                  │
│  POST /api/material-libraries  → 创建素材库                      │
│  GET  /api/material-libraries  → 查询素材库列表                  │
│  PUT  /api/material-libraries/:id  → 更新素材库                  │
│  DELETE /api/material-libraries/:id  → 删除素材库                │
│                                                                  │
│  POST /api/material-libraries/:id/images  → 批量上传图片         │
│  POST /api/material-libraries/:id/videos/init  → 初始化视频上传  │
│  POST /api/material-libraries/:id/videos/chunk  → 上传视频分片   │
│  POST /api/material-libraries/:id/videos/complete  → 完成上传    │
│  GET  /api/material-libraries/:id/files  → 查询文件列表          │
│  DELETE /api/material-libraries/:id/files/:fileId  → 删除文件    │
│                                                                  │
│  ── 任务管理 ──                                                   │
│                                                                  │
│  POST /api/tasks                                                 │
│  { Type, ModelConfigId, MaterialLibraryId, Prompt, Target,       │
│    FrameInterval(Video) }                                        │
│         │                                                        │
│         ▼                                                        │
│  ┌─────────────┐                                                │
│  │  Task       │  任务创建，Status = Pending（等待中）            │
│  │  Pending    │  进入后端排队队列                                │
│  └──────┬──────┘                                                │
│         │                                                        │
│    后端调度执行（使用 ModelConfig 关联的模型配置调用大模型）         │
│         │                                                        │
│         ▼                                                        │
│  ┌─────────────┐    异步抽帧(Type=Video)    ┌─────────────┐      │
│  │  Task       │───────────────────────────▶│  Image x N  │      │
│  │  Analyzing  │                            │  Pending     │      │
│  └──────┬──────┘                            └──────┬──────┘      │
│         │                                          │             │
│  PUT { Status: "Paused" }                   异步调用大模型      │
│         │                                          │             │
│         ▼                               ┌────────────┼──────┐    │
│  ┌─────────────┐                         ▼            ▼      ▼    │
│  │  Task       │                    ┌─────────┐ ┌──────────┐ ┌─┐ │
│  │  Paused     │                    │Detected │ │NotDetect │ │F│ │
│  │  暂停中     │                    │ 已检出   │ │  未检出   │ │失│ │
│  └──────┬──────┘                    └─────────┘ └──────────┘ │败│ │
│         │                                          └─┬──────┘ │ │
│  PUT { Status: "Analyzing" }                      FailReason  │ │
│         │                                                        │
│         ▼                                                        │
│    恢复检测，继续从中断处执行                                       │
│                                                                  │
│  GET /api/tasks?Id=xxx  ◀── 前端轮询获取任务详情+Progress          │
│  GET /api/tasks/:id/images  ◀── 前端获取任务分析结果               │
│  GET /api/tasks/:id/images?ImageId=xxx  ◀── 查询单条分析结果       │
│                                                                  │
│  ── 矫正操作 ──                                                   │
│                                                                  │
│  PUT /tasks/:id/images/:id { Correction: "FalsePositive" }         │
│  → 标记误报，素材仍可见但标注为误报，Progress 自动调整（误报率变化）│
│                                                                  │
│  PUT /tasks/:id/images/:id { Correction: "DeletedFp" }         │
│  → 删除误报，素材列表自动过滤，Progress 自动调整                  │
│                                                                  │
│  PUT /tasks/:id/images/:id { Correction: null }                │
│  → 恢复正常，清除矫正标记，Progress 自动调整                       │
│                                                                  │
│  POST /tasks/:id/images (multipart/form-data, 上传图片)           │
│  → 后端将照片加入任务素材集，列表自动体现                           │
└──────────────────────────────────────────────────────────────────┘
```
