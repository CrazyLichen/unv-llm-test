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

// QueueTaskItem 队列中的任务项
type QueueTaskItem struct {
	// TaskID 任务 ID
	TaskID string
	// Ctx 任务 context（由生产者创建和控制）
	Ctx context.Context
}

// Scheduler 任务调度器
type Scheduler struct {
	// taskQueue 任务队列
	taskQueue chan *QueueTaskItem
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
		taskQueue:    make(chan *QueueTaskItem, 100),
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
	s.taskQueue <- &QueueTaskItem{TaskID: taskID, Ctx: ctx}
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
func (s *Scheduler) executeTask(item *QueueTaskItem) {
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

	// 更新状态为 Analyzing（清除可能存在的 FailReason）
	if err := s.repo.UpdateStatus(item.Ctx, item.TaskID, "Analyzing"); err != nil {
		slog.Error("更新任务状态失败", "taskId", item.TaskID, "error", err)
		return
	}

	// 视频任务：先抽帧
	if task.Type == "Video" {
		if err := s.extractFrames(item.Ctx, task); err != nil {
			slog.Error("视频抽帧失败", "taskId", task.Id, "error", err)
			// 抽帧失败，检查是否有帧可用
			pendingImages, _ := s.repo.ListPendingImages(item.Ctx, task.Id)
			if len(pendingImages) == 0 {
				// 没有任何帧可用，任务标记为 Failed
				failReason := "视频抽帧失败: " + err.Error()
				s.repo.UpdateStatusWithReason(item.Ctx, task.Id, "Failed", failReason)
				slog.Info("任务失败（无可用帧）", "taskId", task.Id, "reason", failReason)
				return
			}
			// 有部分帧可用，继续检测
		}
	}

	// 检查是否有待检测素材（图片任务在 Create 时已创建，视频任务在 extractFrames 中创建）
	pendingImages, _ := s.repo.ListPendingImages(item.Ctx, task.Id)
	if len(pendingImages) == 0 {
		failReason := "无待检测素材"
		s.repo.UpdateStatusWithReason(item.Ctx, task.Id, "Failed", failReason)
		slog.Info("任务失败（无待检测素材）", "taskId", task.Id)
		return
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
func (s *Scheduler) detectImages(item *QueueTaskItem, task *model.Task) {
	// 加载模型配置
	mc, err := s.mcRepo.GetByID(item.Ctx, task.ModelConfigId)
	if err != nil || mc == nil {
		slog.Error("加载模型配置失败", "taskId", task.Id, "modelConfigId", task.ModelConfigId, "error", err)
		failReason := "模型配置不存在或加载失败"
		s.repo.UpdateStatusWithReason(item.Ctx, task.Id, "Failed", failReason)
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
		imageBase64, err := s.loadImageBase64(&img)
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
		}, llm.WithRetry(2), llm.WithTimeoutUs(120000000))

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
func (s *Scheduler) loadImageBase64(img *model.Image) (string, error) {
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

	updates := map[string]interface{}{
		"status": status,
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
		DetectedFlag *bool `json:"detected_flag"`
		Detections   []struct {
			Category       string  `json:"category"`
			Bbox2d         []int32 `json:"bbox_2d"`
			ConfidenceNote string  `json:"confidence_note"`
		} `json:"detections"`
	}

	err := json.Unmarshal([]byte(content), &result)
	if err == nil {
		// JSON 解析成功
		// 判断是否检测到目标：优先使用 detected_flag，若无则根据 detections 数组推断
		detected := len(result.Detections) > 0
		if result.DetectedFlag != nil {
			detected = *result.DetectedFlag
		}

		boxes := make([]model.Box, 0)
		if detected {
			for _, d := range result.Detections {
				box := model.Box{
					Label:      d.Category,
					Confidence: d.ConfidenceNote,
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

	// JSON 解析失败，尝试正则降级
	slog.Warn("LLM 返回 JSON 解析失败，尝试正则降级", "content", content)

	// 正则提取 detected_flag
	flagRe := regexp.MustCompile(`"detected_flag"\s*:\s*(true|false)`)
	flagMatch := flagRe.FindStringSubmatch(content)

	// 正则提取 bbox_2d
	bboxRe := regexp.MustCompile(`"bbox_2d"\s*:\s*\[\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\]`)
	bboxMatches := bboxRe.FindAllStringSubmatch(content, -1)

	// 正则提取 category
	catRe := regexp.MustCompile(`"category"\s*:\s*"([^"]*)"`)
	catMatches := catRe.FindAllStringSubmatch(content, -1)

	// 正则提取 confidence_note
	confRe := regexp.MustCompile(`"confidence_note"\s*:\s*"([^"]*)"`)
	confMatches := confRe.FindAllStringSubmatch(content, -1)

	var detected bool
	if len(flagMatch) >= 2 {
		detected = flagMatch[1] == "true"
	} else {
		// 无 detected_flag 时，根据是否有 detections 推断
		detected = len(bboxMatches) > 0 || len(catMatches) > 0
	}

	boxes := make([]model.Box, 0)
	if detected {
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

	// 正则也无法提取 flag
	slog.Error("无法解析检测结果", "content", content)
	return nil, "Failed", "无法解析检测结果"
}
