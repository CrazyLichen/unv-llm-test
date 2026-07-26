package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/common"
	"llm-test-server/internal/config"
	"llm-test-server/internal/controller"
	"llm-test-server/internal/llm"
	"llm-test-server/internal/model"
	"llm-test-server/internal/repository"
	"llm-test-server/internal/service"
)

// ──────────────────────────── 常量与变量 ────────────────────────────

const (
	serverPort = 18080
)

var (
	baseURL   = fmt.Sprintf("http://localhost:%d", serverPort)
	apiPrefix = fmt.Sprintf("http://localhost:%d/api/material-libraries", serverPort)
)

var (
	testRunDir string
	uploadDir  string
	dbPath     string
)

// ──────────────────────────── 测试框架核心类型 ────────────────────────────

// testCase 测试用例定义
type testCase struct {
	category    string              // 用例分类（如"模型配置CRUD"、"LLM真实交互"）
	name        string              // 用例名称
	description string              // 用例描述（测试什么场景）
	expected    string              // 期望结果
	fn          func() testResult   // 测试函数，返回 testResult
}

// testResult 测试执行结果
type testResult struct {
	passed  bool   // 是否通过
	actual  string // 实际结果描述
	detail  string // 补充信息（错误原因、关键数据等）
}

// caseResult 单条测试用例的执行结果
type caseResult struct {
	tc     testCase
	result testResult
	cost   time.Duration
}

func pass(actual string) testResult {
	return testResult{passed: true, actual: actual}
}

func passf(format string, args ...interface{}) testResult {
	return testResult{passed: true, actual: fmt.Sprintf(format, args...)}
}

func fail(actual string) testResult {
	return testResult{passed: false, actual: actual}
}

func failf(format string, args ...interface{}) testResult {
	return testResult{passed: false, actual: fmt.Sprintf(format, args...)}
}

func skip(reason string) testResult {
	return testResult{passed: true, actual: "跳过: " + reason}
}

// httpResponse 简易 HTTP 响应
type httpResponse struct {
	statusCode int
	body       []byte
}

func httpGetFull(url string) (*httpResponse, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return &httpResponse{statusCode: resp.StatusCode, body: body}, nil
}

// ──────────────────────────── 主函数 ────────────────────────────

