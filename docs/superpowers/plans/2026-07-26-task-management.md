# 任务管理模块实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现任务管理的 4 个 API 接口（创建、列表、删除、暂停/恢复）及调度器，支持图片集和视频集任务的自动排队执行。

**Architecture:** DB 持久化 + 内存 channel 队列 + 单 Worker 协程。任务创建后写入 DB 并入队，Worker 逐个消费执行。暂停/恢复通过 context 取消/重建控制。重启时从 DB 恢复未完成任务。

**Tech Stack:** Go 1.25 / Gin / GORM / SQLite / ffmpeg-go / openai-go v3

---

## 文件结构

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 创建 | `server/internal/model/task.go` | Task/Image 实体 + DTO |
| 创建 | `server/internal/repository/task_repo.go` | Task/Image 数据访问 + Progress 聚合 |
| 创建 | `server/internal/service/task_svc.go` | 任务业务逻辑（CRUD + 暂停/恢复） |
| 创建 | `server/internal/service/task_executor.go` | 调度器：队列、Worker、抽帧、LLM 调用、结果解析 |
| 创建 | `server/internal/controller/task_ctrl.go` | 任务 HTTP handler |
| 修改 | `server/internal/controller/router.go` | 新增任务路由 |
| 修改 | `server/internal/repository/db.go` | AutoMigrate 加入 Task, Image |
| 修改 | `server/internal/repository/model_config_repo.go` | 实现 HasRelatedTasks |
| 修改 | `server/internal/repository/material_library_repo.go` | 实现 HasRelatedTasks |
| 修改 | `server/internal/llm/options.go` | Timeout 改为毫秒 |
| 修改 | `server/internal/llm/client.go` | 适配 TimeoutMs |
| 修改 | `server/internal/service/model_config_svc.go` | 无需修改（HasRelatedTasks 已调用） |
| 修改 | `server/cmd/server/main.go` | 初始化任务模块、启动调度器 |

---

### Task 1: 数据模型 — Task 和 Image 实体

**Files:**
- 创建: `server/internal/model/task.go`
- 修改: `server/internal/repository/db.go`

- [ ] **Step 1: 创建 task.go 数据模型**

```go
package model

import "database/sql"

// ──────────────────────────── 结构体 ────────────────────────────

// Task 任务实体（对应 tasks 表）
type Task struct {
	// Id 任务唯一标识
	Id string `json:"Id" gorm:"primaryKey;column:id"`
	// Name 任务名称
	Name string `json:"Name" gorm:"column:name;not null"`
	// Type 任务类型：Image / Video
	Type string `json:"Type" gorm:"column:type;not null"`
	// Status 任务状态：Pending / Analyzing / Paused / Completed
	Status string `json:"Status" gorm:"column:status;not null;default:Pending"`
	// ModelConfigId 使用的模型配置 ID
	ModelConfigId string `json:"ModelConfigId" gorm:"column:model_config_id;not null"`
	// MaterialLibraryId 关联素材库 ID
	MaterialLibraryId string `json:"MaterialLibraryId" gorm:"column:material_library_id;not null"`
	// Prompt 下发给大模型的提示词
	Prompt string `json:"Prompt" gorm:"column:prompt;not null"`
	// Target 检测目标名称
	Target string `json:"Target" gorm:"column:target;not null"`
	// FrameInterval 抽帧间隔（秒），Type=Video 时有值
	FrameInterval *int32 `json:"FrameInterval" gorm:"column:frame_interval"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt" gorm:"column:created_at;not null"`
	// UpdatedAt 最后更新时间
	UpdatedAt string `json:"UpdatedAt" gorm:"column:updated_at;not null"`
}

// Image 检测素材实体（对应 images 表）
type Image struct {
	// Id 素材唯一标识
	Id string `json:"Id" gorm:"primaryKey;column:id"`
	// TaskId 所属任务 ID
	TaskId string `json:"TaskId" gorm:"column:task_id;not null;index"`
	// AccessUrl 图片/帧浏览器访问 URL
	AccessUrl string `json:"AccessUrl" gorm:"column:access_url;not null"`
	// MaterialFileId 关联原始素材文件 ID，图片集有值，视频帧为 null
	MaterialFileId *string `json:"MaterialFileId" gorm:"column:material_file_id"`
	// FrameIndex 视频帧序号，视频帧有值
	FrameIndex *int32 `json:"FrameIndex" gorm:"column:frame_index"`
	// Status 检测状态：Pending / Detected / NotDetected / Failed
	Status string `json:"Status" gorm:"column:status;not null;default:Pending"`
	// Detection 检测结果（JSON）
	Detection sql.NullString `json:"Detection" gorm:"column:detection;type:text"`
	// FailReason 失败原因
	FailReason *string `json:"FailReason" gorm:"column:fail_reason"`
	// Correction 矫正标记：null / FalsePositive / DeletedFp
	Correction *string `json:"Correction" gorm:"column:correction"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt" gorm:"column:created_at;not null"`
}

// Box 检测框
type Box struct {
	// X1 左上角 X（0-1000 归一化）
	X1 int32 `json:"X1"`
	// Y1 左上角 Y（0-1000 归一化）
	Y1 int32 `json:"Y1"`
	// X2 右下角 X（0-1000 归一化）
	X2 int32 `json:"X2"`
	// Y2 右下角 Y（0-1000 归一化）
	Y2 int32 `json:"Y2"`
	// Confidence 置信度描述
	Confidence string `json:"Confidence"`
	// Label 目标标签
	Label string `json:"Label"`
}

// Detection 检测结果
type Detection struct {
	// HasTarget 是否检测到目标
	HasTarget bool `json:"HasTarget"`
	// Boxes 检测框列表
	Boxes []Box `json:"Boxes"`
	// RawResponse 大模型原始返回
	RawResponse string `json:"RawResponse"`
	// AnalyzedAt 分析完成时间
	AnalyzedAt string `json:"AnalyzedAt"`
}

// ──────────────────────────── 请求/响应 DTO ────────────────────────────

// CreateTaskReq 创建任务请求
type CreateTaskReq struct {
	// Name 任务名称
	Name string `json:"Name" binding:"required"`
	// Type 任务类型
	Type string `json:"Type" binding:"required,oneof=Image Video"`
	// ModelConfigId 模型配置 ID
	ModelConfigId string `json:"ModelConfigId" binding:"required"`
	// MaterialLibraryId 关联素材库 ID
	MaterialLibraryId string `json:"MaterialLibraryId" binding:"required"`
	// Prompt 下发给大模型的提示词
	Prompt string `json:"Prompt" binding:"required"`
	// Target 检测目标名称
	Target string `json:"Target" binding:"required"`
	// FrameInterval 抽帧间隔（秒），Type=Video 时必填
	FrameInterval *int32 `json:"FrameInterval"`
}

// UpdateTaskReq 更新任务请求（暂停/恢复）
type UpdateTaskReq struct {
	// Status 任务状态：Paused / Analyzing
	Status string `json:"Status" binding:"required,oneof=Paused Analyzing"`
}

