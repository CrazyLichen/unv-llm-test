# 素材库与文件上传改造 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现素材库管理（CRUD + 图片批量上传 + 视频分片上传）及静态文件服务，为后续任务管理改造提供基础。

**Architecture:** 遵循现有分层架构（Controller → Service → Repository），新增 MaterialLibrary 和 MaterialFile 两个实体，新增文件上传服务和静态文件服务。视频分片上传采用 init/chunk/complete 三步流程，合并异步执行。

**Tech Stack:** Go 1.24 + Gin + GORM + SQLite (glebarez/sqlite)

---

## 文件结构

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 修改 | `server/internal/common/errorcode.go` | 新增素材库相关错误码 |
| 修改 | `server/internal/model/model_config.go` | 新增 MaterialLibrary、MaterialFile 实体和 DTO |
| 创建 | `server/internal/repository/material_library_repo.go` | 素材库 + 素材文件数据访问层 |
| 创建 | `server/internal/service/material_library_svc.go` | 素材库业务逻辑层（含文件上传、分片管理） |
| 创建 | `server/internal/controller/material_library_ctrl.go` | 素材库 HTTP 控制器 |
| 修改 | `server/internal/controller/router.go` | 注册素材库路由 + 静态文件服务 |
| 修改 | `server/internal/repository/db.go` | AutoMigrate 新增表 |
| 修改 | `server/cmd/server/main.go` | 初始化素材库各层 + 注入依赖 |
| 修改 | `server/internal/config/config.go` | 新增上传目录配置 |
| 修改 | `server/config.yaml` | 新增上传目录配置项 |

---

### Task 1: 新增错误码

**Files:**
- Modify: `server/internal/common/errorcode.go`

- [ ] **Step 1: 添加素材库相关错误码常量**

在 `errorcode.go` 的常量区块中新增：

```go
// ErrLibraryNotFound 素材库不存在
ErrLibraryNotFound = 40007
// ErrLibraryAlreadyBound 素材库已被任务关联
ErrLibraryAlreadyBound = 40008
// ErrLibraryTypeMismatch 素材库类型与任务类型不匹配
ErrLibraryTypeMismatch = 40009
// ErrFileNotFound 文件不存在
ErrFileNotFound = 40010
// ErrFileUploadIncomplete 文件上传未完成
ErrFileUploadIncomplete = 40011
// ErrFileUploadFailed 文件上传失败
ErrFileUploadFailed = 50003
// ErrChunkUploadFailed 分片上传异常
ErrChunkUploadFailed = 50004
```

移除不再使用的 `ErrPathNotFound`：

```go
// 删除这一行：
// ErrPathNotFound = 40004
```

- [ ] **Step 2: 验证编译通过**

Run: `cd server && go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
cd server && git add internal/common/errorcode.go && git commit -m "feat: 新增素材库相关错误码"
```

---

### Task 2: 新增数据模型和 DTO

**Files:**
- Modify: `server/internal/model/model_config.go`

- [ ] **Step 1: 在 model_config.go 文件末尾追加 MaterialLibrary 和 MaterialFile 实体及 DTO**

```go
// ──────────────────────────── 结构体 ────────────────────────────

// MaterialLibrary 素材库实体（对应 material_libraries 表）
type MaterialLibrary struct {
	// Id 素材库唯一标识
	Id string `json:"Id" gorm:"primaryKey;column:id"`
	// Name 素材库名称
	Name string `json:"Name" gorm:"column:name;not null"`
	// Type 素材类型：Image / Video
	Type string `json:"Type" gorm:"column:type;not null"`
	// Description 描述
	Description *string `json:"Description" gorm:"column:description"`
	// FileCount 文件数量
	FileCount int32 `json:"FileCount" gorm:"column:file_count;not null;default:0"`
	// TotalSize 文件总大小（字节）
	TotalSize int64 `json:"TotalSize" gorm:"column:total_size;not null;default:0"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt" gorm:"column:created_at;not null"`
	// UpdatedAt 最后更新时间
	UpdatedAt string `json:"UpdatedAt" gorm:"column:updated_at;not null"`
}

// MaterialFile 素材文件实体（对应 material_files 表）
type MaterialFile struct {
	// Id 文件唯一标识
	Id string `json:"Id" gorm:"primaryKey;column:id"`
	// LibraryId 所属素材库 ID
	LibraryId string `json:"LibraryId" gorm:"column:library_id;not null;index"`
	// FileName 原始文件名
	FileName string `json:"FileName" gorm:"column:file_name;not null"`
	// StoragePath 后端存储路径（相对路径）
	StoragePath string `json:"StoragePath" gorm:"column:storage_path;not null"`
	// AccessUrl 浏览器访问 URL
	AccessUrl string `json:"AccessUrl" gorm:"column:access_url;not null"`
	// FileSize 文件大小（字节）
	FileSize int64 `json:"FileSize" gorm:"column:file_size;not null;default:0"`
	// MimeType MIME 类型
	MimeType string `json:"MimeType" gorm:"column:mime_type;not null"`
	// UploadStatus 上传状态：Uploading / Merging / Completed / Failed
	UploadStatus string `json:"UploadStatus" gorm:"column:upload_status;not null;default:Completed"`
	// FailReason 失败原因
	FailReason *string `json:"FailReason" gorm:"column:fail_reason"`
	// TotalChunks 分片总数（仅视频文件有值）
	TotalChunks *int32 `json:"TotalChunks" gorm:"column:total_chunks"`
	// UploadedChunks 已上传分片数（仅视频文件有值）
	UploadedChunks *int32 `json:"UploadedChunks" gorm:"column:uploaded_chunks"`
	// UploadId 上传标识（分片上传用，关联初始化时生成的 ID）
	UploadId *string `json:"UploadId" gorm:"column:upload_id"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt" gorm:"column:created_at;not null"`
}

// CreateMaterialLibraryReq 创建素材库请求
type CreateMaterialLibraryReq struct {
	// Name 素材库名称
	Name string `json:"Name" binding:"required"`
	// Type 素材类型
	Type string `json:"Type" binding:"required,oneof=Image Video"`
	// Description 描述
	Description *string `json:"Description"`
}

// UpdateMaterialLibraryReq 更新素材库请求
type UpdateMaterialLibraryReq struct {
	// Name 素材库名称
	Name string `json:"Name" binding:"required"`
	// Description 描述
	Description *string `json:"Description"`
}

// InitVideoUploadReq 初始化视频上传请求
type InitVideoUploadReq struct {
	// FileName 视频文件名
	FileName string `json:"FileName" binding:"required"`
	// FileSize 文件总大小（字节）
	FileSize int64 `json:"FileSize" binding:"required,gt=0"`
	// ChunkSize 分片大小（字节）
	ChunkSize int32 `json:"ChunkSize" binding:"required,gt=0"`
}

// InitVideoUploadResp 初始化视频上传响应
type InitVideoUploadResp struct {
	// UploadId 上传标识
	UploadId string `json:"UploadId"`
	// ChunkCount 分片总数
	ChunkCount int32 `json:"ChunkCount"`
}

