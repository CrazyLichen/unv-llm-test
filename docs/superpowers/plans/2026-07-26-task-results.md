# 任务结果领域实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现任务结果领域的 3 个 API 接口：获取任务分析结果、补录漏报照片、标记素材误报/恢复

**Architecture:** 沿现有 Controller → Service → Repository 三层架构扩展，在 Model 层新增 ImageItem 和 UpdateCorrectionReq DTO，Repository 层新增 ListImages 和 GetImageByIDAndTaskId 查询方法，Service 层新增 ListImages/UploadMissedPhoto/MarkCorrection 三个业务方法，Controller 层新增三个 handler，Router 注册 3 条新路由。

**Tech Stack:** Go 1.24 / Gin / GORM / SQLite

---

## 文件变更清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `server/internal/model/task.go` | 新增 ImageItem、UpdateCorrectionReq DTO |
| 修改 | `server/internal/common/apperror.go` | 新增 ErrImageNotFound 错误码和变量 |
| 修改 | `server/internal/repository/task_repo.go` | 新增 ListImages、GetImageByIDAndTaskId 方法 |
| 修改 | `server/internal/service/task_svc.go` | 新增 ListImages、UploadMissedPhoto、MarkCorrection 方法 |
| 修改 | `server/internal/controller/task_ctrl.go` | 新增 ListImages、UploadImage、UpdateCorrection handler |
| 修改 | `server/internal/controller/router.go` | 注册 3 条新路由 |
| 修改 | `server/test/integration/task.go` | 新增任务结果领域集成测试 |

---

### Task 1: 新增错误码和 Model DTO

**Files:**
- Modify: `server/internal/common/apperror.go:51-55` (任务领域错误码区)
- Modify: `server/internal/common/apperror.go:95-99` (任务领域错误变量区)
- Modify: `server/internal/model/task.go:85-171` (DTO 区)

- [ ] **Step 1: 在 apperror.go 中新增 ErrImageNotFound 错误码和变量**

在 `ErrCodeTaskStatusInvalid = 3002` 之后新增：

```go
// ErrCodeImageNotFound 素材不存在错误码
ErrCodeImageNotFound = 3003
```

在 `ErrTaskStatusInvalid` 变量之后新增：

```go
// ErrImageNotFound 素材不存在
ErrImageNotFound = AppError{Code: ErrCodeImageNotFound, Message: "素材不存在"}
```

- [ ] **Step 2: 在 task.go 中新增 ImageItem 和 UpdateCorrectionReq DTO**

在 `DetectedDetail` 结构体之后、`CreateTaskReq` 之前新增：

```go
// ImageItem 检测素材响应 DTO
type ImageItem struct {
	// Id 素材唯一标识
	Id string `json:"Id"`
	// TaskId 所属任务 ID
	TaskId string `json:"TaskId"`
	// AccessUrl 图片/帧浏览器访问 URL
	AccessUrl string `json:"AccessUrl"`
	// MaterialFileId 关联原始素材文件 ID，图片集有值，视频帧为 null
	MaterialFileId *string `json:"MaterialFileId"`
	// FrameIndex 视频帧序号，视频帧有值
	FrameIndex *int32 `json:"FrameIndex"`
	// Status 检测状态
	Status string `json:"Status"`
	// Detection 检测结果，无检测结果时为 null
	Detection *Detection `json:"Detection"`
	// FailReason 失败原因
	FailReason *string `json:"FailReason"`
	// Correction 矫正标记：null=正常, "FalsePositive"=误报, "DeletedFp"=已删除误报
	Correction *string `json:"Correction"`
}

// UpdateCorrectionReq 矫正请求 DTO
type UpdateCorrectionReq struct {
	// Correction 矫正标记：nil=恢复正常, "FalsePositive"=标记误报, "DeletedFp"=删除误报
	Correction *string `json:"Correction"`
}
```

- [ ] **Step 3: 编译验证**