// TaskItem 任务列表项（包含关联名称和进度）
type TaskItem struct {
	// Id 任务唯一标识
	Id string `json:"Id"`
	// Name 任务名称
	Name string `json:"Name"`
	// Type 任务类型
	Type string `json:"Type"`
	// Status 任务状态
	Status string `json:"Status"`
	// ModelConfigId 使用的模型配置 ID
	ModelConfigId string `json:"ModelConfigId"`
	// ModelConfigName 使用的模型配置名称
	ModelConfigName string `json:"ModelConfigName"`
	// MaterialLibraryId 关联素材库 ID
	MaterialLibraryId string `json:"MaterialLibraryId"`
	// MaterialLibraryName 关联素材库名称
	MaterialLibraryName string `json:"MaterialLibraryName"`
	// Prompt 下发给大模型的提示词
	Prompt string `json:"Prompt"`
	// Target 检测目标名称
	Target string `json:"Target"`
	// FrameInterval 抽帧间隔（秒）
	FrameInterval *int32 `json:"FrameInterval"`
	// Progress 检测进度与统计
	Progress TaskProgress `json:"Progress"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt"`
}

// TaskProgress 检测进度与统计
type TaskProgress struct {
	// Total 素材总数
	Total int `json:"Total"`
	// Completed 已完成检测数
	Completed int `json:"Completed"`
	// CompletedDetail 已完成明细
	CompletedDetail CompletedDetail `json:"CompletedDetail"`
	// Pending 待检测数
	Pending int `json:"Pending"`
}

// CompletedDetail 已完成明细
type CompletedDetail struct {
	// Detected 已检出目标数
	Detected int `json:"Detected"`
	// DetectedDetail 检出明细
	DetectedDetail DetectedDetail `json:"DetectedDetail"`
	// NotDetected 未检出目标数
	NotDetected int `json:"NotDetected"`
	// Failed 检测失败数
	Failed int `json:"Failed"`
}