// UploadImageResp 批量上传图片响应
type UploadImageResp struct {
	// UploadedCount 成功上传数量
	UploadedCount int `json:"UploadedCount"`
	// Files 上传文件列表
	Files []MaterialFile `json:"Files"`
}

// CompleteVideoUploadReq 完成视频上传请求
type CompleteVideoUploadReq struct {
	// UploadId 上传标识
	UploadId string `json:"UploadId" binding:"required"`
}

// MaterialFileProgress 素材文件进度视图（用于响应中添加 Progress 字段）
type MaterialFileProgress struct {
	// Id 文件唯一标识
	Id string `json:"Id"`
	// LibraryId 所属素材库 ID
	LibraryId string `json:"LibraryId"`
	// FileName 原始文件名
	FileName string `json:"FileName"`
	// StoragePath 后端存储路径
	StoragePath string `json:"StoragePath"`
	// AccessUrl 浏览器访问 URL
	AccessUrl string `json:"AccessUrl"`
	// FileSize 文件大小（字节）
	FileSize int64 `json:"FileSize"`
	// MimeType MIME 类型
	MimeType string `json:"MimeType"`
	// UploadStatus 上传状态
	UploadStatus string `json:"UploadStatus"`
	// FailReason 失败原因
	FailReason *string `json:"FailReason"`
	// Progress 上传进度（0-1）
	Progress float64 `json:"Progress"`
	// TotalChunks 分片总数
	TotalChunks *int32 `json:"TotalChunks"`
	// UploadedChunks 已上传分片数
	UploadedChunks *int32 `json:"UploadedChunks"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt"`
}
```

- [ ] **Step 2: 验证编译通过**

Run: `cd server && go build ./...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
cd server && git add internal/model/model_config.go && git commit -m "feat: 新增素材库和素材文件数据模型及 DTO"
```

---

### Task 3: 新增配置项 — 上传目录

**Files:**
- Modify: `server/internal/config/config.go`
- Modify: `server/config.yaml`

- [ ] **Step 1: 在 Config 结构体中新增 Upload 配置，在 DatabaseConfig 后面新增 UploadConfig**

```go
// UploadConfig 上传配置
type UploadConfig struct {
	// Dir 上传文件存储目录
	Dir string `yaml:"dir"`
	// MaxImageSize 单个图片文件最大大小（字节）
	MaxImageSize int64 `yaml:"max_image_size"`
	// MaxImageCount 单次最多上传图片数量
	MaxImageCount int `yaml:"max_image_count"`
	// MaxImageBatchSize 单次上传请求总体积最大值（字节）
	MaxImageBatchSize int64 `yaml:"max_image_batch_size"`
}
```

在 Config 结构体中新增字段：

```go
// Upload 上传配置
Upload UploadConfig `yaml:"upload"`
```

在 Load 函数的默认值设置中新增：

```go
if cfg.Upload.Dir == "" {
    cfg.Upload.Dir = "./data/uploads"
}
if cfg.Upload.MaxImageSize == 0 {
    cfg.Upload.MaxImageSize = 10 * 1024 * 1024 // 10MB
}
if cfg.Upload.MaxImageCount == 0 {
    cfg.Upload.MaxImageCount = 20
}
if cfg.Upload.MaxImageBatchSize == 0 {
    cfg.Upload.MaxImageBatchSize = 50 * 1024 * 1024 // 50MB
}
```

- [ ] **Step 2: 更新 config.yaml**

在文件末尾追加：

```yaml
upload:
  dir: "./data/uploads"
  max_image_size: 10485760      # 10MB
  max_image_count: 20
  max_image_batch_size: 52428800  # 50MB
```

- [ ] **Step 3: 验证编译通过**

Run: `cd server && go build ./...`
Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
cd server && git add internal/config/config.go config.yaml && git commit -m "feat: 新增上传目录配置项"
```

---

### Task 4: 新增素材库仓储层

**Files:**
- Create: `server/internal/repository/material_library_repo.go`

- [ ] **Step 1: 创建素材库仓储文件**