Run: `cd c:/Users/李陈/Desktop/大模型测试/server && go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 4: 提交**

```bash
git add server/internal/common/apperror.go server/internal/model/task.go
git commit -m "feat(task-results): 新增 ErrImageNotFound 错误码和 ImageItem/UpdateCorrectionReq DTO"
```

---

### Task 2: 新增 Repository 方法

**Files:**
- Modify: `server/internal/repository/task_repo.go:143-239` (Image 数据访问区)

- [ ] **Step 1: 新增 GetImageByIDAndTaskId 方法**

在 `GetImageByID` 方法之后新增：

```go
// GetImageByIDAndTaskId 按 ID + 任务 ID 精确查询检测素材
func (r *TaskRepo) GetImageByIDAndTaskId(ctx context.Context, taskId, imageId string) (*model.Image, error) {
	var img model.Image
	err := r.db.WithContext(ctx).Where("id = ? AND task_id = ?", imageId, taskId).First(&img).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("按ID和任务ID查询检测素材失败: %w", err)
	}
	return &img, nil
}
```

- [ ] **Step 2: 新增 ListImages 方法**

在 `GetImageByIDAndTaskId` 方法之后新增：

```go
// ListImages 分页查询任务下检测素材列表
func (r *TaskRepo) ListImages(ctx context.Context, taskId string, page, pageSize int, status string, correction string) ([]model.Image, int, error) {
	query := r.db.WithContext(ctx).Model(&model.Image{}).Where("task_id = ?", taskId)

	// 默认排除已删除误报的素材
	if correction == "" {
		query = query.Where("correction IS NULL OR correction != 'DeletedFp'")
	} else {
		query = query.Where("correction = ?", correction)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		slog.Error("统计素材数量失败", "taskId", taskId, "error", err)
		return nil, 0, fmt.Errorf("统计素材数量失败: %w", err)
	}

	var items []model.Image
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		slog.Error("查询素材列表失败", "taskId", taskId, "page", page, "pageSize", pageSize, "error", err)
		return nil, 0, fmt.Errorf("查询素材列表失败: %w", err)
	}

	slog.Info("查询素材列表", "taskId", taskId, "page", page, "pageSize", pageSize, "total", total, "count", len(items))
	return items, int(total), nil
}
```

- [ ] **Step 3: 编译验证**

Run: `cd c:/Users/李陈/Desktop/大模型测试/server && go build ./...`
Expected: 编译成功

- [ ] **Step 4: 提交**

```bash
git add server/internal/repository/task_repo.go
git commit -m "feat(task-results): 新增 ListImages 和 GetImageByIDAndTaskId 仓库方法"
```

---

### Task 3: 新增 Service 方法

**Files:**
- Modify: `server/internal/service/task_svc.go`

- [ ] **Step 1: 在 task_svc.go 中添加 encoding/json import**

将 import 块替换为：

```go
import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
)
```

- [ ] **Step 2: 新增 ListImages 方法**

在 `Update` 方法之后（非导出函数区块之前）新增：

```go
// ListImages 获取任务分析结果列表
func (s *TaskService) ListImages(ctx context.Context, taskId, imageId string, page, pageSize int, status, correction string) ([]model.ImageItem, int, error) {
	// 校验任务存在性
	task, err := s.repo.GetByID(ctx, taskId)
	if err != nil {
		return nil, 0, err
	}
	if task == nil {
		return nil, 0, common.ErrTaskNotFound
	}

	// 按 ImageId 精确查询
	if imageId != "" {
		img, err := s.repo.GetImageByIDAndTaskId(ctx, taskId, imageId)
		if err != nil {
			return nil, 0, err
		}
		if img == nil {
			return nil, 0, common.ErrImageNotFound
		}
		item := s.toImageItem(img)
		return []model.ImageItem{item}, 1, nil
	}

	// 分页查询
	images, total, err := s.repo.ListImages(ctx, taskId, page, pageSize, status, correction)
	if err != nil {
		return nil, 0, err
	}

	items := make([]model.ImageItem, 0, len(images))
	for i := range images {
		items = append(items, s.toImageItem(&images[i]))
	}

	return items, total, nil
}