// DetectedDetail 检出明细
type DetectedDetail struct {
	// TruePositive 正报数
	TruePositive int `json:"TruePositive"`
	// FalsePositive 误报数
	FalsePositive int `json:"FalsePositive"`
}
```

- [ ] **Step 2: 修改 db.go，AutoMigrate 加入 Task 和 Image**

在 `server/internal/repository/db.go` 的 `InitDB` 函数中，将 AutoMigrate 调用改为：

```go
if err := db.AutoMigrate(&model.ModelConfig{}, &model.MaterialLibrary{}, &model.MaterialFile{}, &model.Task{}, &model.Image{}); err != nil {
```

- [ ] **Step 3: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 4: 提交**

```bash
git add server/internal/model/task.go server/internal/repository/db.go
git commit -m "feat(task): 添加 Task 和 Image 数据模型，AutoMigrate 加入新表"
```

---

### Task 2: Repository — 任务数据访问层

**Files:**
- 创建: `server/internal/repository/task_repo.go`
- 修改: `server/internal/repository/model_config_repo.go`
- 修改: `server/internal/repository/material_library_repo.go`

- [ ] **Step 1: 创建 task_repo.go**

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

// TaskRepo 任务数据访问层
type TaskRepo struct {
	db *gorm.DB
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewTaskRepo 创建任务仓储实例
func NewTaskRepo(db *gorm.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

// Create 插入一条任务
func (r *TaskRepo) Create(ctx context.Context, task *model.Task) error {
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		slog.Error("插入任务失败", "id", task.Id, "error", err)
		return fmt.Errorf("插入任务失败: %w", err)
	}
	slog.Info("插入任务成功", "id", task.Id, "name", task.Name)
	return nil
}

// GetByID 根据 ID 查询单条任务
func (r *TaskRepo) GetByID(ctx context.Context, id string) (*model.Task, error) {
	var task model.Task
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error
	if err == gorm.ErrRecordNotFound {
		slog.Warn("任务不存在", "id", id)
		return nil, nil
	}
	if err != nil {
		slog.Error("按ID查询任务失败", "id", id, "error", err)
		return nil, fmt.Errorf("按ID查询任务失败: %w", err)
	}
	return &task, nil
}

// List 分页查询任务列表，返回列表和总数
func (r *TaskRepo) List(ctx context.Context, page, pageSize int, status string) ([]model.Task, int, error) {
	query := r.db.WithContext(ctx).Model(&model.Task{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("统计任务数量失败", "error", err)
		return nil, 0, fmt.Errorf("统计任务数量失败: %w", err)
	}

	var items []model.Task
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		slog.Error("查询任务列表失败", "page", page, "pageSize", pageSize, "error", err)
		return nil, 0, fmt.Errorf("查询任务列表失败: %w", err)
	}

	slog.Info("查询任务列表", "page", page, "pageSize", pageSize, "total", total, "count", len(items))
	return items, int(total), nil
}

// UpdateStatus 更新任务状态
func (r *TaskRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	result := r.db.WithContext(ctx).Model(&model.Task{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": common.NowFormatted(),
	})
	if result.Error != nil {
		slog.Error("更新任务状态失败", "id", id, "error", result.Error)
		return fmt.Errorf("更新任务状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete 删除任务
func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Task{})
	if result.Error != nil {
		slog.Error("删除任务失败", "id", id, "error", result.Error)
		return fmt.Errorf("删除任务失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	slog.Info("删除任务成功", "id", id)
	return nil
}

// FindByMaterialLibraryId 根据素材库 ID 查询任务
func (r *TaskRepo) FindByMaterialLibraryId(ctx context.Context, libraryId string) (*model.Task, error) {
	var task model.Task
	err := r.db.WithContext(ctx).Where("material_library_id = ?", libraryId).First(&task).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		slog.Error("按素材库ID查询任务失败", "libraryId", libraryId, "error", err)
		return nil, fmt.Errorf("按素材库ID查询任务失败: %w", err)
	}
	return &task, nil
}

// FindByModelConfigId 根据模型配置 ID 查询是否存在关联任务
func (r *TaskRepo) FindByModelConfigId(ctx context.Context, modelConfigId string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Task{}).Where("model_config_id = ?", modelConfigId).Count(&count).Error
	if err != nil {
		slog.Error("检查模型配置关联任务失败", "modelConfigId", modelConfigId, "error", err)
		return false, fmt.Errorf("检查模型配置关联任务失败: %w", err)
	}
	return count > 0, nil
}

// FindRecoverableTasks 查询可恢复的任务（Pending 或 Analyzing 状态）
func (r *TaskRepo) FindRecoverableTasks(ctx context.Context) ([]model.Task, error) {
	var tasks []model.Task
	err := r.db.WithContext(ctx).Where("status IN ?", []string{"Pending", "Analyzing"}).Find(&tasks).Error
	if err != nil {
		slog.Error("查询可恢复任务失败", "error", err)
		return nil, fmt.Errorf("查询可恢复任务失败: %w", err)
	}
	return tasks, nil
}

// ──────────────────────────── Image 数据访问 ────────────────────────────

// CreateImage 插入一条检测素材
func (r *TaskRepo) CreateImage(ctx context.Context, img *model.Image) error {
	if err := r.db.WithContext(ctx).Create(img).Error; err != nil {
		slog.Error("插入检测素材失败", "id", img.Id, "error", err)
		return fmt.Errorf("插入检测素材失败: %w", err)
	}
	return nil
}

// CreateImages 批量插入检测素材
func (r *TaskRepo) CreateImages(ctx context.Context, images []model.Image) error {
	if len(images) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&images).Error; err != nil {
		slog.Error("批量插入检测素材失败", "count", len(images), "error", err)
		return fmt.Errorf("批量插入检测素材失败: %w", err)
	}
	return nil
}

// GetImageByID 根据 ID 查询单条检测素材
func (r *TaskRepo) GetImageByID(ctx context.Context, id string) (*model.Image, error) {
	var img model.Image
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&img).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("按ID查询检测素材失败: %w", err)
	}
	return &img, nil
}

// ListPendingImages 查询任务下所有待检测素材
func (r *TaskRepo) ListPendingImages(ctx context.Context, taskId string) ([]model.Image, error) {
	var images []model.Image
	err := r.db.WithContext(ctx).Where("task_id = ? AND status = ?", taskId, "Pending").Find(&images).Error
	if err != nil {
		return nil, fmt.Errorf("查询待检测素材失败: %w", err)
	}
	return images, nil
}

// UpdateImage 更新检测素材
func (r *TaskRepo) UpdateImage(ctx context.Context, id string, updates map[string]interface{}) error {
	result := r.db.WithContext(ctx).Model(&model.Image{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		slog.Error("更新检测素材失败", "id", id, "error", result.Error)
		return fmt.Errorf("更新检测素材失败: %w", result.Error)
	}
	return nil
}

// DeleteImagesByTaskId 删除任务下所有检测素材
func (r *TaskRepo) DeleteImagesByTaskId(ctx context.Context, taskId string) error {
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskId).Delete(&model.Image{}).Error; err != nil {
		slog.Error("删除任务检测素材失败", "taskId", taskId, "error", err)
		return fmt.Errorf("删除任务检测素材失败: %w", err)
	}
	return nil
}

// CountImagesByStatus 按状态统计任务下的素材数量
func (r *TaskRepo) CountImagesByStatus(ctx context.Context, taskId string) (total int, pending int, detected int, notDetected int, failed int, truePositive int, falsePositive int, err error) {
	// 排除已删除误报的素材
	excludeDeletedFp := "task_id = ? AND (correction IS NULL OR correction != 'DeletedFp')"

	var totalVal int64
	r.db.WithContext(ctx).Model(&model.Image{}).Where(excludeDeletedFp, taskId).Count(&totalVal)
	total = int(totalVal)

	r.db.WithContext(ctx).Model(&model.Image{}).Where("task_id = ? AND status = ?", taskId, "Pending").Count(&totalVal)
	pending = int(totalVal)

	r.db.WithContext(ctx).Model(&model.Image{}).Where(excludeDeletedFp+" AND status = ?", taskId, "NotDetected").Count(&totalVal)
	notDetected = int(totalVal)

	r.db.WithContext(ctx).Model(&model.Image{}).Where(excludeDeletedFp+" AND status = ?", taskId, "Failed").Count(&totalVal)
	failed = int(totalVal)

	// Detected 且非 DeletedFp
	r.db.WithContext(ctx).Model(&model.Image{}).Where(excludeDeletedFp+" AND status = ?", taskId, "Detected").Count(&totalVal)
	detected = int(totalVal)

	// TruePositive = Detected AND Correction IS NULL
	r.db.WithContext(ctx).Model(&model.Image{}).Where("task_id = ? AND status = ? AND correction IS NULL", taskId, "Detected").Count(&totalVal)
	truePositive = int(totalVal)

	// FalsePositive = Detected AND Correction = 'FalsePositive'
	r.db.WithContext(ctx).Model(&model.Image{}).Where("task_id = ? AND status = ? AND correction = ?", taskId, "Detected", "FalsePositive").Count(&totalVal)
	falsePositive = int(totalVal)

	return
}
```

- [ ] **Step 2: 实现 model_config_repo.go 的 HasRelatedTasks**

将 `server/internal/repository/model_config_repo.go` 中 `HasRelatedTasks` 方法改为：

```go
// HasRelatedTasks 检查是否有关联任务
func (r *ModelConfigRepo) HasRelatedTasks(ctx context.Context, modelConfigId string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Task{}).Where("model_config_id = ?", modelConfigId).Count(&count).Error
	if err != nil {
		slog.Error("检查模型配置关联任务失败", "modelConfigId", modelConfigId, "error", err)
		return false, fmt.Errorf("检查模型配置关联任务失败: %w", err)
	}
	return count > 0, nil
}
```

注意：需要在文件头部 import 中加入 `"llm-test-server/internal/model"`（已存在则无需添加）。

- [ ] **Step 3: 实现 material_library_repo.go 的 HasRelatedTasks**

将 `server/internal/repository/material_library_repo.go` 中 `HasRelatedTasks` 方法改为：

```go
// HasRelatedTasks 检查素材库是否已关联任务
func (r *MaterialLibraryRepo) HasRelatedTasks(ctx context.Context, libraryId string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Task{}).Where("material_library_id = ?", libraryId).Count(&count).Error
	if err != nil {
		slog.Error("检查素材库关联任务失败", "libraryId", libraryId, "error", err)
		return false, fmt.Errorf("检查素材库关联任务失败: %w", err)
	}
	return count > 0, nil
}
```

注意：需要在文件头部 import 中加入 `"llm-test-server/internal/model"`（已存在则无需添加）。

- [ ] **Step 4: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译成功

- [ ] **Step 5: 提交**

```bash
git add server/internal/repository/task_repo.go server/internal/repository/model_config_repo.go server/internal/repository/material_library_repo.go
git commit -m "feat(task): 添加 TaskRepo，实现 HasRelatedTasks 关联校验"
```

---

### Task 3: LLMClient — Timeout 改为毫秒

**Files:**
- 修改: `server/internal/llm/options.go`
- 修改: `server/internal/llm/client.go`

- [ ] **Step 1: 修改 options.go**

将整个文件替换为：

```go
package llm

// ──────────────────────────── 结构体 ────────────────────────────

// AnalyzeOption 调用选项
type AnalyzeOption struct {
	// MaxRetries 重试次数（默认 0，不重试）
	MaxRetries int
	// TimeoutMs 单次请求超时（毫秒，0 表示使用默认值 60000ms）
	TimeoutMs int
}

// ──────────────────────────── 导出函数 ────────────────────────────

// WithRetry 设置重试次数
func WithRetry(n int) AnalyzeOption {
	return AnalyzeOption{MaxRetries: n}
}

// WithTimeoutMs 设置请求超时（毫秒）
func WithTimeoutMs(ms int) AnalyzeOption {
	return AnalyzeOption{TimeoutMs: ms}
}
```

- [ ] **Step 2: 修改 client.go**

修改 `mergeOptions` 函数：

```go
// mergeOptions 合并调用选项
func mergeOptions(opts []AnalyzeOption) AnalyzeOption {
	result := AnalyzeOption{TimeoutMs: 60000} // 默认 60 秒
	for _, o := range opts {
		if o.MaxRetries > 0 {
			result.MaxRetries = o.MaxRetries
		}
		if o.TimeoutMs > 0 {
			result.TimeoutMs = o.TimeoutMs
		}
	}
	return result
}
```

修改 `Analyze` 方法中请求超时的构造（将原来的 `opt.Timeout` 相关行改为）：

```go
	// 构造请求选项
	reqOpts := []option.RequestOption{
		option.WithMaxRetries(opt.MaxRetries),
	}
	if opt.TimeoutMs > 0 {
		reqOpts = append(reqOpts, option.WithRequestTimeout(time.Duration(opt.TimeoutMs)*time.Millisecond))
	}
```

同时删除 client.go 文件顶部 import 中不再需要的 `"time"` 包... 等等，`time.Duration` 仍然需要 time 包，所以保留。但需要删除 `defaultTimeout` 常量：

```go
// 删除以下常量
const (
	// defaultTimeout 默认请求超时
	defaultTimeout = 60 * time.Second
)
```

- [ ] **Step 3: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add server/internal/llm/options.go server/internal/llm/client.go
git commit -m "refactor(llm): AnalyzeOption.Timeout 改为 TimeoutMs 毫秒单位"
```

---

### Task 4: Service — 任务业务逻辑

**Files:**
- 创建: `server/internal/service/task_svc.go`

- [ ] **Step 1: 创建 task_svc.go**

```go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)

