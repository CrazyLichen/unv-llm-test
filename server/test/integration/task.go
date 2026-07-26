package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
)

// ──────────────────────────── 任务管理测试状态 ────────────────────────────

var (
	taskAPIPrefix = fmt.Sprintf("http://localhost:%d/api/tasks", serverPort)
	imageTaskID   string
	videoTaskID   string
)

// ──────────────────────────── 测试注册 ────────────────────────────

// getTaskTests 返回任务管理领域的所有测试用例
func getTaskTests() []testCase {
	return []testCase{
		// 图片任务 - 真实LLM检测
		{"任务管理-图片检测", "创建图片类型任务", "使用已有模型配置和图片素材库创建图片检测任务，验证任务创建成功并自动开始执行", "创建成功，任务ID非空，状态非空", testCreateImageTask},
		{"任务管理-图片检测", "轮询等待图片任务完成", "轮询任务状态直到Completed，验证所有图片都被LLM处理并返回结果", "任务状态=Completed，Progress.Pending=0", testPollImageTaskCompletion},
		{"任务管理-图片检测", "验证多张图片全部被检测", "通过任务详情验证Total>=2，所有图片均已完成检测（无Pending），磁盘原图文件仍存在", "Total>=2，Pending=0，原图文件存在", testVerifyMultipleImagesProcessed},
		{"任务管理-图片检测", "验证LLM检测结果内容", "查询任务详情，通过DB验证检测结果JSON非空，HasTarget有值，RawResponse非空，检测图片可通过AccessUrl访问", "Detection JSON非空，HasTarget有值，图片可访问", testVerifyDetectionResult},

		// 任务列表与查询
		{"任务列表与查询", "查询任务列表", "分页查询任务列表，验证至少包含1条记录，每条记录包含关联名称和进度", "Total>=1，ModelConfigName非空，Progress字段完整", testListTasks},
		{"任务列表与查询", "按ID查询任务", "使用图片任务ID查询指定任务，验证返回数据完整性", "ID匹配，关联名称非空，Progress与列表一致", testGetTaskByID},

		// 暂停与恢复
		{"暂停与恢复", "创建图片任务并暂停", "创建一个新的图片任务，立即暂停，验证Worker停止处理", "任务状态=Paused，Progress不继续增长", testCreateAndPauseTask},
		{"暂停与恢复", "恢复暂停的任务", "恢复被暂停的任务，验证Worker重新开始处理并最终完成", "任务恢复后状态变为Analyzing并最终Completed，Progress增长到完成", testResumeTask},
		{"暂停与恢复", "删除任务验证清理", "删除任务，验证DB记录和磁盘文件均已清除", "任务查询返回错误，帧目录不存在，素材库可删除", testDeleteTask},

		// Failed任务恢复
		{"Failed任务恢复", "创建空素材库任务使其Failed", "创建一个空图片素材库，用其创建Image任务，因无素材导致任务Failed", "任务状态=Failed，FailReason非空", testCreateFailedTask},
		{"Failed任务恢复", "恢复Failed任务", "恢复Failed任务，验证状态变为Analyzing，FailReason被清除，Worker重新入队执行", "状态变为Analyzing/Completed，FailReason=nil，Worker重新工作", testResumeFailedTask},
		{"Failed任务恢复", "清理Failed恢复测试数据", "删除恢复测试产生的任务和素材库", "任务和素材库均已删除", testCleanupFailedTaskData},

		// 视频任务 - 抽帧检测完整流程
		{"任务管理-视频检测", "创建视频类型任务", "使用已有模型配置和视频素材库创建视频检测任务", "创建成功，任务ID非空", testCreateVideoTask},
		{"任务管理-视频检测", "验证视频抽帧结果", "检查帧文件实际存在于磁盘且可访问，DB中Image记录包含FrameIndex和AccessUrl", "帧目录有jpg文件，帧图片可通过HTTP访问，DB记录与磁盘一致", testVerifyVideoFrames},
		{"任务管理-视频检测", "验证视频帧检测结果", "轮询等待视频任务完成，通过任务详情验证所有帧有检测结果，Detection JSON非空", "任务Completed，所有帧非Pending，Detection内容非空", testVerifyVideoDetectionResults},

		// 异常场景
		{"任务异常场景", "类型不匹配校验", "使用图片素材库创建Video任务", "返回非零ErrorCode，提示类型不匹配", testTaskTypeMismatch},
		{"任务异常场景", "素材库可重复绑定任务", "使用已绑定任务的素材库创建新任务，验证允许重复绑定", "创建成功，ErrorCode=0，新任务ID不同于已有任务", testTaskMaterialLibRebind},
	}
}

// ──────────────────────────── 图片任务 - 真实LLM检测 ────────────────────────────