// UploadMissedPhoto 补录漏报照片
func (s *TaskService) UploadMissedPhoto(ctx context.Context, taskId string, file *multipart.FileHeader) error {
	// 校验任务存在性
	task, err := s.repo.GetByID(ctx, taskId)
	if err != nil {
		return err
	}
	if task == nil {
		return common.ErrTaskNotFound
	}

	// 生成素材 ID
	imgId, err := generateImageID()
	if err != nil {
		return fmt.Errorf("生成素材ID失败: %w", err)
	}

	// 保存文件到磁盘
	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	frameDir := filepath.Join(s.uploadDir, "tasks", taskId, "frames")
	if err := os.MkdirAll(frameDir, 0755); err != nil {
		return common.NewErrFileUploadFailed("创建帧目录失败")
	}

	fullPath := filepath.Join(frameDir, imgId+ext)
	src, err := file.Open()
	if err != nil {
		return common.NewErrFileUploadFailed("打开上传文件失败")
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return common.NewErrFileUploadFailed("创建目标文件失败")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(fullPath)
		return common.NewErrFileUploadFailed("写入文件失败")
	}

	// 创建 Image 记录
	accessUrl := filepath.Join("/uploads/tasks", taskId, "frames", imgId+ext)
	img := &model.Image{
		Id:        imgId,
		TaskId:    taskId,
		AccessUrl: accessUrl,
		Status:    "Detected",
		Detection: sql.NullString{Valid: false},
		CreatedAt: common.NowFormatted(),
	}

	if err := s.repo.CreateImage(ctx, img); err != nil {
		os.Remove(fullPath)
		return err
	}

	slog.Info("补录漏报照片成功", "taskId", taskId, "imageId", imgId)
	return nil
}