// ──────────────────────────── 结构体 ────────────────────────────

// TaskService 任务业务逻辑层
type TaskService struct {
	repo      *repository.TaskRepo
	mcRepo    *repository.ModelConfigRepo
	mlRepo    *repository.MaterialLibraryRepo
	scheduler *Scheduler
	uploadDir string
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewTaskService 创建任务服务实例
func NewTaskService(
	repo *repository.TaskRepo,
	mcRepo *repository.ModelConfigRepo,
	mlRepo *repository.MaterialLibraryRepo,
	scheduler *Scheduler,
	uploadDir string,
) *TaskService {
	return &TaskService{
		repo:      repo,
		mcRepo:    mcRepo,
		mlRepo:    mlRepo,
		scheduler: scheduler,
		uploadDir: uploadDir,
	}
}

// Create 创建任务
func (s *TaskService) Create(ctx context.Context, req *model.CreateTaskReq) error {
	// 校验 Type=Video 时 FrameInterval 必填
	if req.Type == "Video" && (req.FrameInterval == nil || *req.FrameInterval <= 0) {
		return common.NewErrParamValidation("视频任务必须指定抽帧间隔")
	}

	// 校验 ModelConfigId 是否存在
	mc, err := s.mcRepo.GetByID(ctx, req.ModelConfigId)
	if err != nil {
		return err
	}
	if mc == nil {
		return common.ErrModelConfigNotFound
	}

	// 校验 MaterialLibraryId 是否存在
	ml, err := s.mlRepo.GetByID(ctx, req.MaterialLibraryId)
	if err != nil {
		return err
	}
	if ml == nil {
		return common.ErrMaterialLibNotFound
	}

	// 校验素材库类型与任务类型一致
	if ml.Type != req.Type {
		return common.ErrLibTypeMismatch
	}

	// 校验素材库未被其他任务关联
	existing, err := s.repo.FindByMaterialLibraryId(ctx, req.MaterialLibraryId)
	if err != nil {
		return err
	}
	if existing != nil {
		return common.ErrMaterialLibBound
	}

	// 校验素材库中不存在未完成上传的文件
	hasIncomplete, err := s.mlRepo.HasIncompleteFiles(ctx, req.MaterialLibraryId)
	if err != nil {
		return err
	}
	if hasIncomplete {
		return common.ErrFileUploadIncomplete
	}

	// 生成任务 ID
	id, err := generateTaskID()
	if err != nil {
		return fmt.Errorf("生成任务ID失败: %w", err)
	}

	now := common.NowFormatted()
	task := &model.Task{
		Id:                id,
		Name:              req.Name,
		Type:              req.Type,
		Status:            "Pending",
		ModelConfigId:     req.ModelConfigId,
		MaterialLibraryId: req.MaterialLibraryId,
		Prompt:            req.Prompt,
		Target:            req.Target,
		FrameInterval:     req.FrameInterval,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.repo.Create(ctx, task); err != nil {
		return err
	}

	// Type=Image 时：为素材库下所有图片创建 Image 记录
	if req.Type == "Image" {
		if err := s.createImageRecords(ctx, id, req.MaterialLibraryId); err != nil {
			slog.Error("创建图片素材记录失败", "taskId", id, "error", err)
			return err
		}
	}

	// 入队调度
	s.scheduler.Enqueue(id)

	slog.Info("创建任务成功", "id", id, "type", req.Type, "name", req.Name)
	return nil
}

// GetByID 按 ID 查询任务（附带 Progress 和关联名称）
func (s *TaskService) GetByID(ctx context.Context, id string) (*model.TaskItem, error) {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, common.ErrTaskNotFound
	}
	return s.toTaskItem(ctx, task)
}

// List 分页查询任务列表
func (s *TaskService) List(ctx context.Context, page, pageSize int, status string) ([]model.TaskItem, int, error) {
	tasks, total, err := s.repo.List(ctx, page, pageSize, status)
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.TaskItem, 0, len(tasks))
	for _, t := range tasks {
		item, err := s.toTaskItem(ctx, &t)
		if err != nil {
			slog.Warn("转换任务项失败", "taskId", t.Id, "error", err)
			continue
		}
		items = append(items, *item)
	}

	return items, total, nil
}

// Delete 删除任务（级联硬删除）
func (s *TaskService) Delete(ctx context.Context, id string) error {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if task == nil {
		return common.ErrTaskNotFound
	}

	// 取消正在执行的任务
	s.scheduler.CancelTask(id)

	// 删除抽帧文件目录
	frameDir := filepath.Join(s.uploadDir, "tasks", id, "frames")
	if err := os.RemoveAll(frameDir); err != nil {
		slog.Warn("删除抽帧文件目录失败", "dir", frameDir, "error", err)
	}
	// 清理 tasks/id 空目录
	taskDir := filepath.Join(s.uploadDir, "tasks", id)
	os.Remove(taskDir)

	// 级联删除素材记录
	if err := s.repo.DeleteImagesByTaskId(ctx, id); err != nil {
		slog.Error("删除任务素材记录失败", "taskId", id, "error", err)
		return err
	}

	// 删除任务
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	slog.Info("删除任务成功", "id", id)
	return nil
}

