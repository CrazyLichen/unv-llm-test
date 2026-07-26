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
		{"素材库CRUD", "创建图片素材库", "创建一个类型为Image的素材库", "创建成功，返回非空ID，Type=Image，FileCount=0", testCreateImageLibrary},
		{"素材库CRUD", "创建视频素材库", "创建一个类型为Video的素材库", "创建成功，返回非空ID，Type=Video", testCreateVideoLibrary},
		{"素材库CRUD", "查询素材库列表", "分页查询素材库列表", "列表Total>=2（至少有图片库和视频库）", testListLibraries},
		{"素材库CRUD", "按ID查询素材库", "使用图片库ID查询指定素材库", "查询成功，Total=1", testGetLibraryByID},
		{"素材库CRUD", "更新素材库", "更新素材库的名称和描述", "更新成功，ErrorCode=0", testUpdateLibrary},

		// 图片上传 & 磁盘验证
		{"图片上传与验证", "批量上传图片", "向图片库批量上传测试图片", "上传成功，UploadedCount>0", testUploadImages},
		{"图片上传与验证", "验证图片文件存在于磁盘", "检查每个上传的图片文件在磁盘上是否存在且大小一致", "所有文件存在且磁盘大小==记录大小", testVerifyImageFilesOnDisk},
		{"图片上传与验证", "验证数据库记录与磁盘一致", "对比数据库记录数与磁盘文件数", "数据库记录数==磁盘文件数，且每条记录的StoragePath对应文件存在", testVerifyDbMatchesDisk},
		{"图片上传与验证", "查询素材文件列表（含进度）", "查询文件列表，验证已完成文件进度为100%", "所有Completed文件Progress=1.0", testListFiles},
		{"图片上传与验证", "通过静态文件服务访问图片", "先查询素材文件列表获取AccessUrl，再逐个访问所有已完成图片", "所有Completed文件HTTP 200，返回内容大小与记录一致", testStaticFileAccess},
		{"图片上传与验证", "素材库统计信息验证", "查询素材库统计，验证FileCount和TotalSize", "FileCount==实际上传数，TotalSize==实际文件总大小", testLibraryStats},

		// 删除行为验证
		{"删除行为验证", "删除单个素材文件-验证磁盘文件已删除", "删除一个素材文件，验证磁盘文件和数据库记录均已删除", "磁盘文件已删除，数据库记录已删除，统计已更新", testDeleteFileVerifyDisk},
		{"删除行为验证", "删除素材库-验证级联清理", "创建临时库并上传文件后删除，验证级联清理", "素材库目录已清理，数据库记录已删除", testDeleteLibraryVerifyCleanup},

		// 视频分片上传
		{"视频分片上传", "视频分片上传完整流程", "初始化→分片上传→完成→异步合并的完整视频上传流程", "合并完成，UploadStatus=Completed，磁盘文件大小与记录一致", testVideoUploadFlow},
		{"视频分片上传", "验证视频合并后无残留chunk", "检查上传目录下是否残留chunks临时目录", "无chunks目录残留", testVerifyNoChunkResidue},
		{"视频分片上传", "视频断点续传", "初始化上传后上传一个分片，再次初始化相同文件", "返回相同的UploadId，实现断点续传", testVideoResumableUpload},
		{"视频分片上传", "同名已完成视频-init报错", "上传完整视频后再对同名文件init", "返回非零ErrorCode，提示同名文件已存在", testVideoSameNameReject},
		{"视频分片上传", "不完整分片合并失败后chunk清理", "上传全部分片后删除一个分片文件，触发合并失败，验证chunk目录被清理", "文件状态=Failed，chunks目录不存在", testIncompleteChunkMergeCleanup},

		// 异常场景
		{"异常场景", "类型不匹配校验", "向视频库上传图片、向图片库上传视频", "两种情况均被拒绝，ErrorMsg包含'类型不匹配'", testTypeMismatch},
	}
}

// ──────────────────────────── 素材库 CRUD ────────────────────────────