// MarkCorrection 标记素材误报/恢复
func (s *TaskService) MarkCorrection(ctx context.Context, taskId, imageId string, correction *string) error {
	// 校验任务存在性
	task, err := s.repo.GetByID(ctx, taskId)
	if err != nil {
		return err
	}
	if task == nil {
		return common.ErrTaskNotFound
	}

	// 查询素材
	img, err := s.repo.GetImageByIDAndTaskId(ctx, taskId, imageId)
	if err != nil {
		return err
	}
	if img == nil {
		return common.ErrImageNotFound
	}

	// 校验 Correction 值域
	if correction != nil && *correction != "FalsePositive" && *correction != "DeletedFp" {
		return common.NewErrParamValidation("Correction 值域无效，允许: null, FalsePositive, DeletedFp")
	}

	// 更新 Correction 字段
	if err := s.repo.UpdateImage(ctx, imageId, map[string]interface{}{
		"correction": correction,
	}); err != nil {
		return err
	}

	slog.Info("更新矫正标记成功", "taskId", taskId, "imageId", imageId, "correction", correction)
	return nil
}
```

- [ ] **Step 3: 新增 toImageItem 辅助方法**

在非导出函数区块（`generateTaskID` 之前）新增：

```go
// toImageItem 将 Image 实体转换为 ImageItem
func (s *TaskService) toImageItem(img *model.Image) model.ImageItem {
	item := model.ImageItem{
		Id:             img.Id,
		TaskId:         img.TaskId,
		AccessUrl:      img.AccessUrl,
		MaterialFileId: img.MaterialFileId,
		FrameIndex:     img.FrameIndex,
		Status:         img.Status,
		FailReason:     img.FailReason,
		Correction:     img.Correction,
	}

	// 解析 Detection JSON
	if img.Detection.Valid {
		var det model.Detection
		if err := json.Unmarshal([]byte(img.Detection.String), &det); err == nil {
			item.Detection = &det
		}
	}

	return item
}
```

- [ ] **Step 4: 编译验证**

Run: `cd c:/Users/李陈/Desktop/大模型测试/server && go build ./...`
Expected: 编译成功

- [ ] **Step 5: 提交**

```bash
git add server/internal/service/task_svc.go
git commit -m "feat(task-results): 新增 ListImages/UploadMissedPhoto/MarkCorrection 服务方法"
```

---

### Task 4: 新增 Controller handler 和路由注册

**Files:**
- Modify: `server/internal/controller/task_ctrl.go`
- Modify: `server/internal/controller/router.go`

- [ ] **Step 1: 在 task_ctrl.go 中添加 multipart import**

将 import 块替换为：

```go
import (
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
	"llm-test-server/internal/service"
)
```

- [ ] **Step 2: 新增 ListImages handler**

在 `Update` 方法之后新增：

```go
// ListImages 获取任务分析结果列表
func (ctrl *TaskController) ListImages(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	// 按 ImageId 精确查询
	if imageId := c.Query("ImageId"); imageId != "" {
		item, total, err := ctrl.svc.ListImages(c.Request.Context(), id, imageId, 1, 1, "", "")
		if err != nil {
			handleError(c, err)
			return
		}
		common.OK(c, common.PageData{
			Total:    total,
			Page:     1,
			PageSize: 1,
			Items:    item,
		})
		return
	}

	// 分页查询
	page, _ := strconv.Atoi(c.DefaultQuery("Page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("PageSize", "24"))
	status := c.Query("Status")
	correction := c.Query("Correction")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}

	items, total, err := ctrl.svc.ListImages(c.Request.Context(), id, "", page, pageSize, status, correction)
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
```

- [ ] **Step 3: 新增 UploadImage handler**

在 `ListImages` 方法之后新增：

```go
// UploadImage 补录漏报照片
func (ctrl *TaskController) UploadImage(c *gin.Context) {
	id := c.Param("id")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id)

	file, err := c.FormFile("file")
	if err != nil {
		slog.Warn("读取上传文件失败", "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: 未选择文件")
		return
	}

	if err := ctrl.svc.UploadMissedPhoto(c.Request.Context(), id, file); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}
```

- [ ] **Step 4: 新增 UpdateCorrection handler**

在 `UploadImage` 方法之后新增：

```go
// UpdateCorrection 标记素材误报/恢复
func (ctrl *TaskController) UpdateCorrection(c *gin.Context) {
	id := c.Param("id")
	imageId := c.Param("imageId")
	slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path, "id", id, "imageId", imageId)

	var req model.UpdateCorrectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("参数校验失败", "path", c.Request.URL.Path, "id", id, "error", err.Error())
		common.Fail(c, http.StatusBadRequest, common.ErrCodeParamValidation, "参数校验失败: "+err.Error())
		return
	}

	if err := ctrl.svc.MarkCorrection(c.Request.Context(), id, imageId, req.Correction); err != nil {
		handleError(c, err)
		return
	}

	common.OK(c, nil)
}
```

- [ ] **Step 5: 注册路由**

在 `router.go` 的 tasks group 中，`tasks.PUT("/:id", taskCtrl.Update)` 之后新增三行：

```go
			tasks.GET("/:id/images", taskCtrl.ListImages)
			tasks.POST("/:id/images", taskCtrl.UploadImage)
			tasks.PUT("/:id/images/:imageId", taskCtrl.UpdateCorrection)