// Update 更新任务状态（暂停/恢复）
func (s *TaskService) Update(ctx context.Context, id string, req *model.UpdateTaskReq) error {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if task == nil {
		return common.ErrTaskNotFound
	}

	switch req.Status {
	case "Paused":
		// 仅 Pending 和 Analyzing 状态可暂停
		if task.Status != "Pending" && task.Status != "Analyzing" {
			return common.ErrTaskStatusInvalid
		}
		if err := s.repo.UpdateStatus(ctx, id, "Paused"); err != nil {
			return err
		}
		// 取消 context
		s.scheduler.CancelTask(id)
		slog.Info("暂停任务成功", "id", id, "prevStatus", task.Status)

	case "Analyzing":
		// 仅 Paused 状态可恢复
		if task.Status != "Paused" {
			return common.ErrTaskStatusInvalid
		}
		if err := s.repo.UpdateStatus(ctx, id, "Analyzing"); err != nil {
			return err
		}
		// 重新入队（创建新 context）
		s.scheduler.Enqueue(id)
		slog.Info("恢复任务成功", "id", id)

	default:
		return common.NewErrParamValidation("不支持的状态: " + req.Status)
	}

	return nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// createImageRecords 为图片集任务创建 Image 记录
func (s *TaskService) createImageRecords(ctx context.Context, taskId string, libraryId string) error {
	// 查询素材库下所有已完成的文件
	files, _, err := s.mlRepo.ListFiles(ctx, libraryId, 1, 100000, "Completed")
	if err != nil {
		return err
	}

	images := make([]model.Image, 0, len(files))
	for _, f := range files {
		imgId, err := generateImageID()
		if err != nil {
			return fmt.Errorf("生成素材ID失败: %w", err)
		}
		fileId := f.Id
		images = append(images, model.Image{
			Id:             imgId,
			TaskId:         taskId,
			AccessUrl:      f.AccessUrl,
			MaterialFileId: &fileId,
			Status:         "Pending",
			CreatedAt:      common.NowFormatted(),
		})
	}

	return s.repo.CreateImages(ctx, images)
}

// toTaskItem 将 Task 实体转换为 TaskItem（附带关联名称和进度）
func (s *TaskService) toTaskItem(ctx context.Context, task *model.Task) (*model.TaskItem, error) {
	// 查询关联名称
	mc, _ := s.mcRepo.GetByID(ctx, task.ModelConfigId)
	mcName := ""
	if mc != nil {
		mcName = mc.ModelName
	}

	ml, _ := s.mlRepo.GetByID(ctx, task.MaterialLibraryId)
	mlName := ""
	if ml != nil {
		mlName = ml.Name
	}

	// 查询进度
	progress := s.getProgress(ctx, task.Id)

	return &model.TaskItem{
		Id:                 task.Id,
		Name:               task.Name,
		Type:               task.Type,
		Status:             task.Status,
		ModelConfigId:      task.ModelConfigId,
		ModelConfigName:    mcName,
		MaterialLibraryId:  task.MaterialLibraryId,
		MaterialLibraryName: mlName,
		Prompt:             task.Prompt,
		Target:             task.Target,
		FrameInterval:      task.FrameInterval,
		Progress:           progress,
		CreatedAt:          task.CreatedAt,
	}, nil
}

// getProgress 获取任务检测进度
func (s *TaskService) getProgress(ctx context.Context, taskId string) model.TaskProgress {
	total, pending, detected, notDetected, failed, truePositive, falsePositive, err := s.repo.CountImagesByStatus(ctx, taskId)
	if err != nil {
		slog.Warn("查询任务进度失败", "taskId", taskId, "error", err)
		return model.TaskProgress{}
	}

	completed := detected + notDetected + failed
	return model.TaskProgress{
		Total:     total,
		Completed: completed,
		CompletedDetail: model.CompletedDetail{
			Detected: detected,
			DetectedDetail: model.DetectedDetail{
				TruePositive:  truePositive,
				FalsePositive: falsePositive,
			},
			NotDetected: notDetected,
			Failed:      failed,
		},
		Pending: pending,
	}
}

// generateTaskID 生成 task_{uuid32} 格式的任务 ID
func generateTaskID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "task_" + hex.EncodeToString(bytes), nil
}

// generateImageID 生成 img_{uuid32} 格式的素材 ID
func generateImageID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "img_" + hex.EncodeToString(bytes), nil
}
```

- [ ] **Step 2: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译失败，因为 Scheduler 类型尚未定义。这是预期的，将在 Task 5 中创建。

- [ ] **Step 3: 暂不提交，等 Task 5 一起编译通过后提交**

---

### Task 5: Service — 调度器（Scheduler）

**Files:**
- 创建: `server/internal/service/task_executor.go`

- [ ] **Step 1: 创建 task_executor.go**

```go
package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"github.com/u2takey/ffmpeg-go"

	"llm-test-server/internal/common"
	"llm-test-server/internal/llm"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)

// ──────────────────────────── 结构体 ────────────────────────────

// TaskItem 队列中的任务项
type TaskItem struct {
	// TaskID 任务 ID
	TaskID string
	// Ctx 任务 context（由生产者创建和控制）
	Ctx context.Context
}