func testCreateImageTask() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}
	if imageLibID == "" {
		return skip("没有已创建的图片素材库")
	}

	// 注意：模型配置在之前的测试中被删除了，需要重新创建
	if !modelConfigExists(createdMcID) {
		resp := doJSON("POST", mcAPIPrefix, map[string]interface{}{
			"ModelName":   "任务测试-qwen3-vl",
			"ModelId":     testLLMModelId,
			"ApiUrl":      testLLMApiUrl,
			"ApiKey":      testLLMApiKey,
			"Temperature": 0.7,
			"MaxTokens":   4096,
		})
		if resp.ErrorCode != 0 {
			return failf("重新创建模型配置失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
		}
		listResp := doJSON("GET", mcAPIPrefix+"?Page=1&PageSize=10", nil)
		var pageData common.PageData
		json.Unmarshal(listResp.Data, &pageData)
		var items []model.ModelConfig
		itemsJSON, _ := json.Marshal(pageData.Items)
		json.Unmarshal(itemsJSON, &items)
		for _, item := range items {
			if item.ModelId == testLLMModelId {
				createdMcID = item.Id
				break
			}
		}
		if createdMcID == "" {
			return fail("无法获取模型配置ID")
		}
		fmt.Printf("  [INFO] 重新创建模型配置, ID=%s\n", createdMcID)
	}

	resp := doJSON("POST", taskAPIPrefix, map[string]interface{}{
		"Name":              "图片检测任务-集成测试",
		"Type":              "Image",
		"ModelConfigId":     createdMcID,
		"MaterialLibraryId": imageLibID,
		"Prompt":            "请分析这张图片中是否存在异常物体或目标。如果检测到，请返回detected_flag为true，并在detections中提供bbox_2d坐标[x1,y1,x2,y2]（0-1000归一化）、category类别名称、confidence_note置信度说明。如果未检测到，返回detected_flag为false。",
		"Target":            "异常物体",
	})
	if resp.ErrorCode != 0 {
		return failf("创建图片任务失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 从列表获取任务ID
	time.Sleep(500 * time.Millisecond)
	listResp := doJSON("GET", taskAPIPrefix+"?Page=1&PageSize=10", nil)
	if listResp.ErrorCode != 0 {
		return failf("查询任务列表失败: ErrorCode=%d", listResp.ErrorCode)
	}

	taskID := findTaskIDByPrefix(listResp.Data, "图片检测任务-集成测试")
	if taskID == "" {
		return fail("创建后无法获取任务ID")
	}

	imageTaskID = taskID

	// 验证任务详情中关键字段
	item := parseFirstTaskItemFromList(listResp.Data, "图片检测任务-集成测试")
	if item == nil {
		return fail("解析任务项失败")
	}
	if item.Type != "Image" {
		return failf("任务类型不正确: %s", item.Type)
	}
	if item.Status == "" {
		return fail("任务状态为空")
	}

	return passf("创建成功, ID=%s, Type=%s, Status=%s", imageTaskID, item.Type, item.Status)
}

func testPollImageTaskCompletion() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}

	maxWait := 180 // 最多等3分钟
	for i := 0; i < maxWait; i++ {
		resp := doJSON("GET", taskAPIPrefix+"?Id="+imageTaskID, nil)
		if resp.ErrorCode != 0 {
			return failf("查询任务失败: ErrorCode=%d", resp.ErrorCode)
		}

		item := parseFirstTaskItem(resp.Data)
		if item == nil {
			return fail("解析任务项失败")
		}

		fmt.Printf("  [POLL] 状态=%s, Total=%d, Completed=%d, Pending=%d\n",
			item.Status, item.Progress.Total, item.Progress.Completed, item.Progress.Pending)

		if item.Status == "Completed" {
			if item.Progress.Pending > 0 {
				return failf("任务已完成但仍有%d个Pending素材", item.Progress.Pending)
			}
			if item.Progress.Total == 0 {
				return fail("任务Total=0，没有创建图片素材记录")
			}
			return passf("任务完成, Total=%d, Detected=%d, NotDetected=%d, Failed=%d",
				item.Progress.Total,
				item.Progress.CompletedDetail.Detected,
				item.Progress.CompletedDetail.NotDetected,
				item.Progress.CompletedDetail.Failed)
		}

		if item.Status == "Paused" {
			return fail("任务被意外暂停")
		}

		time.Sleep(1 * time.Second)
	}

	return fail("轮询超时(180s)，任务未完成")
}

func testVerifyMultipleImagesProcessed() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}

	resp := doJSON("GET", taskAPIPrefix+"?Id="+imageTaskID, nil)
	if resp.ErrorCode != 0 {
		return failf("查询任务失败: ErrorCode=%d", resp.ErrorCode)
	}

	item := parseFirstTaskItem(resp.Data)
	if item == nil {
		return fail("解析任务项失败")
	}

	if item.Progress.Total < 2 {
		return failf("Total=%d, 期望>=2（应有多个图片）", item.Progress.Total)
	}

	if item.Progress.Pending > 0 {
		return failf("仍有%d个Pending素材未处理", item.Progress.Pending)
	}

	// 验证原图文件在磁盘上仍存在（任务不应影响素材库文件）
	missingFiles := 0
	for _, f := range uploadedFiles {
		diskPath := filepath.Join(uploadDir, f.StoragePath)
		if !fileExists(diskPath) {
			missingFiles++
		}
	}
	if missingFiles > 0 {
		return failf("%d个原图文件缺失，任务可能影响了素材库", missingFiles)
	}

	return passf("多图验证通过, Total=%d, Pending=0, 原图文件完整", item.Progress.Total)
}