```

- [ ] **Step 6: 编译验证**

Run: `cd c:/Users/李陈/Desktop/大模型测试/server && go build ./...`
Expected: 编译成功

- [ ] **Step 7: 提交**

```bash
git add server/internal/controller/task_ctrl.go server/internal/controller/router.go
git commit -m "feat(task-results): 新增 ListImages/UploadImage/UpdateCorrection handler 和路由"
```

---

### Task 5: 集成测试

**Files:**
- Modify: `server/test/integration/task.go`

- [ ] **Step 1: 在 getTaskTests 中新增测试用例**

在"异常场景"区块之前、"任务管理-视频检测"区块之后，新增"任务结果领域"测试区块：

```go
		// 任务结果领域
		{"任务结果领域", "查询图片任务分析结果", "分页查询已完成图片任务的素材检测结果，验证返回包含 Detection 字段", "Total>=2，Items 非空，Detection 字段有值", testListTaskImages},
		{"任务结果领域", "按状态筛选素材", "查询图片任务中 Status=Detected 的素材，验证筛选生效", "返回结果全部为 Detected 状态", testListImagesByStatus},
		{"任务结果领域", "补录漏报照片", "向已完成的图片任务上传一张补录照片，验证创建成功且列表体现", "上传成功，素材列表 Total 增加", testUploadMissedPhoto},
		{"任务结果领域", "标记素材误报", "将一个 Detected 素材标记为 FalsePositive，验证 Progress 统计自动调整", "FalsePositive 计数增加，TruePositive 减少", testMarkFalsePositive},
		{"任务结果领域", "删除误报素材", "将 FalsePositive 素材标记为 DeletedFp，验证默认列表不返回", "DeletedFp 素材不在默认列表中", testMarkDeletedFp},
		{"任务结果领域", "恢复误报素材", "将 DeletedFp 素材恢复为正常（Correction=null），验证 Progress 恢复", "素材恢复正常，Progress 统计恢复", testRestoreCorrection},
```

- [ ] **Step 2: 新增测试变量**

在 `videoTaskID` 变量之后新增：

```go
	missedPhotoImageID string
	correctionImageID  string
```

- [ ] **Step 3: 新增测试函数**

在"辅助函数"区块之前新增：

```go
// ──────────────────────────── 任务结果领域 ────────────────────────────

func testListTaskImages() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}

	resp := doJSON("GET", taskAPIPrefix+"/"+imageTaskID+"/images?Page=1&PageSize=24", nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	if pageData.Total < 2 {
		return failf("Total=%d, 期望>=2", pageData.Total)
	}

	itemsJSON, _ := json.Marshal(pageData.Items)
	var items []model.ImageItem
	json.Unmarshal(itemsJSON, &items)

	if len(items) == 0 {
		return fail("Items 为空")
	}

	// 验证第一条记录的字段
	first := items[0]
	if first.Id == "" {
		return fail("ImageItem.Id 为空")
	}
	if first.TaskId != imageTaskID {
		return failf("TaskId 不匹配: 期望=%s, 实际=%s", imageTaskID, first.TaskId)
	}
	if first.AccessUrl == "" {
		return fail("AccessUrl 为空")
	}
	if first.Status == "" {
		return fail("Status 为空")
	}

	// 记录一个 Detected 素材 ID 用于后续矫正测试
	correctionImageID = ""
	for _, item := range items {
		if item.Status == "Detected" && item.Correction == nil {
			correctionImageID = item.Id
			break
		}
	}

	return passf("查询成功, Total=%d, count=%d, 素材字段完整", pageData.Total, len(items))
}

func testListImagesByStatus() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}

	resp := doJSON("GET", taskAPIPrefix+"/"+imageTaskID+"/images?Page=1&PageSize=24&Status=Detected", nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	if pageData.Total == 0 {
		return pass("没有 Detected 素材（可能全部 NotDetected）")
	}

	itemsJSON, _ := json.Marshal(pageData.Items)
	var items []model.ImageItem
	json.Unmarshal(itemsJSON, &items)

	for _, item := range items {
		if item.Status != "Detected" {
			return failf("筛选失败: 存在非 Detected 状态的素材, Status=%s", item.Status)
		}
	}

	return passf("筛选成功, Total=%d, 全部为 Detected", pageData.Total)
}

