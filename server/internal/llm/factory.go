package llm

import (
	"sync"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// ──────────────────────────── 结构体 ────────────────────────────

// ClientFactory openai.Client 缓存管理（懒加载、失效清理）
type ClientFactory struct {
	clients sync.Map // key: configID, value: *clientEntry
}

// clientEntry 缓存的 client 条目
type clientEntry struct {
	client  *openai.Client
	apiKey  string // 用于检测 config 变更
	apiUrl  string // 用于检测 config 变更
	modelId string // 用于检测 config 变更
}

// ──────────────────────────── 导出函数 ────────────────────────────

// NewClientFactory 创建 ClientFactory 实例
func NewClientFactory() *ClientFactory {
	return &ClientFactory{}
}

// GetOrCreateClient 获取或创建 openai.Client（懒加载，缓存命中时检测配置变更）
func (f *ClientFactory) GetOrCreateClient(configID, apiUrl, apiKey, modelId string) *openai.Client {
	// 检查缓存
	if val, ok := f.clients.Load(configID); ok {
		entry := val.(*clientEntry)
		// 配置未变更，返回缓存 client
		if entry.apiKey == apiKey && entry.apiUrl == apiUrl && entry.modelId == modelId {
			return entry.client
		}
		// 配置已变更，删除旧缓存
		f.clients.Delete(configID)
	}

	// 创建新 client
	client := openai.NewClient(
		option.WithBaseURL(apiUrl),
		option.WithAPIKey(apiKey),
	)

	// 缓存
	f.clients.Store(configID, &clientEntry{
		client:  &client,
		apiKey:  apiKey,
		apiUrl:  apiUrl,
		modelId: modelId,
	})

	return &client
}

// RemoveClient 移除指定配置的缓存 client
func (f *ClientFactory) RemoveClient(configID string) {
	f.clients.Delete(configID)
}