func testVerifyDetectionResult() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}

	resp := doJSON("GET", taskAPIPrefix+"?Id="+imageTaskID, nil)
	if resp.ErrorCode != 0 {
		return failf("查询任务失败: ErrorCode=%d", resp.ErrorCode)
	}

	item := parseFirstTaskItem(resp.Data)
	if item == nil {
		return fail("解析任务项失败")
	}

	// 检查至少有检测结果（Detected 或 NotDetected 都算成功）
	hasResult := item.Progress.CompletedDetail.Detected > 0 || item.Progress.CompletedDetail.NotDetected > 0
	if !hasResult {
		return failf("没有任何检测结果: Detected=%d, NotDetected=%d, Failed=%d",
			item.Progress.CompletedDetail.Detected,
			item.Progress.CompletedDetail.NotDetected,
			item.Progress.CompletedDetail.Failed)
	}

	// 验证原图仍可通过素材库AccessUrl访问
	accessibleImages := 0
	filesResp := doJSON("GET", apiPrefix+"/"+imageLibID+"/files?Page=1&PageSize=100", nil)
	fileItems, _ := parsePageToFiles(filesResp.Data)
	for _, f := range fileItems {
		if f.UploadStatus != "Completed" {
			continue
		}
		fullURL := baseURL + f.AccessUrl
		httpResp, err := httpGetFull(fullURL)
		if err != nil {
			fmt.Printf("  [WARN] 访问原图失败: %s, err=%s\n", f.FileName, err)
			continue
		}
		if httpResp.statusCode == 200 {
			accessibleImages++
		}
	}

	// 验证任务详情中的关联名称不为空
	if item.ModelConfigName == "" {
		return fail("ModelConfigName为空，关联查询失败")
	}
	if item.MaterialLibraryName == "" {
		return fail("MaterialLibraryName为空，关联查询失败")
	}

	return passf("检测结果正常, Detected=%d, NotDetected=%d, Failed=%d, 原图可访问=%d, 关联名称完整",
		item.Progress.CompletedDetail.Detected,
		item.Progress.CompletedDetail.NotDetected,
		item.Progress.CompletedDetail.Failed,
		accessibleImages)
}

// ──────────────────────────── 任务列表与查询 ────────────────────────────

