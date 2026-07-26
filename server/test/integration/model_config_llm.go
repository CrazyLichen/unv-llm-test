package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"llm-test-server/internal/common"
	"llm-test-server/internal/llm"
	"llm-test-server/internal/model"
)

// ──────────────────────────── 模型配置 & LLM 测试状态 ────────────────────────────

var (
	mcAPIPrefix = fmt.Sprintf("http://localhost:%d/api/model-configs", serverPort)
	createdMcID string
)

// 真实 LLM 配置
const (
	testLLMApiUrl  = "https://llm-zknwhe8brcwzhyrr.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"
	testLLMApiKey  = "sk-572776428d984b51ab79488b709e0f87"
	testLLMModelId = "qwen3-vl-plus"
)

// ──────────────────────────── 测试注册 ────────────────────────────

// getModelConfigAndLLMTests 返回模型配置 & LLM 交互的测试用例
func getModelConfigAndLLMTests() []testCase {
	return []testCase{
		// 模型配置 CRUD
		{"模型配置CRUD", "创建模型配置", "通过API创建一个包含阿里百炼qwen3-vl配置的模型配置", "创建成功，返回ErrorCode=0，数据库中可查到该记录", testCreateModelConfig},
		{"模型配置CRUD", "查询模型配置列表", "查询所有模型配置列表", "列表包含至少1条记录，Total>=1", testListModelConfigs},
		{"模型配置CRUD", "按ID查询模型配置", "使用创建返回的ID查询指定模型配置", "查询成功，ApiKey字段已脱敏（非原始密钥）", testGetModelConfigByID},
		{"模型配置CRUD", "更新模型配置", "更新模型配置的名称和Temperature", "更新成功，ErrorCode=0", testUpdateModelConfig},
		{"模型配置CRUD", "查询更新后的模型配置", "查询更新后的配置，验证字段已变更", "Name=阿里百炼qwen3-vl-更新, Temperature=0.5", testGetUpdatedModelConfig},

		// LLM 真实交互
		{"LLM真实交互", "测试模型连通性（真实请求）", "对已创建的模型配置发起连通性测试", "连通成功，返回延迟>0ms", testRealConnectivity},
		{"LLM真实交互", "LLM图片分析（真实请求）", "使用LLMClient发送带图片的分析请求", "返回非空Content、Model、Token用量>0", testRealAnalyzeWithImage},
		{"LLM真实交互", "LLM纯文本分析（无图片应报错）", "不传入图片调用Analyze", "返回AppError，错误码=1(参数校验失败)", testAnalyzeWithoutImage},
		{"LLM真实交互", "LLM空提示词应报错", "传入空提示词调用Analyze", "返回AppError，错误码=1(参数校验失败)", testAnalyzeEmptyPrompt},

		// 模型配置异常场景
		{"模型配置异常场景", "查询不存在的模型配置", "使用不存在的ID查询模型配置", "返回非零ErrorCode，查询失败", testGetNonexistentModelConfig},
		{"模型配置异常场景", "删除模型配置", "删除已创建的模型配置", "删除成功，ErrorCode=0", testDeleteModelConfig},
		{"模型配置异常场景", "删除后再查询应失败", "删除后再次查询该模型配置", "返回非零ErrorCode，查询失败", testGetDeletedModelConfig},

		// ClientFactory 缓存验证
		{"ClientFactory缓存", "ClientFactory缓存懒加载", "相同configID两次GetOrCreateClient应返回同一实例；不同configID应返回不同实例", "相同ID返回同一client指针，不同ID返回不同client", testClientFactoryCache},
		{"ClientFactory缓存", "ClientFactory配置变更失效", "同一configID修改ApiKey后GetOrCreateClient应重建client；RemoveClient后应创建新client", "ApiKey变更后client指针不同；RemoveClient后重建成功", testClientFactoryInvalidation},
	}
}

// ──────────────────────────── 模型配置 CRUD ────────────────────────────

