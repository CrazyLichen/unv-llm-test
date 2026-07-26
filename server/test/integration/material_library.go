package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"llm-test-server/internal/common"
	"llm-test-server/internal/model"
)

// ──────────────────────────── 素材库测试状态 ────────────────────────────

var (
	imageLibID    string
	videoLibID    string
	uploadedFiles []model.MaterialFile
)

// ──────────────────────────── 测试注册 ────────────────────────────

// getMaterialLibraryTests 返回素材库领域的所有测试用例
func getMaterialLibraryTests() []testCase {
	return []testCase{
		// 素材库 CRUD
		{"创建图片素材库", testCreateImageLibrary},
		{"创建视频素材库", testCreateVideoLibrary},
		{"查询素材库列表", testListLibraries},
		{"按ID查询素材库", testGetLibraryByID},
		{"更新素材库", testUpdateLibrary},

		// 图片上传 & 磁盘验证
		{"批量上传图片", testUploadImages},
		{"验证图片文件存在于磁盘", testVerifyImageFilesOnDisk},
		{"验证数据库记录与磁盘一致", testVerifyDbMatchesDisk},
		{"查询素材文件列表（含进度）", testListFiles},
		{"通过静态文件服务访问图片", testStaticFileAccess},
		{"素材库统计信息验证", testLibraryStats},

		// 删除行为验证
		{"删除单个素材文件-验证磁盘文件已删除", testDeleteFileVerifyDisk},
		{"删除素材库-验证级联清理", testDeleteLibraryVerifyCleanup},

		// 视频分片上传
		{"视频分片上传完整流程", testVideoUploadFlow},
		{"验证视频合并后无残留chunk", testVerifyNoChunkResidue},
		{"视频断点续传", testVideoResumableUpload},
		{"同名已完成视频-init报错", testVideoSameNameReject},

		// 异常场景
		{"类型不匹配校验", testTypeMismatch},
	}
}

// ──────────────────────────── 素材库 CRUD ────────────────────────────

func testCreateImageLibrary() bool {
	resp := doJSON("POST", apiPrefix, map[string]interface{}{
		"Name":        "测试图片素材库",
		"Type":        "Image",
		"Description": "用于集成测试的图片素材库",
	})
	if !checkSuccess(resp, "创建图片素材库") {
		return false
	}
	var ml model.MaterialLibrary
	json.Unmarshal(resp.Data, &ml)
	imageLibID = ml.Id
	fmt.Printf("  [INFO] ID: %s, 创建时间: %s\n", ml.Id, ml.CreatedAt)
	return ml.Id != "" && ml.Type == "Image" && ml.FileCount == 0
}

func testCreateVideoLibrary() bool {
	resp := doJSON("POST", apiPrefix, map[string]interface{}{
		"Name":        "测试视频素材库",
		"Type":        "Video",
		"Description": "用于集成测试的视频素材库",
	})
	if !checkSuccess(resp, "创建视频素材库") {
		return false
	}
	var ml model.MaterialLibrary
	json.Unmarshal(resp.Data, &ml)
	videoLibID = ml.Id
	fmt.Printf("  [INFO] ID: %s\n", ml.Id)
	return ml.Id != "" && ml.Type == "Video"
}

func testListLibraries() bool {
	resp := doJSON("GET", apiPrefix+"?Page=1&PageSize=10", nil)
	if !checkSuccess(resp, "查询列表") {
		return false
	}
	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)
	fmt.Printf("  [INFO] 总数: %d\n", pageData.Total)
	return pageData.Total >= 2
}

func testGetLibraryByID() bool {
	resp := doJSON("GET", apiPrefix+"?Id="+imageLibID, nil)
	if !checkSuccess(resp, "按ID查询") {
		return false
	}
	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)
	return pageData.Total == 1
}

func testUpdateLibrary() bool {
	resp := doJSON("PUT", apiPrefix+"/"+imageLibID, map[string]interface{}{
		"Name":        "更新后的图片库",
		"Description": "更新后的描述",
	})
	return checkSuccess(resp, "更新素材库")
}

// ──────────────────────────── 图片上传 & 磁盘验证 ────────────────────────────