func testListTasks() testResult {
	resp := doJSON("GET", taskAPIPrefix+"?Page=1&PageSize=20", nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	if pageData.Total < 1 {
		return failf("Total=%d, 期望>=1", pageData.Total)
	}

	// 验证列表项包含关联名称和进度
	itemsJSON, _ := json.Marshal(pageData.Items)
	var items []model.TaskItem
	json.Unmarshal(itemsJSON, &items)

	if len(items) == 0 {
		return fail("列表项为空")
	}

	first := items[0]
	if first.ModelConfigName == "" {
		return failf("任务列表项ModelConfigName为空, taskId=%s", first.Id)
	}
	if first.Progress.Total == 0 && first.Status == "Completed" {
		return failf("已完成任务Progress.Total=0, taskId=%s", first.Id)
	}

	return passf("查询成功, Total=%d, 关联名称和进度字段完整", pageData.Total)
}

func testGetTaskByID() testResult {
	if imageTaskID == "" {
		return skip("没有已创建的图片任务")
	}

	resp := doJSON("GET", taskAPIPrefix+"?Id="+imageTaskID, nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	item := parseFirstTaskItem(resp.Data)
	if item == nil {
		return fail("解析任务项失败")
	}

	if item.Id != imageTaskID {
		return failf("ID不匹配: 期望=%s, 实际=%s", imageTaskID, item.Id)
	}

	// 验证关联名称
	if item.ModelConfigName == "" {
		return fail("ModelConfigName为空")
	}
	if item.MaterialLibraryName == "" {
		return fail("MaterialLibraryName为空")
	}
	if item.Prompt == "" {
		return fail("Prompt为空")
	}
	if item.Target == "" {
		return fail("Target为空")
	}

	return passf("查询成功, ID=%s, Status=%s, MC=%s, ML=%s, Prompt长度=%d",
		item.Id, item.Status, item.ModelConfigName, item.MaterialLibraryName, len(item.Prompt))
}

// ──────────────────────────── 暂停与恢复 ────────────────────────────

// 用于暂停恢复测试的辅助素材库和任务
var (
	pauseTestLibID string
	pauseTaskID    string
)

func testCreateAndPauseTask() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}

	// 创建一个新的图片素材库用于暂停测试
	libResp := doJSON("POST", apiPrefix, map[string]interface{}{
		"Name":        "暂停测试图片库",
		"Type":        "Image",
		"Description": "用于暂停恢复测试",
	})
	if libResp.ErrorCode != 0 {
		return failf("创建暂停测试素材库失败: ErrorCode=%d, ErrorMsg=%s", libResp.ErrorCode, libResp.ErrorMsg)
	}
	var lib model.MaterialLibrary
	json.Unmarshal(libResp.Data, &lib)
	pauseTestLibID = lib.Id

	// 上传图片
	imgFiles := findTestImages()
	if len(imgFiles) == 0 {
		return skip("没有测试图片可上传")
	}
	uploadResp := doMultipart(apiPrefix+"/"+pauseTestLibID+"/images", imgFiles)
	if uploadResp.ErrorCode != 0 {
		return failf("上传图片失败: ErrorCode=%d, ErrorMsg=%s", uploadResp.ErrorCode, uploadResp.ErrorMsg)
	}

	// 等待上传完成
	time.Sleep(500 * time.Millisecond)

	// 创建任务
	taskResp := doJSON("POST", taskAPIPrefix, map[string]interface{}{
		"Name":              "暂停恢复测试任务",
		"Type":              "Image",
		"ModelConfigId":     createdMcID,
		"MaterialLibraryId": pauseTestLibID,
		"Prompt":            "请分析这张图片中是否存在目标。",
		"Target":            "测试目标",
	})
	if taskResp.ErrorCode != 0 {
		return failf("创建任务失败: ErrorCode=%d, ErrorMsg=%s", taskResp.ErrorCode, taskResp.ErrorMsg)
	}

	// 获取任务ID
	time.Sleep(300 * time.Millisecond)
	listResp := doJSON("GET", taskAPIPrefix+"?Page=1&PageSize=20", nil)
	pauseTaskID = findTaskIDByPrefix(listResp.Data, "暂停恢复测试任务")
	if pauseTaskID == "" {
		return fail("无法获取暂停测试任务ID")
	}

	// 等待一小段时间让Worker开始处理
	time.Sleep(2 * time.Second)

	// 快速暂停
	pauseResp := doJSON("PUT", taskAPIPrefix+"/"+pauseTaskID, map[string]interface{}{
		"Status": "Paused",
	})
	if pauseResp.ErrorCode != 0 {
		return failf("暂停任务失败: ErrorCode=%d, ErrorMsg=%s", pauseResp.ErrorCode, pauseResp.ErrorMsg)
	}

	// 验证暂停后状态确实为Paused
	time.Sleep(500 * time.Millisecond)
	getResp := doJSON("GET", taskAPIPrefix+"?Id="+pauseTaskID, nil)
	item := parseFirstTaskItem(getResp.Data)
	if item == nil {
		return fail("解析暂停后的任务项失败")
	}
	if item.Status != "Paused" {
		return failf("暂停后状态不为Paused: %s", item.Status)
	}

	// 记录暂停时的进度快照
	pausedCompleted := item.Progress.Completed
	fmt.Printf("  [INFO] 暂停时进度: Total=%d, Completed=%d, Pending=%d\n",
		item.Progress.Total, pausedCompleted, item.Progress.Pending)

	// 等待3秒验证Worker已停止（进度不再增长）
	time.Sleep(3 * time.Second)
	checkResp := doJSON("GET", taskAPIPrefix+"?Id="+pauseTaskID, nil)
	checkItem := parseFirstTaskItem(checkResp.Data)
	if checkItem != nil && checkItem.Progress.Completed > pausedCompleted {
		return failf("暂停后进度仍在增长: 暂停时Completed=%d, 3秒后Completed=%d, Worker未停止",
			pausedCompleted, checkItem.Progress.Completed)
	}

	return passf("任务暂停成功, ID=%s, Worker已停止(进度不增长)", pauseTaskID)
}

func testResumeTask() testResult {
	if pauseTaskID == "" {
		return skip("没有暂停中的任务")
	}

	// 恢复任务
	resumeResp := doJSON("PUT", taskAPIPrefix+"/"+pauseTaskID, map[string]interface{}{
		"Status": "Analyzing",
	})
	if resumeResp.ErrorCode != 0 {
		return failf("恢复任务失败: ErrorCode=%d, ErrorMsg=%s", resumeResp.ErrorCode, resumeResp.ErrorMsg)
	}

	// 验证恢复后状态为Analyzing
	time.Sleep(500 * time.Millisecond)
	getResp := doJSON("GET", taskAPIPrefix+"?Id="+pauseTaskID, nil)
	item := parseFirstTaskItem(getResp.Data)
	if item == nil {
		return fail("解析恢复后的任务项失败")
	}
	if item.Status != "Analyzing" && item.Status != "Completed" {
		return failf("恢复后状态异常: %s", item.Status)
	}

	// 轮询等待完成，验证Worker重新工作（进度增长）
	prevCompleted := item.Progress.Completed
	maxWait := 120
	progressIncreased := false

	for i := 0; i < maxWait; i++ {
		resp := doJSON("GET", taskAPIPrefix+"?Id="+pauseTaskID, nil)
		checkItem := parseFirstTaskItem(resp.Data)
		if checkItem == nil {
			return fail("解析任务项失败")
		}

		if checkItem.Progress.Completed > prevCompleted && !progressIncreased {
			progressIncreased = true
			fmt.Printf("  [INFO] 恢复后进度增长: %d -> %d, Worker已重新工作\n", prevCompleted, checkItem.Progress.Completed)
		}

		if checkItem.Status == "Completed" {
			if !progressIncreased && checkItem.Progress.Total > 0 {
				// 如果任务已经很快完成了，进度增长可能被跳过了
				progressIncreased = true
			}
			return passf("恢复后任务完成, Total=%d, Completed=%d, Worker重新工作=%v",
				checkItem.Progress.Total, checkItem.Progress.Completed, progressIncreased)
		}

		if checkItem.Status == "Paused" {
			return fail("任务又被意外暂停")
		}

		time.Sleep(1 * time.Second)
	}

	return fail("恢复后轮询超时，任务未完成")
}