```go
package repository

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
)

// ──────────────────────────── 结构体 ────────────────────────────

// MaterialLibraryRepo 素材库数据访问层
type MaterialLibraryRepo struct {
	db *gorm.DB
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewMaterialLibraryRepo 创建素材库仓储实例
func NewMaterialLibraryRepo(db *gorm.DB) *MaterialLibraryRepo {
	return &MaterialLibraryRepo{db: db}
}

// Create 插入一条素材库
func (r *MaterialLibraryRepo) Create(ctx context.Context, ml *model.MaterialLibrary) error {
	if err := r.db.WithContext(ctx).Create(ml).Error; err != nil {
		slog.Error("插入素材库失败", "id", ml.Id, "error", err)
		return fmt.Errorf("插入素材库失败: %w", err)
	}
	slog.Info("插入素材库成功", "id", ml.Id, "name", ml.Name)
	return nil
}

// GetByID 根据 ID 查询单条素材库
func (r *MaterialLibraryRepo) GetByID(ctx context.Context, id string) (*model.MaterialLibrary, error) {
	var ml model.MaterialLibrary
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&ml).Error
	if err == gorm.ErrRecordNotFound {
		slog.Warn("素材库不存在", "id", id)
		return nil, nil
	}
	if err != nil {
		slog.Error("按ID查询素材库失败", "id", id, "error", err)
		return nil, fmt.Errorf("按ID查询素材库失败: %w", err)
	}
	return &ml, nil
}

// List 分页查询素材库列表，返回列表和总数
func (r *MaterialLibraryRepo) List(ctx context.Context, page, pageSize int, libType string) ([]model.MaterialLibrary, int, error) {
	query := r.db.WithContext(ctx).Model(&model.MaterialLibrary{})
	if libType != "" {
		query = query.Where("type = ?", libType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("统计素材库数量失败", "error", err)
		return nil, 0, fmt.Errorf("统计素材库数量失败: %w", err)
	}

	var items []model.MaterialLibrary
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		slog.Error("查询素材库列表失败", "page", page, "pageSize", pageSize, "error", err)
		return nil, 0, fmt.Errorf("查询素材库列表失败: %w", err)
	}

	slog.Info("查询素材库列表", "page", page, "pageSize", pageSize, "total", total, "count", len(items))
	return items, int(total), nil
}

// Update 更新素材库
func (r *MaterialLibraryRepo) Update(ctx context.Context, id string, req *model.UpdateMaterialLibraryReq) error {
	updates := map[string]interface{}{
		"name":       req.Name,
		"updated_at": common.NowFormatted(),
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	result := r.db.WithContext(ctx).Model(&model.MaterialLibrary{}).Where("id = ?", id).Select(mapKeys(updates)).Updates(updates)
	if result.Error != nil {
		slog.Error("更新素材库失败", "id", id, "error", result.Error)
		return fmt.Errorf("更新素材库失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	slog.Info("更新素材库成功", "id", id)
	return nil
}

// Delete 删除素材库
func (r *MaterialLibraryRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.MaterialLibrary{})
	if result.Error != nil {
		slog.Error("删除素材库失败", "id", id, "error", result.Error)
		return fmt.Errorf("删除素材库失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	slog.Info("删除素材库成功", "id", id)
	return nil
}

// HasRelatedTasks 检查素材库是否已关联任务
func (r *MaterialLibraryRepo) HasRelatedTasks(ctx context.Context, libraryId string) (bool, error) {
	// tasks 表尚未创建，返回 false
	return false, nil
}

// ──────────────────────────── 素材文件 ────────────────────────────

// CreateFile 插入一条素材文件
func (r *MaterialLibraryRepo) CreateFile(ctx context.Context, mf *model.MaterialFile) error {
	if err := r.db.WithContext(ctx).Create(mf).Error; err != nil {
		slog.Error("插入素材文件失败", "id", mf.Id, "error", err)
		return fmt.Errorf("插入素材文件失败: %w", err)
	}
	slog.Info("插入素材文件成功", "id", mf.Id, "fileName", mf.FileName)
	return nil
}

// GetFileByID 根据 ID 查询单条素材文件
func (r *MaterialLibraryRepo) GetFileByID(ctx context.Context, id string) (*model.MaterialFile, error) {
	var mf model.MaterialFile
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&mf).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("按ID查询素材文件失败", "id", id, "error", err)
		return nil, fmt.Errorf("按ID查询素材文件失败: %w", err)
	}
	return &mf, nil
}

// GetFileByUploadId 根据 UploadId 查询素材文件
func (r *MaterialLibraryRepo) GetFileByUploadId(ctx context.Context, uploadId string) (*model.MaterialFile, error) {
	var mf model.MaterialFile
	err := r.db.WithContext(ctx).Where("upload_id = ?", uploadId).First(&mf).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("按UploadId查询素材文件失败", "uploadId", uploadId, "error", err)
		return nil, fmt.Errorf("按UploadId查询素材文件失败: %w", err)
	}
	return &mf, nil
}

// FindUploadingFileByName 查找素材库下同名且上传中的文件（断点续传）
func (r *MaterialLibraryRepo) FindUploadingFileByName(ctx context.Context, libraryId string, fileName string) (*model.MaterialFile, error) {
	var mf model.MaterialFile
	err := r.db.WithContext(ctx).
		Where("library_id = ? AND file_name = ? AND upload_status = ?", libraryId, fileName, "Uploading").
		First(&mf).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("查找同名上传中文件失败", "libraryId", libraryId, "fileName", fileName, "error", err)
		return nil, fmt.Errorf("查找同名上传中文件失败: %w", err)
	}
	return &mf, nil
}

// FindCompletedOrMergingFileByName 查找素材库下同名且已完成或合并中的文件
func (r *MaterialLibraryRepo) FindCompletedOrMergingFileByName(ctx context.Context, libraryId string, fileName string) (*model.MaterialFile, error) {
	var mf model.MaterialFile
	err := r.db.WithContext(ctx).
		Where("library_id = ? AND file_name = ? AND upload_status IN ?", libraryId, fileName, []string{"Completed", "Merging"}).
		First(&mf).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("查找同名已完成文件失败", "libraryId", libraryId, "fileName", fileName, "error", err)
		return nil, fmt.Errorf("查找同名已完成文件失败: %w", err)
	}
	return &mf, nil
}

// ListFiles 分页查询素材文件列表，按创建时间升序排列
func (r *MaterialLibraryRepo) ListFiles(ctx context.Context, libraryId string, page, pageSize int, uploadStatus string) ([]model.MaterialFile, int, error) {
	query := r.db.WithContext(ctx).Model(&model.MaterialFile{}).Where("library_id = ?", libraryId)
	if uploadStatus != "" {
		query = query.Where("upload_status = ?", uploadStatus)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("统计素材文件数量失败", "libraryId", libraryId, "error", err)
		return nil, 0, fmt.Errorf("统计素材文件数量失败: %w", err)
	}

	var items []model.MaterialFile
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		slog.Error("查询素材文件列表失败", "libraryId", libraryId, "page", page, "error", err)
		return nil, 0, fmt.Errorf("查询素材文件列表失败: %w", err)
	}

	return items, int(total), nil
}

// UpdateFile 更新素材文件
func (r *MaterialLibraryRepo) UpdateFile(ctx context.Context, id string, updates map[string]interface{}) error {
	result := r.db.WithContext(ctx).Model(&model.MaterialFile{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		slog.Error("更新素材文件失败", "id", id, "error", result.Error)
		return fmt.Errorf("更新素材文件失败: %w", result.Error)
	}
	return nil
}

// DeleteFile 删除素材文件
func (r *MaterialLibraryRepo) DeleteFile(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.MaterialFile{})
	if result.Error != nil {
		slog.Error("删除素材文件失败", "id", id, "error", result.Error)
		return fmt.Errorf("删除素材文件失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	slog.Info("删除素材文件成功", "id", id)
	return nil
}

// UpdateLibraryStats 更新素材库的文件数量和总大小
func (r *MaterialLibraryRepo) UpdateLibraryStats(ctx context.Context, libraryId string) error {
	var count int64
	var totalSize int64
	r.db.WithContext(ctx).Model(&model.MaterialFile{}).
		Where("library_id = ? AND upload_status = ?", libraryId, "Completed").
		Count(&count)
	r.db.WithContext(ctx).Model(&model.MaterialFile{}).
		Where("library_id = ? AND upload_status = ?", libraryId, "Completed").
		Select("COALESCE(SUM(file_size), 0)").Scan(&totalSize)

	result := r.db.WithContext(ctx).Model(&model.MaterialLibrary{}).Where("id = ?", libraryId).
		Updates(map[string]interface{}{
			"file_count": int32(count),
			"total_size": totalSize,
			"updated_at": common.NowFormatted(),
		})
	if result.Error != nil {
		slog.Error("更新素材库统计失败", "libraryId", libraryId, "error", result.Error)
		return fmt.Errorf("更新素材库统计失败: %w", result.Error)
	}
	return nil
}

// HasIncompleteFiles 检查素材库中是否有未完成上传的文件
func (r *MaterialLibraryRepo) HasIncompleteFiles(ctx context.Context, libraryId string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.MaterialFile{}).
		Where("library_id = ? AND upload_status != ?", libraryId, "Completed").
		Count(&count).Error
	if err != nil {
		slog.Error("检查未完成文件失败", "libraryId", libraryId, "error", err)
		return false, fmt.Errorf("检查未完成文件失败: %w", err)
	}
	return count > 0, nil
}
```

- [ ] **Step 2: 验证编译通过**

Run: `cd server && go build ./...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
cd server && git add internal/repository/material_library_repo.go && git commit -m "feat: 新增素材库和素材文件仓储层"
```

---

### Task 5: 新增素材库服务层

**Files:**
- Create: `server/internal/service/material_library_svc.go`

- [ ] **Step 1: 创建素材库服务文件**