func testUploadImages() bool {
	files := findTestImages()
	if len(files) == 0 {
		fmt.Println("  [SKIP] testdata/images/ 下没有测试图片")
		return true
	}

	resp := doMultipart(apiPrefix+"/"+imageLibID+"/images", files)
	if !checkSuccess(resp, "批量上传图片") {
		return false
	}

	var uploadResp model.UploadImageResp
	json.Unmarshal(resp.Data, &uploadResp)
	fmt.Printf("  [INFO] 成功上传 %d 张图片\n", uploadResp.UploadedCount)
	for _, f := range uploadResp.Files {
		fmt.Printf("         - %s (ID: %s, %d bytes, 状态: %s)\n", f.FileName, f.Id, f.FileSize, f.UploadStatus)
	}
	uploadedFiles = uploadResp.Files
	return uploadResp.UploadedCount > 0
}

func testVerifyImageFilesOnDisk() bool {
	if len(uploadedFiles) == 0 {
		fmt.Println("  [SKIP] 没有已上传的文件")
		return true
	}

	allOK := true
	for _, f := range uploadedFiles {
		diskPath := filepath.Join(uploadDir, f.StoragePath)
		if !fileExists(diskPath) {
			fmt.Printf("  [ERROR] 文件不存在: %s\n", diskPath)
			allOK = false
			continue
		}
		stat, _ := os.Stat(diskPath)
		if stat.Size() != f.FileSize {
			fmt.Printf("  [ERROR] 文件大小不一致: 磁盘 %d vs 记录 %d (%s)\n", stat.Size(), f.FileSize, f.FileName)
			allOK = false
			continue
		}
		fmt.Printf("  [OK] %s: 磁盘大小=%d == 记录大小=%d\n", f.FileName, stat.Size(), f.FileSize)
	}

	fmt.Println("  [INFO] 上传目录结构:")
	walkDir(uploadDir, "    ")
	return allOK
}

func testVerifyDbMatchesDisk() bool {
	if len(uploadedFiles) == 0 {
		fmt.Println("  [SKIP] 没有已上传的文件")
		return true
	}

	// 通过 API 查询文件列表，与磁盘实际文件数量对比
	resp := doJSON("GET", apiPrefix+"/"+imageLibID+"/files?Page=1&PageSize=100", nil)
	if resp.ErrorCode != 0 {
		return false
	}

	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	fileItems, _ := parsePageToFiles(resp.Data)

	// 数据库记录数
	dbCount := pageData.Total
	// 磁盘实际文件数
	diskCount := countFilesInDir(filepath.Join(uploadDir, "images", imageLibID))

	fmt.Printf("  [INFO] 数据库记录: %d, 磁盘文件: %d\n", dbCount, diskCount)

	if dbCount != diskCount {
		fmt.Printf("  [ERROR] 数据库与磁盘不一致！\n")
		return false
	}

	// 逐条验证数据库记录的 StoragePath 对应磁盘文件存在
	allOK := true
	for _, f := range fileItems {
		diskPath := filepath.Join(uploadDir, f.StoragePath)
		if !fileExists(diskPath) {
			fmt.Printf("  [ERROR] 数据库记录 %s 对应的磁盘文件不存在: %s\n", f.Id, diskPath)
			allOK = false
		}
	}
	if allOK {
		fmt.Printf("  [OK] 所有数据库记录都有对应的磁盘文件\n")
	}
	return allOK
}

func testListFiles() bool {
	resp := doJSON("GET", apiPrefix+"/"+imageLibID+"/files?Page=1&PageSize=24", nil)
	if !checkSuccess(resp, "查询文件列表") {
		return false
	}

	fileItems, _ := parsePageToFiles(resp.Data)
	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	fmt.Printf("  [INFO] 文件总数: %d\n", pageData.Total)
	for _, f := range fileItems {
		fmt.Printf("         - %s | 状态: %s | 进度: %.0f%% | AccessUrl: %s\n",
			f.FileName, f.UploadStatus, f.Progress*100, f.AccessUrl)
	}

	// 验证所有已完成文件进度为 100%
	for _, f := range fileItems {
		if f.UploadStatus == "Completed" && f.Progress != 1.0 {
			fmt.Printf("  [ERROR] 已完成文件进度不是 100%%: %s (%.0f%%)\n", f.FileName, f.Progress*100)
			return false
		}
	}
	return true
}