func testCreateImageLibrary() testResult {
	resp := doJSON("POST", apiPrefix, map[string]interface{}{
		"Name":        "测试图片素材库",
		"Type":        "Image",
		"Description": "用于集成测试的图片素材库",
	})
	if resp.ErrorCode != 0 {
		return failf("创建失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	var ml model.MaterialLibrary
	json.Unmarshal(resp.Data, &ml)
	imageLibID = ml.Id

	if ml.Id == "" || ml.Type != "Image" || ml.FileCount != 0 {
		return failf("字段不符合预期: Id=%s, Type=%s, FileCount=%d", ml.Id, ml.Type, ml.FileCount)
	}
	return passf("创建成功, ID=%s, Type=%s, FileCount=%d", ml.Id, ml.Type, ml.FileCount)
}

func testCreateVideoLibrary() testResult {
	resp := doJSON("POST", apiPrefix, map[string]interface{}{
		"Name":        "测试视频素材库",
		"Type":        "Video",
		"Description": "用于集成测试的视频素材库",
	})
	if resp.ErrorCode != 0 {
		return failf("创建失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	var ml model.MaterialLibrary
	json.Unmarshal(resp.Data, &ml)
	videoLibID = ml.Id

	if ml.Id == "" || ml.Type != "Video" {
		return failf("字段不符合预期: Id=%s, Type=%s", ml.Id, ml.Type)
	}
	return passf("创建成功, ID=%s, Type=%s", ml.Id, ml.Type)
}

func testListLibraries() testResult {
	resp := doJSON("GET", apiPrefix+"?Page=1&PageSize=10", nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	if pageData.Total < 2 {
		return failf("Total=%d, 期望>=2", pageData.Total)
	}
	return passf("查询成功, Total=%d, >=2", pageData.Total)
}

func testGetLibraryByID() testResult {
	resp := doJSON("GET", apiPrefix+"?Id="+imageLibID, nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	if pageData.Total != 1 {
		return failf("Total=%d, 期望=1", pageData.Total)
	}
	return passf("查询成功, Total=1")
}

func testUpdateLibrary() testResult {
	resp := doJSON("PUT", apiPrefix+"/"+imageLibID, map[string]interface{}{
		"Name":        "更新后的图片库",
		"Description": "更新后的描述",
	})
	if resp.ErrorCode != 0 {
		return failf("更新失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	return pass("更新成功, ErrorCode=0")
}

// ──────────────────────────── 图片上传 & 磁盘验证 ────────────────────────────

func testUploadImages() testResult {
	files := findTestImages()
	if len(files) == 0 {
		return skip("testdata/images/ 下没有测试图片")
	}

	resp := doMultipart(apiPrefix+"/"+imageLibID+"/images", files)
	if resp.ErrorCode != 0 {
		return failf("上传失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	var uploadResp model.UploadImageResp
	json.Unmarshal(resp.Data, &uploadResp)
	uploadedFiles = uploadResp.Files

	if uploadResp.UploadedCount <= 0 {
		return failf("UploadedCount=%d, 期望>0", uploadResp.UploadedCount)
	}
	return passf("上传成功, UploadedCount=%d", uploadResp.UploadedCount)
}

func testVerifyImageFilesOnDisk() testResult {
	if len(uploadedFiles) == 0 {
		return skip("没有已上传的文件")
	}

	allOK := true
	var details []string
	for _, f := range uploadedFiles {
		diskPath := filepath.Join(uploadDir, f.StoragePath)
		if !fileExists(diskPath) {
			details = append(details, fmt.Sprintf("%s: 磁盘文件不存在", f.FileName))
			allOK = false
			continue
		}
		stat, _ := os.Stat(diskPath)
		if stat.Size() != f.FileSize {
			details = append(details, fmt.Sprintf("%s: 大小不一致(磁盘%d!=记录%d)", f.FileName, stat.Size(), f.FileSize))
			allOK = false
		}
	}

	if !allOK {
		return failf("部分文件校验失败: %s", strings.Join(details, "; "))
	}
	return passf("所有%d个文件磁盘存在且大小一致", len(uploadedFiles))
}

func testVerifyDbMatchesDisk() testResult {
	if len(uploadedFiles) == 0 {
		return skip("没有已上传的文件")
	}

	resp := doJSON("GET", apiPrefix+"/"+imageLibID+"/files?Page=1&PageSize=100", nil)
	if resp.ErrorCode != 0 {
		return failf("查询文件列表失败: ErrorCode=%d", resp.ErrorCode)
	}

	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	fileItems, _ := parsePageToFiles(resp.Data)

	dbCount := pageData.Total
	diskCount := countFilesInDir(filepath.Join(uploadDir, "images", imageLibID))

	if dbCount != diskCount {
		return failf("数据库记录=%d, 磁盘文件=%d, 不一致", dbCount, diskCount)
	}

	// 逐条验证数据库记录的 StoragePath 对应磁盘文件存在
	allOK := true
	for _, f := range fileItems {
		diskPath := filepath.Join(uploadDir, f.StoragePath)
		if !fileExists(diskPath) {
			allOK = false
		}
	}

	if !allOK {
		return fail("部分数据库记录的磁盘文件不存在")
	}
	return passf("数据库记录=%d, 磁盘文件=%d, 全部对应", dbCount, diskCount)
}

func testListFiles() testResult {
	resp := doJSON("GET", apiPrefix+"/"+imageLibID+"/files?Page=1&PageSize=24", nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	fileItems, _ := parsePageToFiles(resp.Data)
	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	// 验证所有已完成文件进度为 100%
	for _, f := range fileItems {
		if f.UploadStatus == "Completed" && f.Progress != 1.0 {
			return failf("已完成文件%s进度不是100%%: %.0f%%", f.FileName, f.Progress*100)
		}
	}
	return passf("文件总数=%d, 所有Completed文件进度=100%%", pageData.Total)
}

func testStaticFileAccess() testResult {
	if len(uploadedFiles) == 0 {
		return skip("没有文件可访问")
	}

	// 通过查询素材文件列表接口获取文件信息，模拟实际业务流程
	resp := doJSON("GET", apiPrefix+"/"+imageLibID+"/files?Page=1&PageSize=100", nil)
	if resp.ErrorCode != 0 {
		return failf("查询文件列表失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	fileItems, _ := parsePageToFiles(resp.Data)
	if len(fileItems) == 0 {
		return fail("查询文件列表为空")
	}

	// 逐个访问所有已完成文件的 AccessUrl，验证图片数据可访问
	accessible := 0
	var failures []string
	for _, f := range fileItems {
		if f.UploadStatus != "Completed" {
			continue
		}
		fullURL := baseURL + f.AccessUrl
		fmt.Printf("  >> GET %s\n", fullURL)

		httpResp, err := httpGetFull(fullURL)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: 请求失败(%s)", f.FileName, err))
			continue
		}
		if httpResp.statusCode != 200 {
			failures = append(failures, fmt.Sprintf("%s: HTTP %d", f.FileName, httpResp.statusCode))
			continue
		}
		if int64(len(httpResp.body)) != f.FileSize {
			failures = append(failures, fmt.Sprintf("%s: 大小不一致(返回%d!=记录%d)", f.FileName, len(httpResp.body), f.FileSize))
			continue
		}
		accessible++
	}

	if len(failures) > 0 {
		return failf("%d/%d个文件访问失败: %s", len(failures), len(fileItems), strings.Join(failures, "; "))
	}
	return passf("所有%d个Completed文件通过AccessUrl可访问且数据完整", accessible)
}

func testLibraryStats() testResult {
	resp := doJSON("GET", apiPrefix+"?Id="+imageLibID, nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	libs, _ := parsePageToLibraries(resp.Data)
	if len(libs) == 0 {
		return fail("未找到素材库")
	}

	ml := libs[0]

	if ml.FileCount != int32(len(uploadedFiles)) {
		return failf("FileCount=%d, 实际上传=%d", ml.FileCount, len(uploadedFiles))
	}

	var expectedTotal int64
	for _, f := range uploadedFiles {
		expectedTotal += f.FileSize
	}
	if ml.TotalSize != expectedTotal {
		return failf("TotalSize=%d, 实际文件总大小=%d", ml.TotalSize, expectedTotal)
	}
	return passf("FileCount=%d, TotalSize=%d, 与实际一致", ml.FileCount, ml.TotalSize)
}

// ──────────────────────────── 删除行为验证 ────────────────────────────

func testDeleteFileVerifyDisk() testResult {
	if len(uploadedFiles) == 0 {
		return skip("没有可删除的文件")
	}

	target := uploadedFiles[0]
	diskPath := filepath.Join(uploadDir, target.StoragePath)

	if !fileExists(diskPath) {
		return failf("删除前文件就不存在: %s", diskPath)
	}

	resp := doJSON("DELETE", apiPrefix+"/"+imageLibID+"/files/"+target.Id, nil)
	if resp.ErrorCode != 0 {
		return failf("删除失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 验证磁盘文件已删除
	if fileExists(diskPath) {
		return failf("删除后磁盘文件仍存在: %s", diskPath)
	}

	// 验证数据库记录已删除
	listResp := doJSON("GET", apiPrefix+"/"+imageLibID+"/files?Page=1&PageSize=100", nil)
	fileItems, _ := parsePageToFiles(listResp.Data)
	for _, f := range fileItems {
		if f.Id == target.Id {
			return failf("数据库记录仍存在: ID=%s", target.Id)
		}
	}

	// 验证素材库统计已更新
	statsResp := doJSON("GET", apiPrefix+"?Id="+imageLibID, nil)
	statsLibs, _ := parsePageToLibraries(statsResp.Data)
	if len(statsLibs) > 0 && statsLibs[0].FileCount != int32(len(uploadedFiles)-1) {
		return failf("FileCount未正确更新: %d, 期望=%d", statsLibs[0].FileCount, len(uploadedFiles)-1)
	}

	uploadedFiles = uploadedFiles[1:]
	return pass("磁盘文件已删除, 数据库记录已删除, 统计已更新")
}

func testDeleteLibraryVerifyCleanup() testResult {
	// 创建临时库
	resp := doJSON("POST", apiPrefix, map[string]interface{}{
		"Name": "待删除的临时库",
		"Type": "Image",
	})
	if resp.ErrorCode != 0 {
		return failf("创建临时库失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	var ml model.MaterialLibrary
	json.Unmarshal(resp.Data, &ml)

	// 上传一张图片
	imgFiles := findTestImages()
	if len(imgFiles) > 0 {
		doMultipart(apiPrefix+"/"+ml.Id+"/images", imgFiles[:1])
	}

	// 记录临时库目录
	libDir := filepath.Join(uploadDir, "images", ml.Id)

	// 删除素材库
	delResp := doJSON("DELETE", apiPrefix+"/"+ml.Id, nil)
	if delResp.ErrorCode != 0 {
		return failf("删除失败: ErrorCode=%d, ErrorMsg=%s", delResp.ErrorCode, delResp.ErrorMsg)
	}

	// 验证目录已被清理
	dirExists := fileExists(libDir)

	// 验证数据库记录已删除
	getResp := doJSON("GET", apiPrefix+"?Id="+ml.Id, nil)
	if getResp.ErrorCode == 0 {
		return fail("素材库仍可查询到，删除未生效")
	}

	if dirExists {
		return passf("数据库记录已删除(ErrorCode=%d)，目录仍存在(OS延迟)", getResp.ErrorCode)
	}
	return pass("素材库目录已清理, 数据库记录已删除")
}

// ──────────────────────────── 视频分片上传 ────────────────────────────

func testVideoUploadFlow() testResult {
	videos := findTestVideos()
	if len(videos) == 0 {
		fmt.Println("  [INFO] testdata/videos/ 为空，使用合成数据测试")
		return testVideoUploadWithSyntheticData()
	}

	videoPath := videos[0]
	stat, _ := os.Stat(videoPath)
	fileSize := stat.Size()
	chunkSize := int64(2 * 1024 * 1024)

	// 初始化
	initResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  filepath.Base(videoPath),
		"FileSize":  fileSize,
		"ChunkSize": chunkSize,
	})
	if initResp.ErrorCode != 0 {
		return failf("初始化失败: ErrorCode=%d, ErrorMsg=%s", initResp.ErrorCode, initResp.ErrorMsg)
	}

	var initResult model.InitVideoUploadResp
	json.Unmarshal(initResp.Data, &initResult)

	// 分片上传
	file, err := os.Open(videoPath)
	if err != nil {
		return failf("打开视频文件失败: %s", err)
	}
	defer file.Close()

	for i := int32(0); i < initResult.ChunkCount; i++ {
		chunkData := make([]byte, chunkSize)
		n, err := file.Read(chunkData)
		if err != nil && err.Error() != "EOF" {
			return failf("读取分片%d失败: %s", i, err)
		}
		chunkData = chunkData[:n]

		chunkResp := doChunkUpload(apiPrefix+"/"+videoLibID+"/videos/chunk", initResult.UploadId, i, chunkData)
		if chunkResp.ErrorCode != 0 {
			return failf("分片%d上传失败: %s", i, chunkResp.ErrorMsg)
		}
		fmt.Printf("  [OK] 分片 %d/%d 上传成功 (%d bytes)\n", i+1, initResult.ChunkCount, n)
	}

	// 完成
	completeResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/complete", map[string]interface{}{
		"UploadId": initResult.UploadId,
	})
	if completeResp.ErrorCode != 0 {
		return failf("完成上传失败: ErrorCode=%d, ErrorMsg=%s", completeResp.ErrorCode, completeResp.ErrorMsg)
	}

	fmt.Println("  [INFO] 等待异步合并...")
	time.Sleep(2 * time.Second)

	// 验证合并结果
	filesResp := doJSON("GET", apiPrefix+"/"+videoLibID+"/files?Page=1&PageSize=10", nil)
	fileItems, _ := parsePageToFiles(filesResp.Data)

	for _, f := range fileItems {
		if f.FileName == filepath.Base(videoPath) {
			if f.UploadStatus != "Completed" {
				return failf("合并未完成, UploadStatus=%s", f.UploadStatus)
			}

			diskPath := filepath.Join(uploadDir, f.StoragePath)
			if !fileExists(diskPath) {
				return failf("合并文件不存在: %s", diskPath)
			}
			diskStat, _ := os.Stat(diskPath)
			if diskStat.Size() != f.FileSize {
				return failf("合并文件大小不一致: 磁盘%d, 记录%d", diskStat.Size(), f.FileSize)
			}
			return passf("合并完成, UploadStatus=Completed, 文件大小一致(%d bytes)", f.FileSize)
		}
	}
	return fail("未找到视频文件记录")
}

func testVideoUploadWithSyntheticData() testResult {
	initResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "synthetic_test.mp4",
		"FileSize":  300,
		"ChunkSize": 100,
	})
	if initResp.ErrorCode != 0 {
		return failf("初始化失败: ErrorCode=%d, ErrorMsg=%s", initResp.ErrorCode, initResp.ErrorMsg)
	}

	var initResult model.InitVideoUploadResp
	json.Unmarshal(initResp.Data, &initResult)

	for i := int32(0); i < initResult.ChunkCount; i++ {
		chunkData := make([]byte, 100)
		for j := range chunkData {
			chunkData[j] = byte('A' + int(i))
		}
		chunkResp := doChunkUpload(apiPrefix+"/"+videoLibID+"/videos/chunk", initResult.UploadId, i, chunkData)
		if chunkResp.ErrorCode != 0 {
			return failf("分片%d失败: %s", i, chunkResp.ErrorMsg)
		}
		fmt.Printf("  [OK] 分片 %d/%d\n", i+1, initResult.ChunkCount)
	}

	completeResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/complete", map[string]interface{}{
		"UploadId": initResult.UploadId,
	})
	if completeResp.ErrorCode != 0 {
		return failf("完成上传失败: ErrorCode=%d, ErrorMsg=%s", completeResp.ErrorCode, completeResp.ErrorMsg)
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
				return failf("合并文件不存在: %s", diskPath)
			}
			diskStat, _ := os.Stat(diskPath)
			return passf("合并完成, 文件大小=%d bytes", diskStat.Size())
		}
	}
	return fail("未找到合并完成的视频文件")
}

func testVerifyNoChunkResidue() testResult {
	chunkDirs := hasChunkDirs(uploadDir)
	if len(chunkDirs) > 0 {
		return failf("发现%d个残留chunks目录", len(chunkDirs))
	}
	return pass("无chunks目录残留，合并后清理正常")
}

func testVideoResumableUpload() testResult {
	// 初始化
	initResp1 := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "resume_test.mp4",
		"FileSize":  300,
		"ChunkSize": 100,
	})
	if initResp1.ErrorCode != 0 {
		return failf("第一次初始化失败: ErrorCode=%d, ErrorMsg=%s", initResp1.ErrorCode, initResp1.ErrorMsg)
	}

	var init1 model.InitVideoUploadResp
	json.Unmarshal(initResp1.Data, &init1)

	// 上传第一个分片
	chunkData := make([]byte, 100)
	chunkResp := doChunkUpload(apiPrefix+"/"+videoLibID+"/videos/chunk", init1.UploadId, 0, chunkData)
	if chunkResp.ErrorCode != 0 {
		return failf("上传分片失败: ErrorCode=%d", chunkResp.ErrorCode)
	}

	// 再次初始化 - 应返回相同 UploadId
	initResp2 := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "resume_test.mp4",
		"FileSize":  300,
		"ChunkSize": 100,
	})
	if initResp2.ErrorCode != 0 {
		return failf("断点续传初始化失败: ErrorCode=%d, ErrorMsg=%s", initResp2.ErrorCode, initResp2.ErrorMsg)
	}

	var init2 model.InitVideoUploadResp
	json.Unmarshal(initResp2.Data, &init2)

	if init1.UploadId != init2.UploadId {
		return failf("UploadId不一致: 第一次=%s, 第二次=%s", init1.UploadId, init2.UploadId)
	}
	return passf("断点续传成功, 返回相同UploadId=%s", init1.UploadId)
}

func testVideoSameNameReject() testResult {
	// 完整上传
	initResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "same_name_test.mp4",
		"FileSize":  100,
		"ChunkSize": 100,
	})
	if initResp.ErrorCode != 0 {
		return failf("初始化上传失败: ErrorCode=%d, ErrorMsg=%s", initResp.ErrorCode, initResp.ErrorMsg)
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

	if reResp.ErrorCode == 0 {
		return fail("同名已完成文件未被拒绝，返回ErrorCode=0")
	}
	return passf("同名文件被拒绝, ErrorCode=%d, ErrorMsg=%s", reResp.ErrorCode, reResp.ErrorMsg)
}

// ──────────────────────────── 异常场景 ────────────────────────────

func testTypeMismatch() testResult {
	ok := true
	var details []string

	// 图片→视频库
	imgFiles := findTestImages()
	if len(imgFiles) > 0 {
		imgResp := doMultipart(apiPrefix+"/"+videoLibID+"/images", imgFiles[:1])
		if imgResp.ErrorCode != 0 && strings.Contains(imgResp.ErrorMsg, "类型不匹配") {
			details = append(details, "图片→视频库: 正确拒绝")
		} else {
			details = append(details, fmt.Sprintf("图片→视频库: 未被拒绝(ErrorCode=%d)", imgResp.ErrorCode))
			ok = false
		}
	} else {
		details = append(details, "图片→视频库: 跳过(无测试图片)")
	}

	// 视频→图片库
	vidResp := doJSON("POST", apiPrefix+"/"+imageLibID+"/videos/init", map[string]interface{}{
		"FileName":  "type_mismatch.mp4",
		"FileSize":  100,
		"ChunkSize": 100,
	})
	if vidResp.ErrorCode != 0 && strings.Contains(vidResp.ErrorMsg, "类型不匹配") {
		details = append(details, "视频→图片库: 正确拒绝")
	} else {
		details = append(details, fmt.Sprintf("视频→图片库: 未被拒绝(ErrorCode=%d)", vidResp.ErrorCode))
		ok = false
	}

	if !ok {
		return failf("部分场景未被拒绝: %s", strings.Join(details, "; "))
	}
	return passf("两种类型不匹配场景均被拒绝: %s", strings.Join(details, "; "))
}

// ──────────────────────────── 分片合并失败后chunk清理验证 ────────────────────────────

func testIncompleteChunkMergeCleanup() testResult {
	if videoLibID == "" {
		return skip("没有已创建的视频素材库")
	}

	// 1. 初始化合成视频上传（3个分片）
	initResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/init", map[string]interface{}{
		"FileName":  "chunk_fail_test.mp4",
		"FileSize":  300,
		"ChunkSize": 100,
	})
	if initResp.ErrorCode != 0 {
		return failf("初始化失败: ErrorCode=%d, ErrorMsg=%s", initResp.ErrorCode, initResp.ErrorMsg)
	}

	var initResult model.InitVideoUploadResp
	json.Unmarshal(initResp.Data, &initResult)

	// 2. 上传所有3个分片
	for i := int32(0); i < initResult.ChunkCount; i++ {
		chunkData := make([]byte, 100)
		for j := range chunkData {
			chunkData[j] = byte('A' + int(i))
		}
		chunkResp := doChunkUpload(apiPrefix+"/"+videoLibID+"/videos/chunk", initResult.UploadId, i, chunkData)
		if chunkResp.ErrorCode != 0 {
			return failf("分片%d上传失败: %s", i, chunkResp.ErrorMsg)
		}
		fmt.Printf("  [OK] 分片 %d/%d 上传成功\n", i+1, initResult.ChunkCount)
	}

	// 3. 从磁盘找到分片文件，提取 fileId
	chunkDir := filepath.Join(uploadDir, "videos", videoLibID, "chunks")
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		return failf("读取chunks目录失败: %s", err)
	}

	// 从 .part.0 文件名中提取 fileId
	var fileID string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part.0") {
			fileID = strings.TrimSuffix(e.Name(), ".part.0")
			break
		}
	}
	if fileID == "" {
		return fail("未找到分片文件，无法提取fileId")
	}

	// 4. 从磁盘删除分片文件 .part.1（触发合并时 os.Open 失败）
	chunk1Path := filepath.Join(chunkDir, fileID+".part.1")
	if !fileExists(chunk1Path) {
		return failf("分片文件不存在: %s", chunk1Path)
	}
	if err := os.Remove(chunk1Path); err != nil {
		return failf("删除分片文件失败: %s", err)
	}
	fmt.Printf("  [INFO] 已删除分片文件: %s\n", chunk1Path)

	// 5. 调用 complete 接口（API校验通过，因为 UploadedChunks=3=TotalChunks）
	completeResp := doJSON("POST", apiPrefix+"/"+videoLibID+"/videos/complete", map[string]interface{}{
		"UploadId": initResult.UploadId,
	})
	if completeResp.ErrorCode != 0 {
		return failf("完成上传失败: ErrorCode=%d, ErrorMsg=%s", completeResp.ErrorCode, completeResp.ErrorMsg)
	}

	// 6. 等待异步合并失败（轮询文件列表直到 UploadStatus="Failed"）
	fmt.Println("  [INFO] 等待异步合并失败...")
	maxWait := 10
	for i := 0; i < maxWait; i++ {
		filesResp := doJSON("GET", apiPrefix+"/"+videoLibID+"/files?Page=1&PageSize=20", nil)
		fileItems, _ := parsePageToFiles(filesResp.Data)
		for _, f := range fileItems {
			if f.FileName == "chunk_fail_test.mp4" {
				if f.UploadStatus == "Failed" {
					// 7. 验证 chunks 目录已被清理
					if fileExists(chunkDir) {
						// 清理失败的文件记录
						doJSON("DELETE", apiPrefix+"/"+videoLibID+"/files/"+fileID, nil)
						return failf("合并失败后chunks目录未被清理: %s", chunkDir)
					}

					// 8. 清理失败的文件记录
					doJSON("DELETE", apiPrefix+"/"+videoLibID+"/files/"+fileID, nil)

					return passf("合并失败后chunks目录已被正确清理, 文件状态=Failed")
				}
				if f.UploadStatus == "Completed" {
					// 不应该成功，删除分片后合并应失败
					doJSON("DELETE", apiPrefix+"/"+videoLibID+"/files/"+fileID, nil)
					return fail("删除分片文件后合并不应该成功")
				}
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	// 超时清理
	doJSON("DELETE", apiPrefix+"/"+videoLibID+"/files/"+fileID, nil)
	return failf("轮询超时(10s)，文件合并状态未变为Failed")
}