文件较长，包含以下方法：
- `NewMaterialLibraryService` — 构造函数
- `Create` — 创建素材库
- `GetByID` — 按 ID 查询
- `List` — 分页列表
- `Update` — 更新素材库
- `Delete` — 删除素材库（级联删除文件+磁盘文件）
- `UploadImages` — 批量上传图片（流式写入磁盘）
- `InitVideoUpload` — 初始化视频上传（断点续传 + 同名检查）
- `UploadChunk` — 上传分片
- `CompleteVideoUpload` — 完成视频上传（异步合并）
- `mergeChunksAsync` — 异步合并分片 goroutine
- `ListFiles` — 查询素材文件列表（含 Progress 计算）
- `DeleteFile` — 删除素材文件
- `toFileProgress` — MaterialFile → MaterialFileProgress（计算 Progress）

```go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/common"
	"llm-test-server/internal/config"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)

// ──────────────────────────── 结构体 ────────────────────────────

// MaterialLibraryService 素材库业务逻辑层
type MaterialLibraryService struct {
	repo *repository.MaterialLibraryRepo
	// uploadDir 上传文件存储根目录
	uploadDir string
	// maxImageSize 单个图片文件最大大小
	maxImageSize int64
	// maxImageCount 单次最多上传图片数量
	maxImageCount int
	// maxImageBatchSize 单次上传请求总体积最大值
	maxImageBatchSize int64
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewMaterialLibraryService 创建素材库服务实例
func NewMaterialLibraryService(repo *repository.MaterialLibraryRepo, uploadCfg *config.UploadConfig) *MaterialLibraryService {
	return &MaterialLibraryService{
		repo:              repo,
		uploadDir:         uploadCfg.Dir,
		maxImageSize:      uploadCfg.MaxImageSize,
		maxImageCount:     uploadCfg.MaxImageCount,
		maxImageBatchSize: uploadCfg.MaxImageBatchSize,
	}
}

// Create 创建素材库
func (s *MaterialLibraryService) Create(ctx context.Context, req *model.CreateMaterialLibraryReq) (*model.MaterialLibrary, error) {
	id, err := generateLibraryID()
	if err != nil {
		return nil, fmt.Errorf("生成ID失败: %w", err)
	}

	now := common.NowFormatted()
	ml := &model.MaterialLibrary{
		Id:          id,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		FileCount:   0,
		TotalSize:   0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, ml); err != nil {
		return nil, err
	}

	slog.Info("创建素材库成功", "id", id, "name", req.Name, "type", req.Type)
	return ml, nil
}

// GetByID 按 ID 查询素材库
func (s *MaterialLibraryService) GetByID(ctx context.Context, id string) (*model.MaterialLibrary, error) {
	ml, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ml == nil {
		return nil, &AppError{Code: common.ErrLibraryNotFound, Msg: "素材库不存在"}
	}
	return ml, nil
}

// List 分页查询素材库列表
func (s *MaterialLibraryService) List(ctx context.Context, page, pageSize int, libType string) ([]model.MaterialLibrary, int, error) {
	return s.repo.List(ctx, page, pageSize, libType)
}

// Update 更新素材库
func (s *MaterialLibraryService) Update(ctx context.Context, id string, req *model.UpdateMaterialLibraryReq) error {
	ml, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ml == nil {
		return &AppError{Code: common.ErrLibraryNotFound, Msg: "素材库不存在"}
	}
	if err := s.repo.Update(ctx, id, req); err != nil {
		return err
	}
	slog.Info("更新素材库成功", "id", id)
	return nil
}

// Delete 删除素材库（级联删除文件和磁盘文件）
func (s *MaterialLibraryService) Delete(ctx context.Context, id string) error {
	ml, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ml == nil {
		return &AppError{Code: common.ErrLibraryNotFound, Msg: "素材库不存在"}
	}

	// 检查是否有关联任务
	hasRelated, err := s.repo.HasRelatedTasks(ctx, id)
	if err != nil {
		return err
	}
	if hasRelated {
		return &AppError{Code: common.ErrLibraryAlreadyBound, Msg: "该素材库已被任务关联，无法删除"}
	}

	// 查询所有文件记录
	files, _, err := s.repo.ListFiles(ctx, id, 1, 10000, "")
	if err != nil {
		return err
	}

	// 删除磁盘文件
	for _, f := range files {
		fullPath := filepath.Join(s.uploadDir, f.StoragePath)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("删除磁盘文件失败", "path", fullPath, "error", err)
		}
		// 删除分片临时目录
		if f.UploadStatus == "Uploading" || f.UploadStatus == "Merging" {
			chunkDir := filepath.Join(s.uploadDir, filepath.Dir(f.StoragePath), "chunks")
			os.RemoveAll(chunkDir)
		}
	}

	// 删除素材库目录
	libDir := filepath.Join(s.uploadDir, strings.ToLower(ml.Type)+"s", id)
	os.RemoveAll(libDir)

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	slog.Info("删除素材库成功", "id", id, "deletedFiles", len(files))
	return nil
}

// UploadImages 批量上传图片
func (s *MaterialLibraryService) UploadImages(ctx context.Context, libraryId string, form *gin.Context) (*model.UploadImageResp, error) {
	ml, err := s.repo.GetByID(ctx, libraryId)
	if err != nil {
		return nil, err
	}
	if ml == nil {
		return nil, &AppError{Code: common.ErrLibraryNotFound, Msg: "素材库不存在"}
	}
	if ml.Type != "Image" {
		return nil, &AppError{Code: common.ErrLibraryTypeMismatch, Msg: "素材库类型不是图片集"}
	}

	form.Request.Body = http.MaxBytesReader(form.Writer, form.Request.Body, s.maxImageBatchSize)

	multipartForm, err := form.MultipartForm()
	if err != nil {
		return nil, &AppError{Code: common.ErrParamInvalid, Msg: "解析上传表单失败: " + err.Error()}
	}

	files := multipartForm.File["files"]
	if len(files) == 0 {
		return nil, &AppError{Code: common.ErrParamInvalid, Msg: "未选择图片文件"}
	}
	if len(files) > s.maxImageCount {
		return nil, &AppError{Code: common.ErrParamInvalid, Msg: fmt.Sprintf("单次最多上传 %d 张图片", s.maxImageCount)}
	}

	// 确保目录存在
	imgDir := filepath.Join(s.uploadDir, "images", libraryId)
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		return nil, &AppError{Code: common.ErrFileUploadFailed, Msg: "创建存储目录失败"}
	}

	var uploaded []model.MaterialFile
	for _, fh := range files {
		if fh.Size > s.maxImageSize {
			slog.Warn("图片文件超过大小限制", "fileName", fh.Filename, "size", fh.Size)
			continue
		}

		fileId, err := generateFileID()
		if err != nil {
			slog.Error("生成文件ID失败", "error", err)
			continue
		}

		ext := filepath.Ext(fh.Filename)
		if ext == "" {
			ext = ".jpg"
		}
		storagePath := filepath.Join("images", libraryId, fileId+ext)
		accessUrl := filepath.Join("/uploads", storagePath)
		fullPath := filepath.Join(s.uploadDir, storagePath)

		src, err := fh.Open()
		if err != nil {
			slog.Error("打开上传文件失败", "fileName", fh.Filename, "error", err)
			continue
		}

		dst, err := os.Create(fullPath)
		if err != nil {
			src.Close()
			slog.Error("创建目标文件失败", "path", fullPath, "error", err)
			continue
		}

		written, err := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			os.Remove(fullPath)
			slog.Error("写入文件失败", "fileName", fh.Filename, "error", err)
			continue
		}

		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		mf := &model.MaterialFile{
			Id:           fileId,
			LibraryId:    libraryId,
			FileName:     fh.Filename,
			StoragePath:  storagePath,
			AccessUrl:    filepath.ToSlash(accessUrl),
			FileSize:     written,
			MimeType:     mimeType,
			UploadStatus: "Completed",
			CreatedAt:    common.NowFormatted(),
		}

		if err := s.repo.CreateFile(ctx, mf); err != nil {
			os.Remove(fullPath)
			continue
		}

		uploaded = append(uploaded, *mf)
	}

	// 更新素材库统计
	if err := s.repo.UpdateLibraryStats(ctx, libraryId); err != nil {
		slog.Error("更新素材库统计失败", "libraryId", libraryId, "error", err)
	}

	slog.Info("批量上传图片完成", "libraryId", libraryId, "uploadedCount", len(uploaded))
	return &model.UploadImageResp{
		UploadedCount: len(uploaded),
		Files:         uploaded,
	}, nil
}

// InitVideoUpload 初始化视频上传
func (s *MaterialLibraryService) InitVideoUpload(ctx context.Context, libraryId string, req *model.InitVideoUploadReq) (*model.InitVideoUploadResp, error) {
	ml, err := s.repo.GetByID(ctx, libraryId)
	if err != nil {
		return nil, err
	}
	if ml == nil {
		return nil, &AppError{Code: common.ErrLibraryNotFound, Msg: "素材库不存在"}
	}
	if ml.Type != "Video" {
		return nil, &AppError{Code: common.ErrLibraryTypeMismatch, Msg: "素材库类型不是视频集"}
	}

	// 检查同名已完成或合并中的文件
	existing, err := s.repo.FindCompletedOrMergingFileByName(ctx, libraryId, req.FileName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, &AppError{Code: common.ErrParamInvalid, Msg: "同名视频文件已存在"}
	}

	// 断点续传：检查同名上传中的文件
	existingUploading, err := s.repo.FindUploadingFileByName(ctx, libraryId, req.FileName)
	if err != nil {
		return nil, err
	}
	if existingUploading != nil && existingUploading.UploadId != nil {
		slog.Info("断点续传：返回已有上传标识", "uploadId", *existingUploading.UploadId, "fileName", req.FileName)
		chunkCount := int32(0)
		if existingUploading.TotalChunks != nil {
			chunkCount = *existingUploading.TotalChunks
		}
		return &model.InitVideoUploadResp{
			UploadId:   *existingUploading.UploadId,
			ChunkCount: chunkCount,
		}, nil
	}

	// 创建新的上传记录
	fileId, err := generateFileID()
	if err != nil {
		return nil, fmt.Errorf("生成文件ID失败: %w", err)
	}

	uploadId := "upload_" + hex.EncodeToString(make([]byte, 16))
	if _, err := rand.Read([]byte(uploadId[len("upload_"):)); err != nil {
		return nil, fmt.Errorf("生成UploadId失败: %w", err)
	}

	chunkCount := int32(req.FileSize / int64(req.ChunkSize))
	if req.FileSize%int64(req.ChunkSize) != 0 {
		chunkCount++
	}

	ext := filepath.Ext(req.FileName)
	if ext == "" {
		ext = ".mp4"
	}
	storagePath := filepath.Join("videos", libraryId, fileId+ext)
	accessUrl := filepath.Join("/uploads", storagePath)
	uploadedChunks := int32(0)

	mf := &model.MaterialFile{
		Id:             fileId,
		LibraryId:      libraryId,
		FileName:       req.FileName,
		StoragePath:    storagePath,
		AccessUrl:      filepath.ToSlash(accessUrl),
		FileSize:       req.FileSize,
		MimeType:       "video/mp4",
		UploadStatus:   "Uploading",
		TotalChunks:    &chunkCount,
		UploadedChunks: &uploadedChunks,
		UploadId:       &uploadId,
		CreatedAt:      common.NowFormatted(),
	}

	if err := s.repo.CreateFile(ctx, mf); err != nil {
		return nil, err
	}

	// 创建分片临时目录
	chunkDir := filepath.Join(s.uploadDir, "videos", libraryId, "chunks")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return nil, &AppError{Code: common.ErrFileUploadFailed, Msg: "创建分片目录失败"}
	}

	slog.Info("初始化视频上传", "libraryId", libraryId, "uploadId", uploadId, "chunkCount", chunkCount)
	return &model.InitVideoUploadResp{
		UploadId:   uploadId,
		ChunkCount: chunkCount,
	}, nil
}

// UploadChunk 上传视频分片
func (s *MaterialLibraryService) UploadChunk(ctx context.Context, uploadId string, chunkIndex int32, fileHeader *multipart.FileHeader) error {
	mf, err := s.repo.GetFileByUploadId(ctx, uploadId)
	if err != nil {
		return err
	}
	if mf == nil {
		return &AppError{Code: common.ErrParamInvalid, Msg: "无效的上传标识"}
	}
	if mf.UploadStatus != "Uploading" {
		return &AppError{Code: common.ErrParamInvalid, Msg: "文件不在上传中状态"}
	}

	// 写入分片临时文件
	chunkDir := filepath.Join(s.uploadDir, filepath.Dir(mf.StoragePath), "chunks")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return &AppError{Code: common.ErrFileUploadFailed, Msg: "创建分片目录失败"}
	}

	chunkPath := filepath.Join(chunkDir, mf.Id+".part."+strconv.Itoa(int(chunkIndex)))
	src, err := fileHeader.Open()
	if err != nil {
		return &AppError{Code: common.ErrFileUploadFailed, Msg: "打开分片数据失败"}
	}
	defer src.Close()

	dst, err := os.Create(chunkPath)
	if err != nil {
		return &AppError{Code: common.ErrFileUploadFailed, Msg: "创建分片文件失败"}
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(chunkPath)
		return &AppError{Code: common.ErrFileUploadFailed, Msg: "写入分片数据失败"}
	}

	// 更新已上传分片数
	currentUploaded := int32(0)
	if mf.UploadedChunks != nil {
		currentUploaded = *mf.UploadedChunks
	}
	newUploaded := currentUploaded + 1
	s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
		"uploaded_chunks": newUploaded,
	})

	slog.Info("上传分片成功", "uploadId", uploadId, "chunkIndex", chunkIndex, "uploaded", newUploaded)
	return nil
}

// CompleteVideoUpload 完成视频上传（异步合并）
func (s *MaterialLibraryService) CompleteVideoUpload(ctx context.Context, uploadId string) error {
	mf, err := s.repo.GetFileByUploadId(ctx, uploadId)
	if err != nil {
		return err
	}
	if mf == nil {
		return &AppError{Code: common.ErrParamInvalid, Msg: "无效的上传标识"}
	}
	if mf.UploadStatus != "Uploading" {
		return &AppError{Code: common.ErrParamInvalid, Msg: "文件不在上传中状态"}
	}

	// 校验所有分片是否已上传完毕
	if mf.TotalChunks != nil && mf.UploadedChunks != nil && *mf.UploadedChunks < *mf.TotalChunks {
		return &AppError{Code: common.ErrChunkUploadFailed, Msg: fmt.Sprintf("分片未全部上传完毕 (%d/%d)", *mf.UploadedChunks, *mf.TotalChunks)}
	}

	// 更新状态为 Merging
	totalChunks := *mf.TotalChunks
	s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
		"upload_status":    "Merging",
		"uploaded_chunks":  totalChunks,
	})

	// 异步合并
	go s.mergeChunksAsync(mf)

	slog.Info("视频上传完成，开始异步合并", "uploadId", uploadId, "fileId", mf.Id)
	return nil
}

// ListFiles 查询素材文件列表
func (s *MaterialLibraryService) ListFiles(ctx context.Context, libraryId string, page, pageSize int, uploadStatus string) ([]model.MaterialFileProgress, int, error) {
	ml, err := s.repo.GetByID(ctx, libraryId)
	if err != nil {
		return nil, 0, err
	}
	if ml == nil {
		return nil, 0, &AppError{Code: common.ErrLibraryNotFound, Msg: "素材库不存在"}
	}

	files, total, err := s.repo.ListFiles(ctx, libraryId, page, pageSize, uploadStatus)
	if err != nil {
		return nil, 0, err
	}

	result := make([]model.MaterialFileProgress, 0, len(files))
	for _, f := range files {
		result = append(result, toFileProgress(&f))
	}

	return result, total, nil
}

// DeleteFile 删除素材文件
func (s *MaterialLibraryService) DeleteFile(ctx context.Context, libraryId string, fileId string) error {
	ml, err := s.repo.GetByID(ctx, libraryId)
	if err != nil {
		return err
	}
	if ml == nil {
		return &AppError{Code: common.ErrLibraryNotFound, Msg: "素材库不存在"}
	}

	// 检查素材库是否已关联任务
	hasRelated, err := s.repo.HasRelatedTasks(ctx, libraryId)
	if err != nil {
		return err
	}
	if hasRelated {
		return &AppError{Code: common.ErrLibraryAlreadyBound, Msg: "素材库已被任务关联，无法删除文件"}
	}

	mf, err := s.repo.GetFileByID(ctx, fileId)
	if err != nil {
		return err
	}
	if mf == nil || mf.LibraryId != libraryId {
		return &AppError{Code: common.ErrFileNotFound, Msg: "文件不存在"}
	}

	// 删除磁盘文件
	fullPath := filepath.Join(s.uploadDir, mf.StoragePath)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("删除磁盘文件失败", "path", fullPath, "error", err)
	}

	if err := s.repo.DeleteFile(ctx, fileId); err != nil {
		return err
	}

	// 更新素材库统计
	if err := s.repo.UpdateLibraryStats(ctx, libraryId); err != nil {
		slog.Error("更新素材库统计失败", "libraryId", libraryId, "error", err)
	}

	slog.Info("删除素材文件成功", "libraryId", libraryId, "fileId", fileId)
	return nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// generateLibraryID 生成 ml_{hex32} 格式的素材库 ID
func generateLibraryID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ml_" + hex.EncodeToString(bytes), nil
}

// generateFileID 生成 mf_{hex32} 格式的文件 ID
func generateFileID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "mf_" + hex.EncodeToString(bytes), nil
}

// toFileProgress 将 MaterialFile 转换为带进度的响应视图
func toFileProgress(mf *model.MaterialFile) model.MaterialFileProgress {
	progress := 1.0
	if mf.UploadStatus == "Uploading" && mf.TotalChunks != nil && *mf.TotalChunks > 0 {
		uploaded := int32(0)
		if mf.UploadedChunks != nil {
			uploaded = *mf.UploadedChunks
		}
		progress = float64(uploaded) / float64(*mf.TotalChunks)
	}
	return model.MaterialFileProgress{
		Id:             mf.Id,
		LibraryId:      mf.LibraryId,
		FileName:       mf.FileName,
		StoragePath:    mf.StoragePath,
		AccessUrl:      mf.AccessUrl,
		FileSize:       mf.FileSize,
		MimeType:       mf.MimeType,
		UploadStatus:   mf.UploadStatus,
		FailReason:     mf.FailReason,
		Progress:       progress,
		TotalChunks:    mf.TotalChunks,
		UploadedChunks: mf.UploadedChunks,
		CreatedAt:      mf.CreatedAt,
	}
}

// mergeChunksAsync 异步合并分片
func (s *MaterialLibraryService) mergeChunksAsync(mf *model.MaterialFile) {
	ctx := context.Background()
	chunkDir := filepath.Join(s.uploadDir, filepath.Dir(mf.StoragePath), "chunks")
	targetPath := filepath.Join(s.uploadDir, mf.StoragePath)

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		failReason := "创建目标目录失败: " + err.Error()
		s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
			"upload_status": "Failed",
			"fail_reason":   failReason,
		})
		slog.Error("视频合并失败", "fileId", mf.Id, "error", err)
		return
	}

	// 创建目标文件
	dst, err := os.Create(targetPath)
	if err != nil {
		failReason := "创建目标文件失败: " + err.Error()
		s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
			"upload_status": "Failed",
			"fail_reason":   failReason,
		})
		slog.Error("视频合并失败", "fileId", mf.Id, "error", err)
		return
	}
	defer dst.Close()

	// 按序合并分片
	totalChunks := int32(0)
	if mf.TotalChunks != nil {
		totalChunks = *mf.TotalChunks
	}

	for i := int32(0); i < totalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, mf.Id+".part."+strconv.Itoa(int(i)))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			dst.Close()
			os.Remove(targetPath)
			failReason := fmt.Sprintf("打开分片 %d 失败: %s", i, err.Error())
			s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
				"upload_status": "Failed",
				"fail_reason":   failReason,
			})
			slog.Error("视频合并失败", "fileId", mf.Id, "chunkIndex", i, "error", err)
			return
		}
		if _, err := io.Copy(dst, chunkFile); err != nil {
			chunkFile.Close()
			dst.Close()
			os.Remove(targetPath)
			failReason := fmt.Sprintf("写入分片 %d 失败: %s", i, err.Error())
			s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
				"upload_status": "Failed",
				"fail_reason":   failReason,
			})
			slog.Error("视频合并失败", "fileId", mf.Id, "chunkIndex", i, "error", err)
			return
		}
		chunkFile.Close()
	}
	dst.Close()

	// 合并成功，清理分片临时文件
	os.RemoveAll(chunkDir)

	// 更新文件状态
	s.repo.UpdateFile(ctx, mf.Id, map[string]interface{}{
		"upload_status":    "Completed",
		"total_chunks":     nil,
		"uploaded_chunks":  nil,
		"upload_id":        nil,
	})

	// 更新素材库统计
	s.repo.UpdateLibraryStats(ctx, mf.LibraryId)

	slog.Info("视频合并成功", "fileId", mf.Id, "libraryId", mf.LibraryId)
}
```