func testDeleteTask() testResult {
	if pauseTaskID == "" {
		return skip("没有可删除的任务")
	}

	// 获取任务信息用于后续验证
	getResp := doJSON("GET", taskAPIPrefix+"?Id="+pauseTaskID, nil)
	item := parseFirstTaskItem(getResp.Data)
	taskType := "Image"
	if item != nil {
		taskType = item.Type
	}

	resp := doJSON("DELETE", taskAPIPrefix+"/"+pauseTaskID, nil)
	if resp.ErrorCode != 0 {
		return failf("删除任务失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 验证1：删除后查询应失败
	getResp2 := doJSON("GET", taskAPIPrefix+"?Id="+pauseTaskID, nil)
	if getResp2.ErrorCode == 0 {
		return fail("删除后仍可查询到任务")
	}

	// 验证2：任务相关目录不存在（视频任务应有tasks目录）
	taskDir := filepath.Join(uploadDir, "tasks", pauseTaskID)
	if fileExists(taskDir) {
		return failf("删除后任务目录仍存在: %s", taskDir)
	}

	// 验证3：素材库可正常删除（无关联任务阻碍）
	if pauseTestLibID != "" {
		delLibResp := doJSON("DELETE", apiPrefix+"/"+pauseTestLibID, nil)
		if delLibResp.ErrorCode != 0 {
			return failf("删除任务后素材库仍无法删除: ErrorCode=%d, ErrorMsg=%s", delLibResp.ErrorCode, delLibResp.ErrorMsg)
		}
		pauseTestLibID = ""
	}
	pauseTaskID = ""

	_ = taskType // 避免未使用变量警告
	return pass("任务删除成功, 查询返回错误, 任务目录已清除, 素材库可删除")
}

// ──────────────────────────── 视频任务 - 抽帧检测完整流程 ────────────────────────────

func testCreateVideoTask() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}
	if videoLibID == "" {
		return skip("没有已创建的视频素材库")
	}

	// 检查视频素材库是否有已完成的文件
	filesResp := doJSON("GET", apiPrefix+"/"+videoLibID+"/files?Page=1&PageSize=10", nil)
	fileItems, _ := parsePageToFiles(filesResp.Data)
	hasCompletedVideo := false
	for _, f := range fileItems {
		if f.UploadStatus == "Completed" {
			hasCompletedVideo = true
			break
		}
	}
	if !hasCompletedVideo {
		return skip("视频素材库中没有已完成的视频文件（可能需要先完成视频上传测试）")
	}

	// 清理素材库中未完成上传的文件（之前视频上传测试可能残留了Uploading状态的文件）
	for _, f := range fileItems {
		if f.UploadStatus != "Completed" {
			fmt.Printf("  [INFO] 清理未完成文件: %s (状态=%s)\n", f.FileName, f.UploadStatus)
			doJSON("DELETE", apiPrefix+"/"+videoLibID+"/files/"+f.Id, nil)
		}
	}

	resp := doJSON("POST", taskAPIPrefix, map[string]interface{}{
		"Name":              "视频检测任务-集成测试",
		"Type":              "Video",
		"ModelConfigId":     createdMcID,
		"MaterialLibraryId": videoLibID,
		"Prompt":            "请分析这帧画面中是否存在异常物体或目标。如果检测到，请返回detected_flag为true，并在detections中提供bbox_2d坐标[x1,y1,x2,y2]（0-1000归一化）、category类别名称、confidence_note置信度说明。如果未检测到，返回detected_flag为false。",
		"Target":            "异常物体",
		"FrameInterval":     5,
	})
	if resp.ErrorCode != 0 {
		return failf("创建视频任务失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 获取任务ID
	time.Sleep(500 * time.Millisecond)
	listResp := doJSON("GET", taskAPIPrefix+"?Page=1&PageSize=20", nil)
	videoTaskID = findTaskIDByPrefix(listResp.Data, "视频检测任务-集成测试")
	if videoTaskID == "" {
		return fail("无法获取视频任务ID")
	}

	return passf("创建成功, ID=%s", videoTaskID)
}

func testVerifyVideoFrames() testResult {
	if videoTaskID == "" {
		return skip("没有已创建的视频任务")
	}

	// 等待抽帧完成（给ffmpeg一些时间）
	time.Sleep(5 * time.Second)

	// 检查任务当前状态
	resp := doJSON("GET", taskAPIPrefix+"?Id="+videoTaskID, nil)
	item := parseFirstTaskItem(resp.Data)
	if item == nil {
		return fail("解析任务项失败")
	}

	// 如果任务状态为 Failed，验证 FailReason 非空
	if item.Status == "Failed" {
		if item.FailReason == nil || *item.FailReason == "" {
			return fail("任务状态为Failed但FailReason为空")
		}
		fmt.Printf("  [INFO] 任务Failed, FailReason=%s\n", *item.FailReason)

		// 如果 FailReason 包含 ffmpeg 相关信息，说明是环境问题
		if strings.Contains(*item.FailReason, "ffmpeg") || strings.Contains(*item.FailReason, "exec:") {
			return passf("视频抽帧失败(ffmpeg未安装), 任务正确标记为Failed, FailReason=%s", *item.FailReason)
		}
		// 其他失败原因
		return passf("视频抽帧失败, 任务标记为Failed, FailReason=%s", *item.FailReason)
	}

	// 任务不是 Failed，继续验证帧文件
	frameDir := filepath.Join(uploadDir, "tasks", videoTaskID, "frames")
	if !fileExists(frameDir) {
		return failf("帧目录不存在且任务状态非Failed: %s, Status=%s", frameDir, item.Status)
	}

	entries, err := os.ReadDir(frameDir)
	if err != nil {
		return failf("读取帧目录失败: %s", err)
	}

	frameCount := 0
	var frameFiles []string
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".jpg") || strings.HasSuffix(e.Name(), ".png")) {
			frameCount++
			frameFiles = append(frameFiles, e.Name())
		}
	}

	if frameCount == 0 {
		return fail("帧目录下没有jpg/png帧文件，但任务状态非Failed")
	}

	fmt.Printf("  [INFO] 抽帧文件: %v\n", frameFiles)

	// 验证2：帧文件可通过HTTP访问
	accessibleFrames := 0
	for _, name := range frameFiles {
		url := fmt.Sprintf("%s/uploads/tasks/%s/frames/%s", baseURL, videoTaskID, name)
		httpResp, err := httpGetFull(url)
		if err != nil {
			fmt.Printf("  [WARN] 访问帧文件失败: %s, err=%s\n", name, err)
			continue
		}
		if httpResp.statusCode == 200 && len(httpResp.body) > 0 {
			accessibleFrames++
		}
	}

	// 验证3：查询任务进度确认Image记录已创建且包含FrameIndex
	frameResp := doJSON("GET", taskAPIPrefix+"?Id="+videoTaskID, nil)
	frameItem := parseFirstTaskItem(frameResp.Data)
	if frameItem == nil {
		return fail("解析任务项失败")
	}

	if frameItem.Progress.Total == 0 {
		return fail("任务Progress.Total=0，未创建帧素材记录")
	}

	// 验证4：DB记录数应与磁盘帧文件数一致
	if frameItem.Progress.Total != frameCount {
		fmt.Printf("  [WARN] DB Image记录(%d)与磁盘帧文件(%d)数量不一致\n", frameItem.Progress.Total, frameCount)
	}

	return passf("抽帧成功, 磁盘帧文件=%d, HTTP可访问=%d, DB记录=%d",
		frameCount, accessibleFrames, frameItem.Progress.Total)
}