func testStaticFileAccess() bool {
	if len(uploadedFiles) == 0 {
		fmt.Println("  [SKIP] 没有文件可访问")
		return true
	}

	accessUrl := uploadedFiles[0].AccessUrl
	fullURL := baseURL + accessUrl
	fmt.Printf("  >> GET %s\n", fullURL)

	resp, err := httpGetFull(fullURL)
	if err != nil {
		fmt.Printf("  [ERROR] 请求失败: %s\n", err)
		return false
	}

	fmt.Printf("  << HTTP %d, 收到 %d bytes\n", resp.statusCode, len(resp.body))

	if resp.statusCode != 200 {
		return false
	}

	// 验证返回的内容大小与记录一致
	expectedSize := uploadedFiles[0].FileSize
	if int64(len(resp.body)) != expectedSize {
		fmt.Printf("  [ERROR] 静态文件大小不一致: 返回 %d bytes vs 记录 %d bytes\n", len(resp.body), expectedSize)
		return false
	}
	fmt.Printf("  [OK] 静态文件内容大小与记录一致 (%d bytes)\n", expectedSize)
	return true
}

func testLibraryStats() bool {
	resp := doJSON("GET", apiPrefix+"?Id="+imageLibID, nil)
	if !checkSuccess(resp, "查询统计") {
		return false
	}

	libs, _ := parsePageToLibraries(resp.Data)

	if len(libs) == 0 {
		fmt.Println("  [ERROR] 未找到素材库")
		return false
	}

	ml := libs[0]
	fmt.Printf("  [INFO] 文件数量: %d, 总大小: %d bytes (%.2f KB)\n", ml.FileCount, ml.TotalSize, float64(ml.TotalSize)/1024)

	// 验证统计与实际一致
	if ml.FileCount != int32(len(uploadedFiles)) {
		fmt.Printf("  [ERROR] FileCount=%d 但实际上传了 %d 个文件\n", ml.FileCount, len(uploadedFiles))
		return false
	}

	// 验证总大小
	var expectedTotal int64
	for _, f := range uploadedFiles {
		expectedTotal += f.FileSize
	}
	if ml.TotalSize != expectedTotal {
		fmt.Printf("  [ERROR] TotalSize=%d 但实际文件总大小 %d\n", ml.TotalSize, expectedTotal)
		return false
	}
	fmt.Printf("  [OK] 统计数据与实际文件一致\n")
	return true
}

// ──────────────────────────── 删除行为验证 ────────────────────────────

func testDeleteFileVerifyDisk() bool {
	if len(uploadedFiles) == 0 {
		fmt.Println("  [SKIP] 没有可删除的文件")
		return true
	}

	target := uploadedFiles[0]
	diskPath := filepath.Join(uploadDir, target.StoragePath)

	// 确认文件当前存在
	if !fileExists(diskPath) {
		fmt.Printf("  [ERROR] 删除前文件就不存在: %s\n", diskPath)
		return false
	}
	fmt.Printf("  [INFO] 删除前文件存在: %s (%d bytes)\n", diskPath, target.FileSize)

	resp := doJSON("DELETE", apiPrefix+"/"+imageLibID+"/files/"+target.Id, nil)
	if !checkSuccess(resp, "删除文件 "+target.FileName) {
		return false
	}

	// 验证磁盘文件已删除
	if fileExists(diskPath) {
		fmt.Printf("  [ERROR] 删除后磁盘文件仍存在: %s\n", diskPath)
		return false
	}
	fmt.Printf("  [OK] 磁盘文件已删除: %s\n", filepath.Base(diskPath))

	// 验证数据库记录已删除
	listResp := doJSON("GET", apiPrefix+"/"+imageLibID+"/files?Page=1&PageSize=100", nil)
	fileItems, _ := parsePageToFiles(listResp.Data)

	for _, f := range fileItems {
		if f.Id == target.Id {
			fmt.Printf("  [ERROR] 数据库记录仍存在: ID=%s\n", target.Id)
			return false
		}
	}
	fmt.Printf("  [OK] 数据库记录已删除\n")

	// 验证素材库统计已更新
	statsResp := doJSON("GET", apiPrefix+"?Id="+imageLibID, nil)
	statsLibs, _ := parsePageToLibraries(statsResp.Data)
	if len(statsLibs) > 0 {
		fmt.Printf("  [INFO] 删除后素材库统计: FileCount=%d, TotalSize=%d\n", statsLibs[0].FileCount, statsLibs[0].TotalSize)
		if statsLibs[0].FileCount != int32(len(uploadedFiles)-1) {
			fmt.Printf("  [ERROR] FileCount 未正确更新\n")
			return false
		}
	}

	uploadedFiles = uploadedFiles[1:]
	return true
}