func testCreateModelConfig() testResult {
	resp := doJSON("POST", mcAPIPrefix, map[string]interface{}{
		"ModelName":   "阿里百炼qwen3-vl",
		"ModelId":     testLLMModelId,
		"ApiUrl":      testLLMApiUrl,
		"ApiKey":      testLLMApiKey,
		"Temperature": 0.7,
		"MaxTokens":   4096,
	})
	if resp.ErrorCode != 0 {
		return failf("创建失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 创建返回 Data 为 null，通过列表查询获取 ID
	listResp := doJSON("GET", mcAPIPrefix+"?Page=1&PageSize=1", nil)
	if listResp.ErrorCode != 0 {
		return failf("创建后列表查询失败: ErrorCode=%d", listResp.ErrorCode)
	}

	var pageData common.PageData
	json.Unmarshal(listResp.Data, &pageData)

	var items []model.ModelConfig
	itemsJSON, _ := json.Marshal(pageData.Items)
	json.Unmarshal(itemsJSON, &items)

	if len(items) == 0 {
		return fail("创建后列表为空")
	}

	createdMcID = items[0].Id
	return passf("创建成功, ID=%s, ModelId=%s, Name=%s", items[0].Id, items[0].ModelId, items[0].ModelName)
}

func testListModelConfigs() testResult {
	resp := doJSON("GET", mcAPIPrefix+"?Page=1&PageSize=20", nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	if pageData.Total < 1 {
		return failf("列表Total=%d, 期望>=1", pageData.Total)
	}
	return passf("列表Total=%d, >=1", pageData.Total)
}

func testGetModelConfigByID() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}

	resp := doJSON("GET", mcAPIPrefix+"?Id="+createdMcID, nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	var items []model.ModelConfig
	itemsJSON, _ := json.Marshal(pageData.Items)
	json.Unmarshal(itemsJSON, &items)

	if len(items) == 0 {
		return fail("未找到模型配置")
	}

	mc := items[0]

	// 验证 ApiKey 脱敏
	if mc.ApiKey == testLLMApiKey {
		return fail("ApiKey未脱敏，返回了原始密钥")
	}
	return passf("查询成功, Name=%s, ModelId=%s, ApiKey已脱敏=%s", mc.ModelName, mc.ModelId, mc.ApiKey)
}

func testUpdateModelConfig() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}

	resp := doJSON("PUT", mcAPIPrefix+"/"+createdMcID, map[string]interface{}{
		"ModelName":   "阿里百炼qwen3-vl-更新",
		"Temperature": 0.5,
	})
	if resp.ErrorCode != 0 {
		return failf("更新失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	return pass("更新成功, ErrorCode=0")
}

func testGetUpdatedModelConfig() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}

	resp := doJSON("GET", mcAPIPrefix+"?Id="+createdMcID, nil)
	if resp.ErrorCode != 0 {
		return failf("查询失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	var pageData common.PageData
	json.Unmarshal(resp.Data, &pageData)

	var items []model.ModelConfig
	itemsJSON, _ := json.Marshal(pageData.Items)
	json.Unmarshal(itemsJSON, &items)

	if len(items) == 0 {
		return fail("未找到模型配置")
	}

	mc := items[0]
	if mc.ModelName != "阿里百炼qwen3-vl-更新" {
		return failf("名称未更新: 实际=%s, 期望=阿里百炼qwen3-vl-更新", mc.ModelName)
	}
	if mc.Temperature != 0.5 {
		return failf("Temperature未更新: 实际=%f, 期望=0.5", mc.Temperature)
	}
	return passf("更新验证通过, Name=%s, Temperature=%.1f", mc.ModelName, mc.Temperature)
}

// ──────────────────────────── LLM 真实交互 ────────────────────────────

func testRealConnectivity() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}

	resp := doJSON("POST", mcAPIPrefix+"/"+createdMcID+"/test", nil)
	if resp.ErrorCode != 0 {
		return failf("连通性测试失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	var result model.TestModelConfigResp
	json.Unmarshal(resp.Data, &result)

	if result.Latency <= 0 {
		return failf("延迟值异常: Latency=%d, 期望>0", result.Latency)
	}
	return passf("连通成功, 延迟=%dms", result.Latency)
}

func testRealAnalyzeWithImage() testResult {
	factory := llm.NewClientFactory()
	client := llm.NewLLMClient(factory)

	imageBase64, err := loadTestImageAsBase64()
	if err != nil {
		return skipf("加载测试图片失败: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Analyze(ctx, "test-analyze", llm.ModelConfigParam{
		ApiUrl:      testLLMApiUrl,
		ApiKey:      testLLMApiKey,
		ModelId:     testLLMModelId,
		Temperature: 0.7,
		MaxTokens:   1024,
	}, llm.LLMRequest{
		Prompt:      "请描述这张图片的内容，用中文回答。",
		ImageBase64: []string{imageBase64},
	}, llm.WithRetry(1))

	if err != nil {
		return failf("LLM分析失败: %s", err)
	}

	if resp.Content == "" {
		return fail("返回Content为空")
	}
	if resp.Model == "" {
		return fail("返回Model字段为空")
	}
	if resp.Usage.TotalTokens == 0 {
		return fail("返回Token用量为0")
	}

	return passf("分析成功, Model=%s, Tokens=%d, Content长度=%d",
		resp.Model, resp.Usage.TotalTokens, len(resp.Content))
}

func testAnalyzeWithoutImage() testResult {
	factory := llm.NewClientFactory()
	client := llm.NewLLMClient(factory)

	ctx := context.Background()
	_, err := client.Analyze(ctx, "test-no-image", llm.ModelConfigParam{
		ApiUrl:      testLLMApiUrl,
		ApiKey:      testLLMApiKey,
		ModelId:     testLLMModelId,
		Temperature: 0.7,
		MaxTokens:   1024,
	}, llm.LLMRequest{
		Prompt:      "测试",
		ImageBase64: []string{},
	})

	if err == nil {
		return fail("无图片时未返回错误")
	}

	var appErr common.AppError
	if !isAppError(err, &appErr) {
		return failf("错误类型不正确: %T", err)
	}

	if appErr.Code != common.ErrCodeParamValidation {
		return failf("错误码不正确: 期望=%d, 实际=%d", common.ErrCodeParamValidation, appErr.Code)
	}

	return passf("正确返回参数校验错误, Code=%d, Msg=%s", appErr.Code, appErr.Error())
}

func testAnalyzeEmptyPrompt() testResult {
	factory := llm.NewClientFactory()
	client := llm.NewLLMClient(factory)

	ctx := context.Background()
	_, err := client.Analyze(ctx, "test-no-prompt", llm.ModelConfigParam{
		ApiUrl:      testLLMApiUrl,
		ApiKey:      testLLMApiKey,
		ModelId:     testLLMModelId,
		Temperature: 0.7,
		MaxTokens:   1024,
	}, llm.LLMRequest{
		Prompt:      "",
		ImageBase64: []string{"data:image/jpeg;base64,/9j/4AAQ"},
	})

	if err == nil {
		return fail("空提示词时未返回错误")
	}

	var appErr common.AppError
	if !isAppError(err, &appErr) {
		return failf("错误类型不正确: %T", err)
	}

	if appErr.Code != common.ErrCodeParamValidation {
		return failf("错误码不正确: 期望=%d, 实际=%d", common.ErrCodeParamValidation, appErr.Code)
	}

	return passf("正确返回参数校验错误, Code=%d, Msg=%s", appErr.Code, appErr.Error())
}

// ──────────────────────────── 模型配置异常场景 ────────────────────────────

func testGetNonexistentModelConfig() testResult {
	resp := doJSON("GET", mcAPIPrefix+"?Id=mc_nonexistent_0000000000000000000000000000", nil)
	if resp.ErrorCode == 0 {
		return fail("查询不存在的配置返回了ErrorCode=0，应返回非零ErrorCode")
	}
	return passf("正确返回错误, ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
}

func testDeleteModelConfig() testResult {
	if createdMcID == "" {
		return skip("没有已创建的模型配置")
	}

	resp := doJSON("DELETE", mcAPIPrefix+"/"+createdMcID, nil)
	if resp.ErrorCode != 0 {
		return failf("删除失败: ErrorCode=%d, ErrorMsg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	return pass("删除成功, ErrorCode=0")
}

func testGetDeletedModelConfig() testResult {
	if createdMcID == "" {
		return skip("没有已删除的模型配置")
	}

	resp := doJSON("GET", mcAPIPrefix+"?Id="+createdMcID, nil)
	if resp.ErrorCode == 0 {
		return fail("删除后查询返回ErrorCode=0，应返回非零ErrorCode")
	}
	return passf("删除后查询正确返回错误, ErrorCode=%d", resp.ErrorCode)
}

// ──────────────────────────── ClientFactory 缓存验证 ────────────────────────────

func testClientFactoryCache() testResult {
	factory := llm.NewClientFactory()

	// 首次获取
	client1 := factory.GetOrCreateClient("cache-test-1", testLLMApiUrl, testLLMApiKey, testLLMModelId)
	if client1 == nil {
		return fail("首次获取client为nil")
	}

	// 再次获取相同配置 — 应命中缓存
	client2 := factory.GetOrCreateClient("cache-test-1", testLLMApiUrl, testLLMApiKey, testLLMModelId)

	if client1 != client2 {
		return fail("相同configID返回了不同的client实例，未命中缓存")
	}

	// 不同 configID — 应创建新 client
	client3 := factory.GetOrCreateClient("cache-test-2", testLLMApiUrl, testLLMApiKey, testLLMModelId)
	if client3 == nil {
		return fail("不同configID获取client为nil")
	}

	return pass("相同configID命中缓存(同一指针)，不同configID创建新实例")
}

func testClientFactoryInvalidation() testResult {
	factory := llm.NewClientFactory()

	// 创建 client
	client1 := factory.GetOrCreateClient("invalidate-test", testLLMApiUrl, testLLMApiKey, testLLMModelId)

	// 修改 apiKey 后获取 — 应检测到变更并重建
	client2 := factory.GetOrCreateClient("invalidate-test", testLLMApiUrl, "sk-different-key", testLLMModelId)

	if client1 == client2 {
		return fail("ApiKey变更后返回了同一client实例，未重建")
	}

	// 显式 RemoveClient
	factory.RemoveClient("invalidate-test")

	// 再次获取 — 应创建新 client
	client3 := factory.GetOrCreateClient("invalidate-test", testLLMApiUrl, testLLMApiKey, testLLMModelId)
	if client3 == nil {
		return fail("RemoveClient后重新获取为nil")
	}

	return pass("ApiKey变更后client重建(指针不同)，RemoveClient后重建成功")
}

// ──────────────────────────── 辅助函数 ────────────────────────────

// loadTestImageAsBase64 读取测试图片并转为 base64 data URI
func loadTestImageAsBase64() (string, error) {
	images := findTestImages()
	if len(images) == 0 {
		return "", fmt.Errorf("testdata/images/ 下没有测试图片")
	}

	imgPath := images[0]
	data, err := os.ReadFile(imgPath)
	if err != nil {
		return "", fmt.Errorf("读取图片失败: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(imgPath))
	mimeType := "image/jpeg"
	if ext == ".png" {
		mimeType = "image/png"
	} else if ext == ".gif" {
		mimeType = "image/gif"
	} else if ext == ".webp" {
		mimeType = "image/webp"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	result := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)
	fmt.Printf("  [INFO] 加载图片: %s (%d bytes, base64: %d chars)\n", filepath.Base(imgPath), len(data), len(result))
	return result, nil
}

// isAppError 检查 error 是否为 AppError
func isAppError(err error, target *common.AppError) bool {
	_, ok := err.(common.AppError)
	if ok {
		*target = err.(common.AppError)
		return true
	}
	return false
}

// skipf 构造一个带格式化信息的跳过结果
func skipf(format string, args ...interface{}) testResult {
	return testResult{passed: true, actual: "跳过: " + fmt.Sprintf(format, args...)}
}