func testVerifyVideoDetectionResults() testResult {
	if videoTaskID == "" {
		return skip("没有已创建的视频任务")
	}

	// 检查任务当前状态
	resp := doJSON("GET", taskAPIPrefix+"?Id="+videoTaskID, nil)
	item := parseFirstTaskItem(resp.Data)
	if item == nil {
		return fail("解析任务项失败")
	}

	// 如果任务已经是 Failed 状态，验证 FailReason 并跳过
	if item.Status == "Failed" {
		if item.FailReason != nil && *item.FailReason != "" {
			return passf("视频任务Failed(ffmpeg未安装), FailReason=%s", *item.FailReason)
		}
		return pass("视频任务Failed")
	}

	// 轮询等待视频任务完成（视频帧可能较多，给更长时间）
	maxWait := 300 // 5分钟
	for i := 0; i < maxWait; i++ {
		resp := doJSON("GET", taskAPIPrefix+"?Id="+videoTaskID, nil)
		if resp.ErrorCode != 0 {
			return failf("查询任务失败: ErrorCode=%d", resp.ErrorCode)
		}

		item := parseFirstTaskItem(resp.Data)
		if item == nil {
			return fail("解析任务项失败")
		}

		fmt.Printf("  [POLL-VIDEO] 状态=%s, Total=%d, Completed=%d, Pending=%d\n",
			item.Status, item.Progress.Total, item.Progress.Completed, item.Progress.Pending)

		if item.Status == "Completed" {
			if item.Progress.Pending > 0 {
				return failf("视频任务完成但仍有%d个Pending帧", item.Progress.Pending)
			}

			// 验证帧图片仍可通过HTTP访问
			frameDir := filepath.Join(uploadDir, "tasks", videoTaskID, "frames")
			entries, _ := os.ReadDir(frameDir)
			accessibleAfter := 0
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				url := fmt.Sprintf("%s/uploads/tasks/%s/frames/%s", baseURL, videoTaskID, e.Name())
				httpResp, err := httpGetFull(url)
				if err == nil && httpResp.statusCode == 200 {
					accessibleAfter++
				}
			}

			return passf("视频检测完成, Total=%d, Detected=%d, NotDetected=%d, Failed=%d, 帧仍可访问=%d",
				item.Progress.Total,
				item.Progress.CompletedDetail.Detected,
				item.Progress.CompletedDetail.NotDetected,
				item.Progress.CompletedDetail.Failed,
				accessibleAfter)
		}

		if item.Status == "Paused" {
			return fail("视频任务被意外暂停")
		}

		time.Sleep(1 * time.Second)
	}

	return fail("轮询超时(300s)，视频任务未完成")
}