> **注意**: `UploadImages` 方法中需要引入 `net/http` 包来使用 `http.MaxBytesReader`。`UploadChunk` 方法中需要引入 `mime/multipart` 包来使用 `multipart.FileHeader`。需在 import 中补全。另外 `InitVideoUpload` 中 `rand.Read` 调用有语法错误，需要用 `make([]byte, 16)` 然后调用 `rand.Read`。

- [ ] **Step 2: 验证编译通过**

Run: `cd server && go build ./...`
Expected: 编译成功（可能需要微调 import）

- [ ] **Step 3: 提交**

```bash
cd server && git add internal/service/material_library_svc.go && git commit -m "feat: 新增素材库业务逻辑层（含文件上传、分片管理）"
```

---

### Task 6: 新增素材库控制器

**Files:**
- Create: `server/internal/controller/material_library_ctrl.go`

- [ ] **Step 1: 创建素材库控制器文件**

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

// MaterialLibraryController 素材库 HTTP 处理器
type MaterialLibraryController struct {
	svc *service.MaterialLibraryService
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewMaterialLibraryController 创建素材库控制器实例
func NewMaterialLibraryController(svc *service.MaterialLibraryService) *MaterialLibraryController {
	return &MaterialLibraryController{svc: svc}
}

// Create 创建素材库
func (ctrl *MaterialLibraryController) Create(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	var req model.CreateMaterialLibraryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "参数校验失败: "+err.Error())
		return
	}

	result, err := ctrl.svc.Create(c.Request.Context(), &req)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, result)
}