func testUploadMissedPhoto() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}

	// 查询当前 Total
	beforeResp := doJSON("GET", taskAPIPrefix+"/"+imageTaskID+"/images?Page=1&PageSize=1", nil)
	var beforePage common.PageData
	json.Unmarshal(beforeResp.Data, &beforePage)
	beforeTotal := beforePage.Total

	// 上传一张测试图片
	imgFiles := findTestImages()
	if len(imgFiles) == 0 {
		return skip("没有测试图片可上传")
	}

	uploadResp := doMultipart(taskAPIPrefix+"/"+imageTaskID+"/images", []string{imgFiles[0]})
	if uploadResp.ErrorCode != 0 {
		return failf("补录照片失败: ErrorCode=%d, ErrorMsg=%s", uploadResp.ErrorCode, uploadResp.ErrorMsg)
	}

	// 查询更新后的 Total
	time.Sleep(500 * time.Millisecond)
	afterResp := doJSON("GET", taskAPIPrefix+"/"+imageTaskID+"/images?Page=1&PageSize=100", nil)
	var afterPage common.PageData
	json.Unmarshal(afterResp.Data, &afterPage)

	if afterPage.Total <= beforeTotal {
		return failf("补录后 Total 未增加: before=%d, after=%d", beforeTotal, afterPage.Total)
	}

	// 查找补录的照片（Status=Detected, Detection=null）
	itemsJSON, _ := json.Marshal(afterPage.Items)
	var items []model.ImageItem
	json.Unmarshal(itemsJSON, &items)

	for _, item := range items {
		if item.Status == "Detected" && item.Detection == nil {
			missedPhotoImageID = item.Id
			break
		}
	}

	if missedPhotoImageID == "" {
		return fail("补录照片未在列表中找到")
	}

	return passf("补录成功, Total: %d -> %d, 补录照片ID=%s", beforeTotal, afterPage.Total, missedPhotoImageID)
}