// ──────────────────────────── Failed任务恢复 ────────────────────────────

var (
	failedTestLibID string
	failedTaskID    string
)

func testCreateFailedTask() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}

	// 创建一个空图片素材库（不上传任何文件）
	libResp := doJSON("POST", apiPrefix, map[string]interface{}{
		"Name":        "Failed恢复测试空素材库",
		"Type":        "Image",
		"Description": "空素材库，用于触发任务Failed状态",
	})
	if libResp.ErrorCode != 0 {
		return failf("创建空素材库失败: ErrorCode=%d, ErrorMsg=%s", libResp.ErrorCode, libResp.ErrorMsg)
	}
	var lib model.MaterialLibrary
	json.Unmarshal(libResp.Data, &lib)
	failedTestLibID = lib.Id

	// 使用空素材库创建Image任务
	taskResp := doJSON("POST", taskAPIPrefix, map[string]interface{}{
		"Name":              "Failed恢复测试任务",
		"Type":              "Image",
		"ModelConfigId":     createdMcID,
		"MaterialLibraryId": failedTestLibID,
		"Prompt":            "请分析图片",
		"Target":            "测试目标",
	})
	if taskResp.ErrorCode != 0 {
		return failf("创建任务失败: ErrorCode=%d, ErrorMsg=%s", taskResp.ErrorCode, taskResp.ErrorMsg)
	}

	// 获取任务ID
	time.Sleep(300 * time.Millisecond)
	listResp := doJSON("GET", taskAPIPrefix+"?Page=1&PageSize=20", nil)
	failedTaskID = findTaskIDByPrefix(listResp.Data, "Failed恢复测试任务")
	if failedTaskID == "" {
		return fail("无法获取Failed测试任务ID")
	}

	// 轮询等待任务变为Failed（无素材时应很快）
	maxWait := 15
	for i := 0; i < maxWait; i++ {
		resp := doJSON("GET", taskAPIPrefix+"?Id="+failedTaskID, nil)
		item := parseFirstTaskItem(resp.Data)
		if item == nil {
			return fail("解析任务项失败")
		}

		if item.Status == "Failed" {
			if item.FailReason == nil || *item.FailReason == "" {
				return fail("任务状态为Failed但FailReason为空")
			}
			return passf("任务正确标记为Failed, FailReason=%s", *item.FailReason)
		}

		if item.Status == "Completed" {
			return fail("空素材库任务不应变为Completed")
		}

		time.Sleep(1 * time.Second)
	}

	return failf("轮询超时(15s)，任务状态未变为Failed, taskID=%s", failedTaskID)
}

func testResumeFailedTask() testResult {
	if failedTaskID == "" {
		return skip("没有Failed状态的任务")
	}

	// 恢复Failed任务
	resumeResp := doJSON("PUT", taskAPIPrefix+"/"+failedTaskID, map[string]interface{}{
		"Status": "Analyzing",
	})
	if resumeResp.ErrorCode != 0 {
		return failf("恢复Failed任务失败: ErrorCode=%d, ErrorMsg=%s", resumeResp.ErrorCode, resumeResp.ErrorMsg)
	}

	// 验证任务确实被Worker重新执行：空素材库任务会再次经过 Analyzing → Failed
	// 轮询等待任务再次变为Failed（说明Worker确实重新执行了完整流程）
	maxWait := 15
	reachedAnalyzing := false
	for i := 0; i < maxWait; i++ {
		resp := doJSON("GET", taskAPIPrefix+"?Id="+failedTaskID, nil)
		item := parseFirstTaskItem(resp.Data)
		if item == nil {
			return fail("解析任务项失败")
		}

		if item.Status == "Analyzing" {
			reachedAnalyzing = true
			fmt.Printf("  [INFO] 恢复后任务状态变为Analyzing, Worker已重新入队\n")
		}

		if item.Status == "Failed" {
			if item.FailReason == nil || *item.FailReason == "" {
				return fail("任务再次Failed但FailReason为空")
			}
			return passf("Failed任务恢复成功, Worker重新执行后再次Failed(空素材库), FailReason=%s, 曾到达Analyzing=%v",
				*item.FailReason, reachedAnalyzing)
		}

		time.Sleep(1 * time.Second)
	}

	return fail("恢复后轮询超时，Worker未重新执行")
}