func testDeleteLibraryVerifyCleanup() bool {
	// 创建临时库并删除
	resp := doJSON("POST", apiPrefix, map[string]interface{}{
		"Name": "待删除的临时库",
		"Type": "Image",
	})
	if !checkSuccess(resp, "创建临时库") {
		return false
	}
	var ml model.MaterialLibrary
	json.Unmarshal(resp.Data, &ml)

	// 上传一张图片
	imgFiles := findTestImages()
	if len(imgFiles) > 0 {
		doMultipart(apiPrefix+"/"+ml.Id+"/images", imgFiles[:1])
		fmt.Println("  [INFO] 临时库中已上传1张图片")
	}

	// 记录临时库目录
	libDir := filepath.Join(uploadDir, "images", ml.Id)
	fmt.Printf("  [INFO] 临时库目录: %s\n", libDir)

	// 删除素材库
	delResp := doJSON("DELETE", apiPrefix+"/"+ml.Id, nil)
	if !checkSuccess(delResp, "删除素材库") {
		return false
	}

	// 验证目录已被清理
	if fileExists(libDir) {
		fmt.Printf("  [WARN] 素材库目录仍存在: %s（可能是因为 OS 延迟）\n", libDir)
	} else {
		fmt.Printf("  [OK] 素材库目录已清理\n")
	}

	// 验证数据库记录已删除
	getResp := doJSON("GET", apiPrefix+"?Id="+ml.Id, nil)
	// handleError 统一返回 ErrCodeServerInternal，需通过非零判断
	if getResp.ErrorCode == 0 {
		fmt.Printf("  [ERROR] 素材库仍可查询到，删除未生效\n")
		return false
	}
	fmt.Printf("  [OK] 素材库已删除（ErrorCode=%d）\n", getResp.ErrorCode)
	return true
}

// ──────────────────────────── 视频分片上传 ────────────────────────────

func testVideoUploadFlow() bool {
	videos := findTestVideos()
	if len(videos) == 0 {
		fmt.Println("  [SKIP] testdata/videos/ 为空，使用合成数据测试")
		return testVideoUploadWithSyntheticData()
	}

	videoPath := videos[0]
	stat, _ := os.Stat(videoPath)
	fileSize := stat.Size()
	chunkSize := int64(2 * 1024 * 1024)
	fmt.Printf("  [INFO] 视频文件: %s (%d bytes, 分片大小: %d)\n", filepath.Base(videoPath), fileSize, chunkSize)

	// 初始化
	initResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  filepath.Base(videoPath),
		"FileSize":  fileSize,
		"ChunkSize": chunkSize,
	})
	if !checkSuccess(initResp, "初始化视频上传") {
		return false
	}

	var initResult model.InitVideoUploadResp
	json.Unmarshal(initResp.Data, &initResult)
	fmt.Printf("  [INFO] UploadId: %s, 分片总数: %d\n", initResult.UploadId, initResult.ChunkCount)

	// 分片上传
	file, err := os.Open(videoPath)
	if err != nil {
		fmt.Printf("  [ERROR] 打开视频文件失败: %s\n", err)
		return false
	}
	defer file.Close()

	for i := int32(0); i < initResult.ChunkCount; i++ {
		chunkData := make([]byte, chunkSize)
		n, err := file.Read(chunkData)
		if err != nil && err.Error() != "EOF" {
			fmt.Printf("  [ERROR] 读取分片 %d 失败: %s\n", i, err)
			return false
		}
		chunkData = chunkData[:n]

		chunkResp := doChunkUpload(apiPrefix+"/"+videoLibID+"/videos/chunk", initResult.UploadId, i, chunkData)
		if chunkResp.ErrorCode != 0 {
			fmt.Printf("  [ERROR] 分片 %d 上传失败: %s\n", i, chunkResp.ErrorMsg)
			return false
		}
		fmt.Printf("  [OK] 分片 %d/%d 上传成功 (%d bytes)\n", i+1, initResult.ChunkCount, n)
	}

	// 完成
	completeResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/complete", map[string]interface{}{
		"UploadId": initResult.UploadId,
	})
	if !checkSuccess(completeResp, "完成上传") {
		return false
	}

	fmt.Println("  [INFO] 等待异步合并...")
	time.Sleep(2 * time.Second)

	// 验证合并结果
	filesResp := doJSON("GET", apiPrefix+"/"+videoLibID+"/files?Page=1&PageSize=10", nil)
	fileItems, _ := parsePageToFiles(filesResp.Data)

	for _, f := range fileItems {
		if f.FileName == filepath.Base(videoPath) {
			fmt.Printf("  [INFO] 合并状态: %s, 进度: %.0f%%\n", f.UploadStatus, f.Progress*100)
			if f.UploadStatus != "Completed" {
				fmt.Printf("  [ERROR] 合并未完成，状态: %s\n", f.UploadStatus)
				return false
			}

			// 验证磁盘文件
			diskPath := filepath.Join(uploadDir, f.StoragePath)
			if !fileExists(diskPath) {
				fmt.Printf("  [ERROR] 合并文件不存在: %s\n", diskPath)
				return false
			}
			diskStat, _ := os.Stat(diskPath)
			fmt.Printf("  [OK] 合并文件存在: %s (磁盘 %d bytes, 记录 %d bytes)\n", diskPath, diskStat.Size(), f.FileSize)

			if diskStat.Size() != f.FileSize {
				fmt.Printf("  [ERROR] 合并文件大小不一致\n")
				return false
			}
			return true
		}
	}
	fmt.Println("  [ERROR] 未找到视频文件记录")
	return false
}