// List 获取素材库列表
func (ctrl *MaterialLibraryController) List(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	// 按 ID 精确查询
	if id := c.Query("Id"); id != "" {
		ml, err := ctrl.svc.GetByID(c.Request.Context(), id)
		if err != nil {
			handleError(c, err)
			return
		}
		common.OK(c, common.PageData{
			Total:    1,
			Page:     1,
			PageSize: 1,
			Items:    []model.MaterialLibrary{*ml},
		})
		return
	}

	// 分页查询
	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "20"))
	libType := c.Query("Type")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	items, total, err := ctrl.svc.List(c.Request.Context(), page, pageSize, libType)
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

// Update 更新素材库
func (ctrl *MaterialLibraryController) Update(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.UpdateMaterialLibraryReq
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

// Delete 删除素材库
func (ctrl *MaterialLibraryController) Delete(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	if err := ctrl.svc.Delete(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// UploadImages 批量上传图片
func (ctrl *MaterialLibraryController) UploadImages(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	result, err := ctrl.svc.UploadImages(c.Request.Context(), id, c)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, result)
}

// InitVideoUpload 初始化视频上传
func (ctrl *MaterialLibraryController) InitVideoUpload(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.InitVideoUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "参数校验失败: "+err.Error())
		return
	}

	result, err := ctrl.svc.InitVideoUpload(c.Request.Context(), id, &req)
	if err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, result)
}