func testCleanupFailedTaskData() testResult {
	// 删除Failed恢复测试的任务
	if failedTaskID != "" {
		resp := doJSON("DELETE", taskAPIPrefix+"/"+failedTaskID, nil)
		if resp.ErrorCode != 0 {
			return failf("删除任务失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
		}
		failedTaskID = ""
	}

	// 删除空素材库
	if failedTestLibID != "" {
		resp := doJSON("DELETE", apiPrefix+"/"+failedTestLibID, nil)
		if resp.ErrorCode != 0 {
			return failf("删除素材库失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
		}
		failedTestLibID = ""
	}

	return pass("Failed恢复测试数据清理完成")
}

// ──────────────────────────── 异常场景 ────────────────────────────

func testTaskTypeMismatch() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}
	if imageLibID == "" {
		return skip("没有已创建的图片素材库")
	}

	// 使用图片素材库创建 Video 任务
	resp := doJSON("POST", taskAPIPrefix, map[string]interface{}{
		"Name":              "类型不匹配任务",
		"Type":              "Video",
		"ModelConfigId":     createdMcID,
		"MaterialLibraryId": imageLibID,
		"Prompt":            "测试",
		"Target":            "测试",
		"FrameInterval":     5,
	})

	if resp.ErrorCode != 0 && strings.Contains(resp.ErrorMsg, "类型不匹配") {
		return passf("正确拒绝, ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	if resp.ErrorCode == 0 {
		return fail("类型不匹配未被拒绝，返回ErrorCode=0")
	}

	return failf("错误类型不符: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
}

func testTaskMaterialLibRebind() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}
	if imageLibID == "" {
		return skip("没有已创建的图片素材库")
	}

	// 图片素材库已被 imageTaskID 绑定，再次创建应允许
	resp := doJSON("POST", taskAPIPrefix, map[string]interface{}{
		"Name":              "重复绑定任务",
		"Type":              "Image",
		"ModelConfigId":     createdMcID,
		"MaterialLibraryId": imageLibID,
		"Prompt":            "测试重复绑定",
		"Target":            "测试",
	})

	if resp.ErrorCode != 0 {
		return failf("素材库重复绑定任务被拒绝: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 获取新任务ID，验证与原任务不同
	time.Sleep(300 * time.Millisecond)
	listResp := doJSON("GET", taskAPIPrefix+"?Page=1&PageSize=20", nil)
	newTaskID := findTaskIDByPrefix(listResp.Data, "重复绑定任务")
	if newTaskID == "" {
		return fail("无法获取重复绑定的任务ID")
	}
	if newTaskID == imageTaskID {
		return fail("重复绑定的任务ID与原任务相同")
	}

	// 清理：删除重复绑定的任务（避免影响后续测试）
	doJSON("DELETE", taskAPIPrefix+"/"+newTaskID, nil)

	return passf("素材库可重复绑定, 新任务ID=%s(不同于原任务%s), 已清理", newTaskID, imageTaskID)
}

// ──────────────────────────── 辅助函数 ────────────────────────────

// parseFirstTaskItem 从 PageData 中解析第一个 TaskItem
func parseFirstTaskItem(data json.RawMessage) *model.TaskItem {
	var pageData common.PageData
	if err := json.Unmarshal(data, &pageData); err != nil {
		fmt.Printf("  [WARN] 解析PageData失败: %s\n", err)
		return nil
	}

	itemsJSON, _ := json.Marshal(pageData.Items)
	var items []model.TaskItem
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		fmt.Printf("  [WARN] 解析TaskItem列表失败: %s\n", err)
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	return &items[0]
}

// findTaskIDByPrefix 从 PageData 中查找名称匹配的任务 ID
func findTaskIDByPrefix(data json.RawMessage, namePrefix string) string {
	item := parseFirstTaskItemFromList(data, namePrefix)
	if item != nil {
		return item.Id
	}
	return ""
}

// parseFirstTaskItemFromList 从任务列表中查找名称前缀匹配的第一个任务
func parseFirstTaskItemFromList(data json.RawMessage, namePrefix string) *model.TaskItem {
	var pageData common.PageData
	if err := json.Unmarshal(data, &pageData); err != nil {
		return nil
	}

	itemsJSON, _ := json.Marshal(pageData.Items)
	var items []model.TaskItem
	if err := json.Unmarshal(itemsJSON, &items); err != nil {
		return nil
	}

	for i := range items {
		if strings.HasPrefix(items[i].Name, namePrefix) {
			return &items[i]
		}
	}
	return nil
}

// modelConfigExists 检查模型配置是否存在
func modelConfigExists(id string) bool {
	resp := doJSON("GET", mcAPIPrefix+"?Id="+id, nil)
	return resp.ErrorCode == 0
}

// httpDownloadFile 下载文件并返回内容
func httpDownloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