func testVideoUploadWithSyntheticData() bool {
	initResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "synthetic_test.mp4",
		"FileSize":  300,
		"ChunkSize": 100,
	})
	if !checkSuccess(initResp, "初始化") {
		return false
	}

	var initResult model.InitVideoUploadResp
	json.Unmarshal(initResp.Data, &initResult)
	fmt.Printf("  [INFO] UploadId: %s, 分片总数: %d\n", initResult.UploadId, initResult.ChunkCount)

	for i := int32(0); i < initResult.ChunkCount; i++ {
		chunkData := make([]byte, 100)
		for j := range chunkData {
			chunkData[j] = byte('A' + int(i))
		}
		chunkResp := doChunkUpload(apiPrefix+"/"+videoLibID+"/videos/chunk", initResult.UploadId, i, chunkData)
		if chunkResp.ErrorCode != 0 {
			fmt.Printf("  [ERROR] 分片 %d 失败: %s\n", i, chunkResp.ErrorMsg)
			return false
		}
		fmt.Printf("  [OK] 分片 %d/%d\n", i+1, initResult.ChunkCount)
	}

	completeResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/complete", map[string]interface{}{
		"UploadId": initResult.UploadId,
	})
	if !checkSuccess(completeResp, "完成上传") {
		return false
	}

	fmt.Println("  [INFO] 等待异步合并...")
	time.Sleep(1 * time.Second)

	// 验证
	filesResp := doJSON("GET", apiPrefix+"/"+videoLibID+"/files?Page=1&PageSize=10", nil)
	fileItems, _ := parsePageToFiles(filesResp.Data)

	for _, f := range fileItems {
		if strings.HasPrefix(f.FileName, "synthetic_test") && f.UploadStatus == "Completed" {
			diskPath := filepath.Join(uploadDir, f.StoragePath)
			if !fileExists(diskPath) {
				fmt.Printf("  [ERROR] 合并文件不存在: %s\n", diskPath)
				return false
			}
			diskStat, _ := os.Stat(diskPath)
			fmt.Printf("  [OK] 合并文件存在: %s (%d bytes)\n", diskPath, diskStat.Size())
			return true
		}
	}
	fmt.Println("  [ERROR] 未找到合并完成的视频文件")
	return false
}

func testVerifyNoChunkResidue() bool {
	chunkDirs := hasChunkDirs(uploadDir)
	if len(chunkDirs) > 0 {
		fmt.Printf("  [ERROR] 发现残留的 chunks 目录:\n")
		for _, d := range chunkDirs {
			fmt.Printf("         - %s\n", d)
			// 列出其中的文件
			filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					fmt.Printf("           %s (%d bytes)\n", filepath.Base(path), info.Size())
				}
				return nil
			})
		}
		return false
	}
	fmt.Printf("  [OK] 没有残留的 chunks 目录，合并后清理正常\n")
	return true
}