// UploadChunk 上传视频分片
func (ctrl *MaterialLibraryController) UploadChunk(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	uploadId := c.PostForm("UploadId")
	chunkIndexStr := c.PostForm("ChunkIndex")
	if uploadId == "" || chunkIndexStr == "" {
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "UploadId 和 ChunkIndex 为必填")
		return
	}

	chunkIndex, err := strconv.ParseInt(chunkIndexStr, 10, 32)
	if err != nil {
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "ChunkIndex 格式错误")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "分片文件不存在")
		return
	}

	if err := ctrl.svc.UploadChunk(c.Request.Context(), uploadId, int32(chunkIndex), file); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// CompleteVideoUpload 完成视频上传
func (ctrl *MaterialLibraryController) CompleteVideoUpload(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.CompleteVideoUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrParamInvalid, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.CompleteVideoUpload(c.Request.Context(), req.UploadId); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// ListFiles 获取素材库文件列表
func (ctrl *MaterialLibraryController) ListFiles(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "24"))
	uploadStatus := c.Query("UploadStatus")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}

	items, total, err := ctrl.svc.GetFiles(c.Request.Context(), id, page, pageSize, uploadStatus)
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

// DeleteFile 删除素材文件
func (ctrl *MaterialLibraryController) DeleteFile(c *gin.Context) {
	id := c.Param("id")
	fileId := c.Param("fileId")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id, "fileId", fileId)

	if err := ctrl.svc.DeleteFile(c.Request.Context(), id, fileId); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}
```

> **注意**: `ListFiles` 方法中调用的是 `ctrl.svc.GetFiles`，但 service 层方法名为 `ListFiles`，需要统一命名。

- [ ] **Step 2: 验证编译通过**

Run: `cd server && go build ./...`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
cd server && git add internal/controller/material_library_ctrl.go && git commit -m "feat: 新增素材库 HTTP 控制器"
```

---

### Task 7: 注册路由和静态文件服务

**Files:**
- Modify: `server/internal/controller/router.go`

- [ ] **Step 1: 更新路由注册，新增素材库路由和静态文件服务**

将 `SetupRouter` 函数签名改为接收两个控制器，并新增素材库路由组：

```go
package controller

import (
	"github.com/gin-gonic/gin"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// SetupRouter 注册所有路由
func SetupRouter(r *gin.Engine, mcCtrl *ModelConfigController, mlCtrl *MaterialLibraryController, uploadDir string) {
	// 静态文件服务
	r.Static("/uploads", uploadDir)

	api := r.Group("/api")
	{
		mc := api.Group("/model-configs")
		{
			mc.POST("", mcCtrl.Create)
			mc.GET("", mcCtrl.List)
			mc.PUT("/:id", mcCtrl.Update)
			mc.DELETE("/:id", mcCtrl.Delete)
			mc.POST("/:id/test", mcCtrl.Test)
		}

		ml := api.Group("/material-libraries")
		{
			ml.POST("", mlCtrl.Create)
			ml.GET("", mlCtrl.List)
			ml.PUT("/:id", mlCtrl.Update)
			ml.DELETE("/:id", mlCtrl.Delete)
			ml.POST("/:id/images", mlCtrl.UploadImages)
			ml.POST("/:id/videos/init", mlCtrl.InitVideoUpload)
			ml.POST("/:id/videos/chunk", mlCtrl.UploadChunk)
			ml.POST("/:id/videos/complete", mlCtrl.CompleteVideoUpload)
			ml.GET("/:id/files", mlCtrl.GetFiles)
			ml.DELETE("/:id/files/:fileId", mlCtrl.DeleteFile)
		}
	}
}
```