// Scheduler 任务调度器
type Scheduler struct {
	// taskQueue 任务队列
	taskQueue chan *TaskItem
	// cancelMap 任务 ID → CancelFunc（入队时存入，暂停/删除时调用）
	cancelMap sync.Map
	// parentCtx 全局 context，用于优雅关闭
	parentCtx context.Context
	// parentCancel 全局取消函数
	parentCancel context.CancelFunc
	// wg Worker 退出等待
	wg sync.WaitGroup
	// repo 任务仓储
	repo *repository.TaskRepo
	// mcRepo 模型配置仓储
	mcRepo *repository.ModelConfigRepo
	// mlRepo 素材库仓储
	mlRepo *repository.MaterialLibraryRepo
	// llmClient LLM 客户端
	llmClient *llm.LLMClient
	// uploadDir 文件存储根目录
	uploadDir string
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewScheduler 创建调度器实例
func NewScheduler(
	repo *repository.TaskRepo,
	mcRepo *repository.ModelConfigRepo,
	mlRepo *repository.MaterialLibraryRepo,
	llmClient *llm.LLMClient,
	uploadDir string,
) *Scheduler {
	parentCtx, parentCancel := context.WithCancel(context.Background())
	return &Scheduler{
		taskQueue:    make(chan *TaskItem, 100),
		parentCtx:    parentCtx,
		parentCancel: parentCancel,
		repo:         repo,
		mcRepo:       mcRepo,
		mlRepo:       mlRepo,
		llmClient:    llmClient,
		uploadDir:    uploadDir,
	}
}

// Start 启动 Worker
func (s *Scheduler) Start() {
	go s.worker()
	slog.Info("调度器 Worker 已启动")
}

// Stop 优雅关闭
func (s *Scheduler) Stop() {
	slog.Info("调度器开始优雅关闭...")
	s.parentCancel()
	s.wg.Wait()
	slog.Info("调度器已关闭")
}

// Enqueue 将任务入队
func (s *Scheduler) Enqueue(taskID string) {
	ctx, cancel := context.WithCancel(s.parentCtx)
	s.cancelMap.Store(taskID, cancel)
	s.taskQueue <- &TaskItem{TaskID: taskID, Ctx: ctx}
	slog.Info("任务入队", "taskId", taskID)
}

// CancelTask 取消任务（暂停或删除时调用）
func (s *Scheduler) CancelTask(taskID string) {
	if val, ok := s.cancelMap.Load(taskID); ok {
		cancel := val.(context.CancelFunc)
		cancel()
		s.cancelMap.Delete(taskID)
		slog.Info("任务 context 已取消", "taskId", taskID)
	}
}

// RecoverFromDB 从数据库恢复未完成任务
func (s *Scheduler) RecoverFromDB() {
	tasks, err := s.repo.FindRecoverableTasks(context.Background())
	if err != nil {
		slog.Error("恢复任务失败", "error", err)
		return
	}

	for _, t := range tasks {
		// 将 Analyzing 状态重置为 Pending（异常重启后需要重新执行）
		if t.Status == "Analyzing" {
			s.repo.UpdateStatus(context.Background(), t.Id, "Pending")
		}
		s.Enqueue(t.Id)
	}

	if len(tasks) > 0 {
		slog.Info("从数据库恢复任务", "count", len(tasks))
	}
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// worker 消费任务队列
func (s *Scheduler) worker() {
	for {
		select {
		case item := <-s.taskQueue:
			// 执行前检查 ctx 是否已取消
			if item.Ctx.Err() != nil {
				slog.Info("任务已取消，跳过", "taskId", item.TaskID)
				continue
			}

			s.wg.Add(1)
			s.executeTask(item)
			s.wg.Done()

		case <-s.parentCtx.Done():
			slog.Info("Worker 收到关闭信号，退出")
			return
		}
	}
}

// executeTask 执行单个任务
func (s *Scheduler) executeTask(item *TaskItem) {
	defer s.cancelMap.Delete(item.TaskID)

	// 再次检查 ctx
	if item.Ctx.Err() != nil {
		return
	}

	// 从 DB 加载任务详情
	task, err := s.repo.GetByID(item.Ctx, item.TaskID)
	if err != nil || task == nil {
		slog.Error("加载任务详情失败", "taskId", item.TaskID, "error", err)
		return
	}

	// 更新状态为 Analyzing
	if err := s.repo.UpdateStatus(item.Ctx, item.TaskID, "Analyzing"); err != nil {
		slog.Error("更新任务状态失败", "taskId", item.TaskID, "error", err)
		return
	}

	// 视频任务：先抽帧
	if task.Type == "Video" {
		if err := s.extractFrames(item.Ctx, task); err != nil {
			slog.Error("视频抽帧失败", "taskId", task.Id, "error", err)
			// 抽帧失败不终止任务，已抽帧的继续检测
		}
	}

	// 逐个素材调用 LLM
	s.detectImages(item, task)

	// 检查是否全部完成
	s.checkCompletion(item.Ctx, task.Id)
}

// extractFrames 视频抽帧并创建 Image 记录
func (s *Scheduler) extractFrames(ctx context.Context, task *model.Task) error {
	// 查找素材库下的视频文件
	files, _, err := s.mlRepo.ListFiles(ctx, task.MaterialLibraryId, 1, 10, "Completed")
	if err != nil {
		return fmt.Errorf("查询视频文件失败: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("素材库下无已完成的视频文件")
	}

	// 取第一个视频文件
	videoFile := files[0]
	videoPath := filepath.Join(s.uploadDir, videoFile.StoragePath)

	// 创建帧输出目录
	frameDir := filepath.Join(s.uploadDir, "tasks", task.Id, "frames")
	if err := os.MkdirAll(frameDir, 0755); err != nil {
		return fmt.Errorf("创建帧目录失败: %w", err)
	}

	// 检查 ctx 是否已取消
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// 使用 ffmpeg-go 抽帧
	framePattern := filepath.Join(frameDir, "frame_%04d.jpg")
	err = ffmpeg_go.Input(videoPath).
		Filter("fps", ffmpeg_go.Args{fmt.Sprintf("1/%d", *task.FrameInterval)}).
		Output(framePattern, ffmpeg_go.KwArgs{"q:v": "2"}).
		OverWriteOutput().
		Run()
	if err != nil {
		return common.NewErrVideoFrameFailed(err.Error())
	}

	// 遍历帧文件创建 Image 记录
	entries, err := os.ReadDir(frameDir)
	if err != nil {
		return fmt.Errorf("读取帧目录失败: %w", err)
	}

	images := make([]model.Image, 0, len(entries))
	for i, entry := range entries {
		if entry.IsDir() {
			continue
		}

		imgId, err := generateImageID()
		if err != nil {
			slog.Error("生成帧素材ID失败", "error", err)
			continue
		}

		frameIndex := int32(i)
		storagePath := filepath.Join("tasks", task.Id, "frames", entry.Name())
		accessUrl := filepath.Join("/uploads", storagePath)

		images = append(images, model.Image{
			Id:         imgId,
			TaskId:     task.Id,
			AccessUrl:  filepath.ToSlash(accessUrl),
			FrameIndex: &frameIndex,
			Status:     "Pending",
			CreatedAt:  common.NowFormatted(),
		})
	}

	if len(images) > 0 {
		if err := s.repo.CreateImages(ctx, images); err != nil {
			return fmt.Errorf("创建帧素材记录失败: %w", err)
		}
	}

	slog.Info("视频抽帧完成", "taskId", task.Id, "frameCount", len(images))
	return nil
}

// detectImages 逐个素材调用 LLM
func (s *Scheduler) detectImages(item *TaskItem, task *model.Task) {
	// 加载模型配置
	mc, err := s.mcRepo.GetByID(item.Ctx, task.ModelConfigId)
	if err != nil || mc == nil {
		slog.Error("加载模型配置失败", "taskId", task.Id, "modelConfigId", task.ModelConfigId, "error", err)
		return
	}

	configParam := llm.ModelConfigParam{
		ApiUrl:      mc.ApiUrl,
		ApiKey:      mc.ApiKey,
		ModelId:     mc.ModelId,
		Temperature: mc.Temperature,
		MaxTokens:   mc.MaxTokens,
	}

	for {
		// 检查 ctx 是否已取消（中途暂停/删除）
		if item.Ctx.Err() != nil {
			slog.Info("任务检测被取消", "taskId", task.Id)
			return
		}

		// 查询下一个待检测素材
		images, err := s.repo.ListPendingImages(item.Ctx, task.Id)
		if err != nil {
			slog.Error("查询待检测素材失败", "taskId", task.Id, "error", err)
			return
		}
		if len(images) == 0 {
			return // 全部完成
		}

		img := images[0]

		// 构造图片数据（base64）
		imageBase64, err := s.loadImageBase64(img)
		if err != nil {
			slog.Error("加载图片数据失败", "imageId", img.Id, "error", err)
			failReason := "加载图片数据失败: " + err.Error()
			s.repo.UpdateImage(item.Ctx, img.Id, map[string]interface{}{
				"status":      "Failed",
				"fail_reason": failReason,
			})
			continue
		}

		// 调用 LLM
		resp, err := s.llmClient.Analyze(item.Ctx, mc.Id, configParam, llm.LLMRequest{
			Prompt:      task.Prompt,
			ImageBase64: []string{imageBase64},
		}, llm.WithRetry(2), llm.WithTimeoutMs(120000))

		if err != nil {
			slog.Error("LLM 调用失败", "imageId", img.Id, "error", err)
			failReason := "LLM 调用失败: " + err.Error()
			s.repo.UpdateImage(item.Ctx, img.Id, map[string]interface{}{
				"status":      "Failed",
				"fail_reason": failReason,
			})
			continue
		}

		// 解析结果
		s.parseAndSaveResult(item.Ctx, &img, resp.Content)
	}
}

// loadImageBase64 加载图片并转为 base64 data URI
func (s *Scheduler) loadImageBase64(img model.Image) (string, error) {
	// 构造文件路径
	var fullPath string
	if img.MaterialFileId != nil && *img.MaterialFileId != "" {
		// 图片集素材：从素材文件获取路径
		mf, err := s.mlRepo.GetFileByID(context.Background(), *img.MaterialFileId)
		if err != nil || mf == nil {
			return "", fmt.Errorf("查询素材文件失败: %w", err)
		}
		fullPath = filepath.Join(s.uploadDir, mf.StoragePath)
	} else {
		// 视频帧：从 AccessUrl 推导路径
		// AccessUrl 格式: /uploads/tasks/{taskId}/frames/frame_XXXX.jpg
		relPath := img.AccessUrl
		if len(relPath) > 8 && relPath[:8] == "/uploads" {
			relPath = relPath[9:] // 去掉 "/uploads/"
		}
		fullPath = filepath.Join(s.uploadDir, relPath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("读取图片文件失败: %w", err)
	}

	mimeType := "image/jpeg"
	if filepath.Ext(fullPath) == ".png" {
		mimeType = "image/png"
	}

	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// parseAndSaveResult 解析 LLM 返回结果并保存
func (s *Scheduler) parseAndSaveResult(ctx context.Context, img *model.Image, content string) {
	detection, status, failReason := parseDetectionResult(content)

	now := common.NowFormatted()
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}

	if detection != nil {
		detectionJSON, err := json.Marshal(detection)
		if err != nil {
			slog.Error("序列化检测结果失败", "imageId", img.Id, "error", err)
		} else {
			updates["detection"] = string(detectionJSON)
		}
	}

	if failReason != "" {
		updates["fail_reason"] = failReason
	}

	if err := s.repo.UpdateImage(ctx, img.Id, updates); err != nil {
		slog.Error("保存检测结果失败", "imageId", img.Id, "error", err)
	}
}

// checkCompletion 检查任务是否全部完成
func (s *Scheduler) checkCompletion(ctx context.Context, taskId string) {
	images, err := s.repo.ListPendingImages(ctx, taskId)
	if err != nil {
		slog.Error("检查任务完成状态失败", "taskId", taskId, "error", err)
		return
	}

	if len(images) == 0 {
		if err := s.repo.UpdateStatus(ctx, taskId, "Completed"); err != nil {
			slog.Error("更新任务为已完成失败", "taskId", taskId, "error", err)
			return
		}
		slog.Info("任务检测完成", "taskId", taskId)
	}
}

// parseDetectionResult 解析 LLM 返回的检测结果
// 返回：detection 结构、素材状态、失败原因
func parseDetectionResult(content string) (*model.Detection, string, string) {
	// 尝试标准 JSON 解析
	var result struct {
		DetectedFlag bool `json:"detected_flag"`
		Detections   []struct {
			Category       string  `json:"category"`
			Bbox2d         []int32 `json:"bbox_2d"`
			ConfidenceNote string  `json:"confidence_note"`
		} `json:"detections"`
	}

	err := json.Unmarshal([]byte(content), &result)
	if err == nil {
		// JSON 解析成功
		boxes := make([]model.Box, 0)
		if result.DetectedFlag {
			for _, d := range result.Detections {
				box := model.Box{
					Label:       d.Category,
					Confidence:  d.ConfidenceNote,
				}
				if len(d.Bbox2d) >= 4 {
					box.X1 = d.Bbox2d[0]
					box.Y1 = d.Bbox2d[1]
					box.X2 = d.Bbox2d[2]
					box.Y2 = d.Bbox2d[3]
				}
				boxes = append(boxes, box)
			}
		}

		status := "NotDetected"
		if result.DetectedFlag {
			status = "Detected"
		}

		return &model.Detection{
			HasTarget:   result.DetectedFlag,
			Boxes:       boxes,
			RawResponse: content,
			AnalyzedAt:  common.NowFormatted(),
		}, status, ""
	}

	// JSON 解析失败，尝试正则降级
	slog.Warn("LLM 返回 JSON 解析失败，尝试正则降级", "content", content)

	// 正则提取 detected_flag
	flagRe := regexp.MustCompile(`"detected_flag"\s*:\s*(true|false)`)
	flagMatch := flagRe.FindStringSubmatch(content)

	if len(flagMatch) >= 2 {
		detected := flagMatch[1] == "true"

		boxes := make([]model.Box, 0)
		if detected {
			// 正则提取 bbox_2d
			bboxRe := regexp.MustCompile(`"bbox_2d"\s*:\s*\[\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\]`)
			bboxMatches := bboxRe.FindAllStringSubmatch(content, -1)

			// 正则提取 category
			catRe := regexp.MustCompile(`"category"\s*:\s*"([^"]*)"`)
			catMatches := catRe.FindAllStringSubmatch(content, -1)

			// 正则提取 confidence_note
			confRe := regexp.MustCompile(`"confidence_note"\s*:\s*"([^"]*)"`)
			confMatches := confRe.FindAllStringSubmatch(content, -1)

			maxLen := len(bboxMatches)
			if len(catMatches) > maxLen {
				maxLen = len(catMatches)
			}

			for i := 0; i < maxLen; i++ {
				box := model.Box{}
				if i < len(bboxMatches) && len(bboxMatches[i]) >= 5 {
					x1, _ := strconv.ParseInt(bboxMatches[i][1], 10, 32)
					y1, _ := strconv.ParseInt(bboxMatches[i][2], 10, 32)
					x2, _ := strconv.ParseInt(bboxMatches[i][3], 10, 32)
					y2, _ := strconv.ParseInt(bboxMatches[i][4], 10, 32)
					box.X1 = int32(x1)
					box.Y1 = int32(y1)
					box.X2 = int32(x2)
					box.Y2 = int32(y2)
				}
				if i < len(catMatches) && len(catMatches[i]) >= 2 {
					box.Label = catMatches[i][1]
				}
				if i < len(confMatches) && len(confMatches[i]) >= 2 {
					box.Confidence = confMatches[i][1]
				}
				boxes = append(boxes, box)
			}
		}

		status := "NotDetected"
		if detected {
			status = "Detected"
		}

		return &model.Detection{
			HasTarget:   detected,
			Boxes:       boxes,
			RawResponse: content,
			AnalyzedAt:  common.NowFormatted(),
		}, status, ""
	}

	// 正则也无法提取 flag
	slog.Error("无法解析检测结果", "content", content)
	return nil, "Failed", "无法解析检测结果"
}
```

- [ ] **Step 2: 安装 ffmpeg-go 依赖**

Run: `cd server && go get github.com/u2takey/ffmpeg-go`

- [ ] **Step 3: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add server/internal/service/task_svc.go server/internal/service/task_executor.go server/go.mod server/go.sum
git commit -m "feat(task): 实现任务业务逻辑和调度器"
```

---

### Task 6: Controller — 任务 HTTP handler + 路由注册

**Files:**
- 创建: `server/internal/controller/task_ctrl.go`
- 修改: `server/internal/controller/router.go`

- [ ] **Step 1: 创建 task_ctrl.go**

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

// TaskController 任务 HTTP 处理器
type TaskController struct {
	svc *service.TaskService
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewTaskController 创建任务控制器实例
func NewTaskController(svc *service.TaskService) *TaskController {
	return &TaskController{svc: svc}
}

// Create 创建任务
func (ctrl *TaskController) Create(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	var req model.CreateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.Create(c.Request.Context(), &req); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// List 获取任务列表
func (ctrl *TaskController) List(c *gin.Context) {
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)

	// 按 ID 精确查询
	if id := c.Query("Id"); id != "" {
		item, err := ctrl.svc.GetByID(c.Request.Context(), id)
		if err != nil {
			handleError(c, err)
			return
		}
		common.OK(c, common.PageData{
			Total:    1,
			Page:     1,
			PageSize: 1,
			Items:    []model.TaskItem{*item},
		})
		return
	}

	// 分页查询
	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "20"))
	status := c.Query("Status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	items, total, err := ctrl.svc.List(c.Request.Context(), page, pageSize, status)
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

// Delete 删除任务
func (ctrl *TaskController) Delete(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	if err := ctrl.svc.Delete(c.Request.Context(), id); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}

// Update 更新任务状态（暂停/恢复）
func (ctrl *TaskController) Update(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	var req model.UpdateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.Update(c.Request.Context(), id, &req); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}
```

- [ ] **Step 2: 修改 router.go — 新增任务路由**

将 `SetupRouter` 函数签名改为接收 `taskCtrl` 参数，并添加任务路由组：

```go
func SetupRouter(r *gin.Engine, mcCtrl *ModelConfigController, mlCtrl *MaterialLibraryController, taskCtrl *TaskController, uploadDir string) {
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
			ml.GET("/:id/files", mlCtrl.ListFiles)
			ml.DELETE("/:id/files/:fileId", mlCtrl.DeleteFile)
		}

		tasks := api.Group("/tasks")
		{
			tasks.POST("", taskCtrl.Create)
			tasks.GET("", taskCtrl.List)
			tasks.DELETE("/:id", taskCtrl.Delete)
			tasks.PUT("/:id", taskCtrl.Update)
		}
	}
}
```

- [ ] **Step 3: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译失败，main.go 还没更新调用签名。下一步修改。

- [ ] **Step 4: 提交**

```bash
git add server/internal/controller/task_ctrl.go server/internal/controller/router.go
git commit -m "feat(task): 添加任务 Controller 和路由注册"
```

---

### Task 7: Main — 初始化任务模块并启动调度器

**Files:**
- 修改: `server/cmd/server/main.go`

- [ ] **Step 1: 修改 main.go**

在 `mlCtrl` 初始化之后、注册路由之前，添加任务模块初始化代码，并修改 `SetupRouter` 调用：

```go
	// 初始化任务模块
	taskRepo := repository.NewTaskRepo(db)
	scheduler := service.NewScheduler(taskRepo, mcRepo, mlRepo, llmClient, cfg.Upload.Dir)
	taskSvc := service.NewTaskService(taskRepo, mcRepo, mlRepo, scheduler, cfg.Upload.Dir)
	taskCtrl := controller.NewTaskController(taskSvc)

	// 启动调度器并恢复未完成任务
	scheduler.Start()
	scheduler.RecoverFromDB()

	// 注册路由
	r := gin.Default()
	controller.SetupRouter(r, mcCtrl, mlCtrl, taskCtrl, cfg.Upload.Dir)

	// 优雅关闭
	defer scheduler.Stop()
```

注意：`defer scheduler.Stop()` 需放在 `r.Run` 之前。

完整的 main.go 改动是在 `mlCtrl` 创建后插入任务初始化，并修改 SetupRouter 调用签名。

- [ ] **Step 2: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译成功

- [ ] **Step 3: 启动服务验证**

Run: `cd server && go run ./cmd/server/main.go`
Expected: 服务启动成功，日志显示 "调度器 Worker 已启动"，数据库创建 tasks 和 images 表

- [ ] **Step 4: 提交**

```bash
git add server/cmd/server/main.go
git commit -m "feat(task): 初始化任务模块并启动调度器"
```

---

### Task 8: 集成测试验证

**Files:**
- 修改: `server/test/integration/main.go`（可选，手动测试也可）

- [ ] **Step 1: 手动测试创建图片集任务**

先确保有已创建的模型配置和图片素材库，然后：

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "Name": "测试图片任务",
    "Type": "Image",
    "ModelConfigId": "mc_xxx",
    "MaterialLibraryId": "ml_xxx",
    "Prompt": "请检测图片中是否有异常",
    "Target": "异常"
  }'
```

Expected: `{"ErrorCode":0,"ErrorMsg":"","Data":null}`

- [ ] **Step 2: 手动测试获取任务列表**

```bash
curl http://localhost:8080/api/tasks
```

Expected: 返回包含刚创建的任务，Status 为 Analyzing 或 Completed

- [ ] **Step 3: 手动测试暂停/恢复**

```bash
# 创建新任务后立即暂停
curl -X PUT http://localhost:8080/api/tasks/{taskId} \
  -H "Content-Type: application/json" \
  -d '{"Status": "Paused"}'

# 恢复
curl -X PUT http://localhost:8080/api/tasks/{taskId} \
  -H "Content-Type: application/json" \
  -d '{"Status": "Analyzing"}'
```

- [ ] **Step 4: 手动测试删除任务**

```bash
curl -X DELETE http://localhost:8080/api/tasks/{taskId}
```

Expected: `{"ErrorCode":0,"ErrorMsg":"","Data":null}`

- [ ] **Step 5: 最终提交**

如有测试修复，提交所有变更。