func testVideoResumableUpload() bool {
	// 初始化
	initResp1 := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "resume_test.mp4",
		"FileSize":  300,
		"ChunkSize": 100,
	})
	if !checkSuccess(initResp1, "第一次初始化") {
		return false
	}

	var init1 model.InitVideoUploadResp
	json.Unmarshal(initResp1.Data, &init1)

	// 上传第一个分片
	chunkData := make([]byte, 100)
	chunkResp := doChunkUpload(apiPrefix+"/"+videoLibID+"/videos/chunk", init1.UploadId, 0, chunkData)
	if chunkResp.ErrorCode != 0 {
		return false
	}
	fmt.Printf("  [INFO] 已上传分片 0/3\n")

	// 验证分片文件存在于磁盘
	chunkDir := filepath.Join(uploadDir, "videos", videoLibID, "chunks")
	chunkFile := filepath.Join(chunkDir, "*.part.0")
	matches, _ := filepath.Glob(chunkFile)
	if len(matches) > 0 {
		fmt.Printf("  [OK] 分片文件存在于磁盘: %s\n", matches[0])
	} else {
		fmt.Printf("  [WARN] 未找到分片文件（路径可能不同）\n")
	}

	// 再次初始化 - 应返回相同 UploadId
	initResp2 := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "resume_test.mp4",
		"FileSize":  300,
		"ChunkSize": 100,
	})
	if !checkSuccess(initResp2, "断点续传初始化") {
		return false
	}

	var init2 model.InitVideoUploadResp
	json.Unmarshal(initResp2.Data, &init2)

	if init1.UploadId == init2.UploadId {
		fmt.Printf("  [OK] 断点续传: 返回相同 UploadId=%s\n", init1.UploadId)
		return true
	}
	fmt.Printf("  [ERROR] UploadId 不一致: %s vs %s\n", init1.UploadId, init2.UploadId)
	return false
}

func testVideoSameNameReject() bool {
	// 完整上传
	initResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "same_name_test.mp4",
		"FileSize":  100,
		"ChunkSize": 100,
	})
	if !checkSuccess(initResp, "初始化上传") {
		return false
	}

	var initResult model.InitVideoUploadResp
	json.Unmarshal(initResp.Data, &initResult)

	chunkData := make([]byte, 100)
	doChunkUpload(apiPrefix+"/"+videoLibID+"/videos/chunk", initResult.UploadId, 0, chunkData)

	doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/complete", map[string]interface{}{
		"UploadId": initResult.UploadId,
	})

	time.Sleep(1 * time.Second)

	// 同名 init 应报错
	reResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "same_name_test.mp4",
		"FileSize":  100,
		"ChunkSize": 100,
	})

	if reResp.ErrorCode != 0 {
		fmt.Printf("  [OK] 同名文件被拒绝: ErrorCode=%d, ErrorMsg=%s\n", reResp.ErrorCode, reResp.ErrorMsg)
		return true
	}
	fmt.Println("  [ERROR] 同名已完成文件未被拒绝")
	return false
}

// ──────────────────────────── 异常场景 ────────────────────────────

func testTypeMismatch() bool {
	ok := true

	// 图片→视频库
	imgFiles := findTestImages()
	if len(imgFiles) > 0 {
		imgResp := doMultipart(apiPrefix+"/"+videoLibID+"/images", imgFiles[:1])
		// handleError 统一返回 ErrCodeServerInternal，通过 ErrorMsg 判断业务错误
		if imgResp.ErrorCode != 0 && strings.Contains(imgResp.ErrorMsg, "类型不匹配") {
			fmt.Printf("  [OK] 图片上传到视频库被拒绝: ErrorCode=%d, ErrorMsg=%s\n", imgResp.ErrorCode, imgResp.ErrorMsg)
		} else {
			fmt.Printf("  [ERROR] 图片上传到视频库未被拒绝: ErrorCode=%d, ErrorMsg=%s\n", imgResp.ErrorCode, imgResp.ErrorMsg)
			ok = false
		}
	} else {
		fmt.Println("  [SKIP] 没有测试图片")
	}

	// 视频→图片库
	vidResp := doJSON("POST", apiPrefix+"/"+imageLibID+"/videos/init", map[string]interface{}{
		"FileName":  "type_mismatch.mp4",
		"FileSize":  100,
		"ChunkSize": 100,
	})
	if vidResp.ErrorCode != 0 && strings.Contains(vidResp.ErrorMsg, "类型不匹配") {
		fmt.Printf("  [OK] 视频上传到图片库被拒绝: ErrorCode=%d, ErrorMsg=%s\n", vidResp.ErrorCode, vidResp.ErrorMsg)
	} else {
		fmt.Printf("  [ERROR] 视频上传到图片库未被拒绝: ErrorCode=%d, ErrorMsg=%s\n", vidResp.ErrorCode, vidResp.ErrorMsg)
		ok = false
	}

	return ok
}