func main() {
	fmt.Println("========================================")
	fmt.Println("  集成测试（真实服务 + 真实 LLM 交互）")
	fmt.Println("========================================")
	fmt.Println()

	if err := setupEnv(); err != nil {
		fmt.Printf("[FATAL] 初始化失败: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("[SETUP] 测试运行目录: %s\n", testRunDir)
	fmt.Printf("[SETUP] 数据库:        %s\n", dbPath)
	fmt.Printf("[SETUP] 上传目录:      %s\n", uploadDir)
	fmt.Println()

	serverStop, err := startServer()
	if err != nil {
		fmt.Printf("[FATAL] 启动服务失败: %s\n", err)
		os.Exit(1)
	}
	defer serverStop()

	if !waitForServer() {
		fmt.Println("[FATAL] 服务未能在 5 秒内就绪")
		os.Exit(1)
	}
	fmt.Printf("[SETUP] 服务已启动: %s\n\n", baseURL)

	tests := append(getModelConfigAndLLMTests(), getMaterialLibraryTests()...)

	// 执行所有测试用例，收集结果
	var results []caseResult

	fmt.Println("----------------------------------------")
	fmt.Println("  开始执行测试用例")
	fmt.Println("----------------------------------------")

	for _, tc := range tests {
		fmt.Printf("\n[RUN]  %s - %s\n", tc.category, tc.name)
		start := time.Now()
		r := tc.fn()
		cost := time.Since(start)
		results = append(results, caseResult{tc: tc, result: r, cost: cost})

		status := "PASS"
		if !r.passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s (耗时: %s)\n", status, tc.name, cost.Round(time.Millisecond))
	}

	// 生成测试报告
	fmt.Println()
	report := buildReport(results)
	fmt.Print(report)

	// 写入报告文件
	reportPath := filepath.Join(testRunDir, "test-report.txt")
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		fmt.Printf("[WARN] 写入报告文件失败: %s\n", err)
	} else {
		fmt.Printf("报告已写入: %s\n", reportPath)
	}

	// 统计
	passed := 0
	failed := 0
	for _, r := range results {
		if r.result.passed {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("\n测试产物目录: %s\n", testRunDir)

	if failed > 0 {
		os.Exit(1)
	}
}

// ──────────────────────────── 测试报告 ────────────────────────────

func buildReport(results []caseResult) string {
	// 按分类分组
	type group struct {
		category string
		items    []caseResult
	}
	var groups []group
	groupIdx := map[string]int{}

	for _, r := range results {
		idx, ok := groupIdx[r.tc.category]
		if !ok {
			groups = append(groups, group{category: r.tc.category})
			idx = len(groups) - 1
			groupIdx[r.tc.category] = idx
		}
		groups[idx].items = append(groups[idx].items, r)
	}

	// 统计
	totalPassed := 0
	totalFailed := 0
	for _, r := range results {
		if r.result.passed {
			totalPassed++
		} else {
			totalFailed++
		}
	}

	// 打印报告
	var sb strings.Builder

	sb.WriteString("╔══════════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                        集成测试报告                                 ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════════════╝\n")
	sb.WriteString("\n")

	for gi, g := range groups {
		sb.WriteString(fmt.Sprintf("── %s ──\n", g.category))

		for _, r := range g.items {
			status := "PASS"
			if !r.result.passed {
				status = "FAIL"
			}

			sb.WriteString(fmt.Sprintf("\n  [%s] %s\n", status, r.tc.name))
			sb.WriteString(fmt.Sprintf("    描述:   %s\n", r.tc.description))
			sb.WriteString(fmt.Sprintf("    期望:   %s\n", r.tc.expected))
			sb.WriteString(fmt.Sprintf("    实际:   %s\n", r.result.actual))
			if r.result.detail != "" {
				sb.WriteString(fmt.Sprintf("    详情:   %s\n", r.result.detail))
			}
			sb.WriteString(fmt.Sprintf("    耗时:   %s\n", r.cost.Round(time.Millisecond)))
		}

		if gi < len(groups)-1 {
			sb.WriteString("\n")
		}
	}

	// 汇总
	sb.WriteString("\n══════════════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("  汇总: 共 %d 个用例, %d 通过, %d 失败\n", len(results), totalPassed, totalFailed))
	if totalFailed > 0 {
		sb.WriteString("\n  失败用例:\n")
		for _, r := range results {
			if !r.result.passed {
				sb.WriteString(fmt.Sprintf("    - %s: %s\n", r.tc.name, r.result.actual))
			}
		}
	}
	sb.WriteString("══════════════════════════════════════════════════════════════════════\n")

	return sb.String()
}

// ──────────────────────────── 环境初始化 ────────────────────────────

func setupEnv() error {
	serverDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if filepath.Base(serverDir) == "integration" {
		serverDir = filepath.Dir(filepath.Dir(serverDir))
	}

	testRunDir = filepath.Join(serverDir, "data", "integration-test")
	uploadDir = filepath.Join(testRunDir, "uploads")
	dbPath = filepath.Join(testRunDir, "llm-test.db")

	os.RemoveAll(testRunDir)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("创建上传目录失败: %w", err)
	}
	return nil
}

func startServer() (func(), error) {
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: serverPort, Mode: "debug"},
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: dbPath},
		Log:      config.LogConfig{Level: "info", Format: "json", File: filepath.Join(testRunDir, "integration-test.log")},
		Upload: config.UploadConfig{
			Dir:              uploadDir,
			MaxImageSize:     10 * 1024 * 1024,
			MaxImageCount:    20,
			MaxImageBatchSize: 50 * 1024 * 1024,
		},
	}

	if err := common.InitLogger(&cfg.Log); err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	db, err := repository.InitDB(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	mcRepo := repository.NewModelConfigRepo(db)
	llmFactory := llm.NewClientFactory()
	llmClient := llm.NewLLMClient(llmFactory)
	mcSvc := service.NewModelConfigService(mcRepo, llmClient)
	mcCtrl := controller.NewModelConfigController(mcSvc)

	mlRepo := repository.NewMaterialLibraryRepo(db)
	mlSvc := service.NewMaterialLibraryService(mlRepo, &cfg.Upload)
	mlCtrl := controller.NewMaterialLibraryController(mlSvc)

	r := gin.Default()
	controller.SetupRouter(r, mcCtrl, mlCtrl, cfg.Upload.Dir)

	go func() {
		r.Run(fmt.Sprintf(":%d", serverPort))
	}()

	return func() {}, nil
}

func waitForServer() bool {
	for i := 0; i < 50; i++ {
		resp, err := http.Get(baseURL + "/api/material-libraries")
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// ──────────────────────────── HTTP 辅助 ────────────────────────────

type apiResponse struct {
	ErrorCode int             `json:"ErrorCode"`
	ErrorMsg  string          `json:"ErrorMsg"`
	Data      json.RawMessage `json:"Data"`
}

func doJSON(method, url string, body interface{}) *apiResponse {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
		fmt.Printf("  >> %s %s\n", method, url)
		fmt.Printf("  >> Body: %s\n", string(b))
	} else {
		fmt.Printf("  >> %s %s\n", method, url)
	}

	req, _ := http.NewRequest(method, url, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  << 请求失败: %s\n", err)
		return &apiResponse{ErrorCode: -1, ErrorMsg: err.Error()}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("  << HTTP %d\n", resp.StatusCode)

	var apiResp apiResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		fmt.Printf("  << 响应解析失败: %s\n", string(bodyBytes))
		return &apiResponse{ErrorCode: -2, ErrorMsg: "响应解析失败"}
	}

	pretty, _ := json.MarshalIndent(apiResp, "  ", "  ")
	fmt.Printf("  << %s\n", string(pretty))
	return &apiResp
}

func doMultipart(url string, filePaths []string) *apiResponse {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for _, fp := range filePaths {
		file, err := os.Open(fp)
		if err != nil {
			fmt.Printf("  << 打开文件失败: %s, %s\n", fp, err)
			continue
		}
		part, _ := writer.CreateFormFile("files", filepath.Base(fp))
		io.Copy(part, file)
		file.Close()
	}
	writer.Close()

	fmt.Printf("  >> POST %s (multipart, %d files)\n", url, len(filePaths))
	for _, f := range filePaths {
		stat, _ := os.Stat(f)
		fmt.Printf("     - %s (%d bytes)\n", filepath.Base(f), stat.Size())
	}

	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  << 请求失败: %s\n", err)
		return &apiResponse{ErrorCode: -1, ErrorMsg: err.Error()}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("  << HTTP %d\n", resp.StatusCode)

	var apiResp apiResponse
	json.Unmarshal(bodyBytes, &apiResp)

	pretty, _ := json.MarshalIndent(apiResp, "  ", "  ")
	fmt.Printf("  << %s\n", string(pretty))
	return &apiResp
}

func doChunkUpload(url, uploadId string, chunkIndex int32, chunkData []byte) *apiResponse {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("UploadId", uploadId)
	writer.WriteField("ChunkIndex", fmt.Sprintf("%d", chunkIndex))
	part, _ := writer.CreateFormFile("file", "chunk.dat")
	part.Write(chunkData)
	writer.Close()

	fmt.Printf("  >> POST %s (chunk, UploadId=%s, index=%d, size=%d)\n", url, uploadId, chunkIndex, len(chunkData))

	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  << 请求失败: %s\n", err)
		return &apiResponse{ErrorCode: -1, ErrorMsg: err.Error()}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var apiResp apiResponse
	json.Unmarshal(bodyBytes, &apiResp)
	return &apiResp
}

func testDataDir() string {
	dir, _ := os.Getwd()
	if filepath.Base(dir) == "integration" {
		dir = filepath.Dir(filepath.Dir(dir))
	}
	return filepath.Join(dir, "testdata")
}

func findTestImages() []string {
	imageDir := filepath.Join(testDataDir(), "images")
	files, _ := filepath.Glob(filepath.Join(imageDir, "*.jpg"))
	if len(files) == 0 {
		files, _ = filepath.Glob(filepath.Join(imageDir, "*.png"))
	}
	return files
}

func findTestVideos() []string {
	videoDir := filepath.Join(testDataDir(), "videos")
	files, _ := filepath.Glob(filepath.Join(videoDir, "*.mp4"))
	if len(files) == 0 {
		files, _ = filepath.Glob(filepath.Join(videoDir, "*"))
	}
	return files
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// countFilesInDir 递归统计目录下的文件数量
func countFilesInDir(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// hasChunkDirs 检查是否有残留的 chunks 目录
func hasChunkDirs(dir string) []string {
	var chunkDirs []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && info.Name() == "chunks" {
			chunkDirs = append(chunkDirs, path)
		}
		return nil
	})
	return chunkDirs
}

// parsePageItems 从 PageData 中解析 Items 为目标类型
func parsePageItems(data json.RawMessage, target interface{}) error {
	var pageData common.PageData
	if err := json.Unmarshal(data, &pageData); err != nil {
		return err
	}
	items, _ := json.Marshal(pageData.Items)
	return json.Unmarshal(items, target)
}

// parsePageToLibraries 从 PageData 中解析素材库列表
func parsePageToLibraries(data json.RawMessage) ([]model.MaterialLibrary, error) {
	var libs []model.MaterialLibrary
	return libs, parsePageItems(data, &libs)
}

// parsePageToFiles 从 PageData 中解析素材文件列表
func parsePageToFiles(data json.RawMessage) ([]model.MaterialFileProgress, error) {
	var files []model.MaterialFileProgress
	return files, parsePageItems(data, &files)
}