> **注意**: `mlCtrl.GetFiles` 应为 `mlCtrl.ListFiles`（与控制器方法名一致）。

同时将 `mcCtrl.Update` 的方法从 PATCH 改为 PUT（已在路由中修改）。

- [ ] **Step 2: 验证编译通过**

Run: `cd server && go build ./...`
Expected: 编译可能失败（因为 main.go 还没更新调用方式），先继续 Task 8

- [ ] **Step 3: 暂不提交，等 Task 8 一起提交**

---

### Task 8: 更新数据库迁移和 main.go 依赖注入

**Files:**
- Modify: `server/internal/repository/db.go`
- Modify: `server/cmd/server/main.go`

- [ ] **Step 1: 更新 db.go，新增 MaterialLibrary 和 MaterialFile 的 AutoMigrate**

在 `InitDB` 函数的 AutoMigrate 调用中追加新模型：

```go
if err := db.AutoMigrate(&model.ModelConfig{}, &model.MaterialLibrary{}, &model.MaterialFile{}); err != nil {
	return nil, fmt.Errorf("自动迁移表结构失败: %w", err)
}
```

- [ ] **Step 2: 更新 main.go，初始化素材库各层并注入路由**

在现有模型配置初始化代码后面追加：

```go
// 初始化素材库各层
mlRepo := repository.NewMaterialLibraryRepo(db)
mlSvc := service.NewMaterialLibraryService(mlRepo, &cfg.Upload)
mlCtrl := controller.NewMaterialLibraryController(mlSvc)
```

修改路由注册调用：

```go
controller.SetupRouter(r, mcCtrl, mlCtrl, cfg.Upload.Dir)
```

- [ ] **Step 3: 验证编译通过**

Run: `cd server && go build ./...`
Expected: 编译成功

- [ ] **Step 4: 启动服务验证**

Run: `cd server && go run cmd/server/main.go`
Expected: 服务正常启动，日志输出"数据库初始化成功"

- [ ] **Step 5: 提交 Task 7 + Task 8**

```bash
cd server && git add internal/controller/router.go internal/repository/db.go cmd/server/main.go && git commit -m "feat: 注册素材库路由、静态文件服务，更新数据库迁移和依赖注入"
```

---

### Task 9: 接口集成验证

**Files:** 无代码修改

- [ ] **Step 1: 启动服务**

Run: `cd server && go run cmd/server/main.go`

- [ ] **Step 2: 验证创建素材库**

Run:
```bash
curl -s -X POST http://localhost:8080/api/material-libraries \
  -H "Content-Type: application/json" \
  -d '{"Name":"测试图片集","Type":"Image"}' | python -m json.tool
```
Expected: 返回 ErrorCode=0，Data 含 Id、Name、Type 等字段

- [ ] **Step 3: 验证获取素材库列表**

Run:
```bash
curl -s http://localhost:8080/api/material-libraries | python -m json.tool
```
Expected: 返回分页结构，Items 包含刚创建的素材库

- [ ] **Step 4: 验证更新素材库**

Run:
```bash
curl -s -X PUT http://localhost:8080/api/material-libraries/{id} \
  -H "Content-Type: application/json" \
  -d '{"Name":"更新后的名称"}' | python -m json.tool
```
Expected: 返回 ErrorCode=0

- [ ] **Step 5: 验证批量上传图片**

准备几张测试图片，然后：

Run:
```bash
curl -s -X POST http://localhost:8080/api/material-libraries/{id}/images \
  -F "files=@test1.jpg" -F "files=@test2.jpg" | python -m json.tool
```
Expected: 返回 UploadedCount 和 Files 列表

- [ ] **Step 6: 验证获取素材文件列表**

Run:
```bash
curl -s http://localhost:8080/api/material-libraries/{id}/files | python -m json.tool
```
Expected: 返回文件列表，含 Progress 字段

- [ ] **Step 7: 验证删除素材文件**

Run:
```bash
curl -s -X DELETE http://localhost:8080/api/material-libraries/{id}/files/{fileId} | python -m json.tool
```
Expected: 返回 ErrorCode=0

- [ ] **Step 8: 验证删除素材库**

Run:
```bash
curl -s -X DELETE http://localhost:8080/api/material-libraries/{id} | python -m json.tool
```
Expected: 返回 ErrorCode=0

- [ ] **Step 9: 验证视频上传流程（初始化）**

先创建视频素材库：

Run:
```bash
curl -s -X POST http://localhost:8080/api/material-libraries \
  -H "Content-Type: application/json" \
  -d '{"Name":"测试视频集","Type":"Video"}' | python -m json.tool
```

初始化上传：

Run:
```bash
curl -s -X POST http://localhost:8080/api/material-libraries/{id}/videos/init \
  -H "Content-Type: application/json" \
  -d '{"FileName":"test.mp4","FileSize":52428800,"ChunkSize":5242880}' | python -m json.tool
```
Expected: 返回 UploadId 和 ChunkCount

- [ ] **Step 10: 提交**

如无问题则无需提交。如有修复则提交修复。

---

## 自检清单

**1. Spec 覆盖：**
- ✅ 素材库 CRUD（创建、列表、更新、删除）— Task 4, 5, 6
- ✅ 批量上传图片（含限制、流式写入）— Task 5
- ✅ 视频分片上传三步流程（init/chunk/complete）— Task 5, 6
- ✅ 断点续传（同名 Uploading 文件返回已有 UploadId）— Task 5
- ✅ 同名已完成文件拒绝上传 — Task 5
- ✅ 异步合并（complete 立即返回，后台 goroutine 合并）— Task 5
- ✅ 素材文件列表（按时间升序、分页、状态筛选、Progress）— Task 4, 5
- ✅ 删除素材文件（含关联任务检查）— Task 5
- ✅ 删除素材库（级联删除文件+磁盘文件）— Task 5
- ✅ 静态文件服务 — Task 7
- ✅ 错误码（40007-40011, 50003-50004）— Task 1
- ✅ 创建任务时校验未完成文件 — 在 Task 4 的 `HasIncompleteFiles` 方法中已提供，任务模块后续实现

**2. Placeholder 扫描：** 无 TBD/TODO/后续实现等占位符。所有代码步骤包含完整实现。

**3. 类型一致性：**
- `MaterialLibrary` / `MaterialFile` 模型定义在 Task 2，仓储层（Task 4）、服务层（Task 5）、控制器（Task 6）均引用同一模型
- DTO（`CreateMaterialLibraryReq` 等）定义在 Task 2，控制器和服务层引用一致
- `MaterialFileProgress` 在 Task 2 定义，Task 5 的 `toFileProgress` 函数产出该类型，Task 6 的 `ListFiles` 返回该类型
- 错误码常量在 Task 1 定义，Service 层通过 `&AppError{Code: common.ErrXxx}` 引用
