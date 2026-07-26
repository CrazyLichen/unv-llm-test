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

// testCase 测试用例
type testCase struct {
	name string
	fn   func() bool
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

// ──────────────────────────── 导出函数 ────────────────────────────

func main() {
	fmt.Println("========================================")
	fmt.Println("  素材库接口集成测试（真实服务）")
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

	tests := getMaterialLibraryTests()

	passed := 0
	failed := 0

	fmt.Println("----------------------------------------")
	fmt.Println("  开始执行测试用例")
	fmt.Println("----------------------------------------")
	for _, tc := range tests {
		fmt.Printf("\n[RUN]  %s\n", tc.name)
		ok := tc.fn()
		if ok {
			passed++
			fmt.Printf("[PASS] %s\n", tc.name)
		} else {
			failed++
			fmt.Printf("[FAIL] %s\n", tc.name)
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Printf("  测试完成: %d 通过, %d 失败, 共 %d\n", passed, failed, passed+failed)
	fmt.Println("========================================")
	fmt.Printf("\n测试产物目录: %s\n", testRunDir)

	if failed > 0 {
		os.Exit(1)
	}
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
	llmClient := llm.NewLLMClient(nil) // 集成测试不需要真实LLM调用
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

func checkSuccess(resp *apiResponse, label string) bool {
	if resp.ErrorCode != 0 {
		fmt.Printf("  [ERROR] %s 失败: ErrorCode=%d, ErrorMsg=%s\n", label, resp.ErrorCode, resp.ErrorMsg)
		return false
	}
	fmt.Printf("  [OK] %s\n", label)
	return true
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

// walkDir 递归打印目录结构
func walkDir(dir string, indent string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		fullPath := filepath.Join(dir, e.Name())
		if e.IsDir() {
			fileCount := countFilesInDir(fullPath)
			fmt.Printf("%s%s/ (%d files)\n", indent, e.Name(), fileCount)
			walkDir(fullPath, indent+"  ")
		} else {
			info, _ := e.Info()
			fmt.Printf("%s%s (%d bytes)\n", indent, e.Name(), info.Size())
		}
	}
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