func testMarkFalsePositive() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}
	if correctionImageID == "" {
		return skip("没有可用于矫正测试的 Detected 素材")
	}

	// 标记误报
	resp := doJSON("PUT", taskAPIPrefix+"/"+imageTaskID+"/images/"+correctionImageID, map[string]interface{}{
		"Correction": "FalsePositive",
	})
	if resp.ErrorCode != 0 {
		return failf("标记误报失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 查询任务进度验证 FalsePositive 增加
	taskResp := doJSON("GET", taskAPIPrefix+"?Id="+imageTaskID, nil)
	item := parseFirstTaskItem(taskResp.Data)
	if item == nil {
		return fail("解析任务项失败")
	}

	if item.Progress.CompletedDetail.DetectedDetail.FalsePositive == 0 {
		return fail("标记误报后 FalsePositive 仍为 0")
	}

	// 验证素材列表中该素材的 Correction 字段
	imgResp := doJSON("GET", taskAPIPrefix+"/"+imageTaskID+"/images?ImageId="+correctionImageID, nil)
	var imgPage common.PageData
	json.Unmarshal(imgResp.Data, &imgPage)
	imgItemsJSON, _ := json.Marshal(imgPage.Items)
	var imgItems []model.ImageItem
	json.Unmarshal(imgItemsJSON, &imgItems)

	if len(imgItems) == 0 {
		return fail("查询矫正素材失败")
	}
	if imgItems[0].Correction == nil || *imgItems[0].Correction != "FalsePositive" {
		return fail("素材 Correction 字段不为 FalsePositive")
	}

	return passf("标记误报成功, FalsePositive=%d", item.Progress.CompletedDetail.DetectedDetail.FalsePositive)
}

func testMarkDeletedFp() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}
	if correctionImageID == "" {
		return skip("没有可用于矫正测试的素材")
	}

	// 标记为 DeletedFp
	resp := doJSON("PUT", taskAPIPrefix+"/"+imageTaskID+"/images/"+correctionImageID, map[string]interface{}{
		"Correction": "DeletedFp",
	})
	if resp.ErrorCode != 0 {
		return failf("删除误报失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 默认列表不应包含 DeletedFp 素材
	listResp := doJSON("GET", taskAPIPrefix+"/"+imageTaskID+"/images?Page=1&PageSize=100", nil)
	var pageData common.PageData
	json.Unmarshal(listResp.Data, &pageData)

	itemsJSON, _ := json.Marshal(pageData.Items)
	var items []model.ImageItem
	json.Unmarshal(itemsJSON, &items)

	for _, item := range items {
		if item.Id == correctionImageID {
			return fail("DeletedFp 素材仍出现在默认列表中")
		}
	}

	// 通过 Correction=DeletedFp 筛选可以查到
	fpResp := doJSON("GET", taskAPIPrefix+"/"+imageTaskID+"/images?Correction=DeletedFp&Page=1&PageSize=100", nil)
	var fpPage common.PageData
	json.Unmarshal(fpResp.Data, &fpPage)

	if fpPage.Total == 0 {
		return fail("通过 Correction=DeletedFp 筛选不到已删除误报素材")
	}

	return passf("删除误报成功, 默认列表已过滤, DeletedFp 筛选 Total=%d", fpPage.Total)
}

func testRestoreCorrection() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}
	if correctionImageID == "" {
		return skip("没有可用于矫正测试的素材")
	}

	// 恢复正常（Correction = null）
	resp := doJSON("PUT", taskAPIPrefix+"/"+imageTaskID+"/images/"+correctionImageID, map[string]interface{}{
		"Correction": nil,
	})
	if resp.ErrorCode != 0 {
		return failf("恢复矫正失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 验证素材恢复出现在默认列表中
	imgResp := doJSON("GET", taskAPIPrefix+"/"+imageTaskID+"/images?ImageId="+correctionImageID, nil)
	var imgPage common.PageData
	json.Unmarshal(imgResp.Data, &imgPage)
	imgItemsJSON, _ := json.Marshal(imgPage.Items)
	var imgItems []model.ImageItem
	json.Unmarshal(imgItemsJSON, &imgItems)

	if len(imgItems) == 0 {
		return fail("恢复后素材仍不在默认列表中")
	}
	if imgItems[0].Correction != nil {
		return failf("恢复后 Correction 不为 null: %s", *imgItems[0].Correction)
	}

	// 验证 Progress 恢复
	taskResp := doJSON("GET", taskAPIPrefix+"?Id="+imageTaskID, nil)
	item := parseFirstTaskItem(taskResp.Data)
	if item == nil {
		return fail("解析任务项失败")
	}

	if item.Progress.CompletedDetail.DetectedDetail.TruePositive == 0 {
		return fail("恢复后 TruePositive 为 0")
	}

	return passf("恢复成功, Correction=null, TruePositive=%d", item.Progress.CompletedDetail.DetectedDetail.TruePositive)
}
```

- [ ] **Step 4: 编译验证**

Run: `cd c:/Users/李陈/Desktop/大模型测试/server && go build ./...`
Expected: 编译成功

- [ ] **Step 5: 提交**

```bash
git add server/test/integration/task.go
git commit -m "test(task-results): 新增任务结果领域集成测试用例"
```

---

### Task 6: 端到端验证

- [ ] **Step 1: 启动服务器**

Run: `cd c:/Users/李陈/Desktop/大模型测试/server && go run cmd/server/main.go`
Expected: 服务器正常启动，日志显示路由注册成功

- [ ] **Step 2: 手动测试 GET /api/tasks/:id/images**

使用 curl 或浏览器访问一个已存在任务的素材列表，验证分页返回正常。

Run: `curl -s "http://localhost:8080/api/tasks?Page=1&PageSize=1" | head -c 200`
Expected: 返回任务列表 JSON

- [ ] **Step 3: 运行集成测试**

Run: `cd c:/Users/李陈/Desktop/大模型测试/server && go run test/integration/main.go`
Expected: 所有测试用例通过

- [ ] **Step 4: 最终提交（如有修复）**

如果集成测试中发现问题，修复后提交。
