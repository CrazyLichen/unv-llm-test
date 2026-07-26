package model

// ──────────────────────────── 结构体 ────────────────────────────

// ModelConfig 模型配置实体（对应 model_configs 表）
type ModelConfig struct {
	// Id 配置唯一标识
	Id string `json:"Id" gorm:"primaryKey;column:id"`
	// ModelName 用户自定义名称/显示名称
	ModelName string `json:"ModelName" gorm:"column:model_name;not null"`
	// ModelId API 模型标识（如 gpt-4o、qwen-vl-max）
	ModelId string `json:"ModelId" gorm:"column:model_id;not null"`
	// ApiUrl 大模型 API 地址
	ApiUrl string `json:"ApiUrl" gorm:"column:api_url;not null"`
	// ApiKey API 访问密钥
	ApiKey string `json:"ApiKey" gorm:"column:api_key;not null"`
	// Temperature 温度参数（0-2）
	Temperature float64 `json:"Temperature" gorm:"column:temperature;not null;default:0.7"`
	// MaxTokens 最大生成 token 数
	MaxTokens int32 `json:"MaxTokens" gorm:"column:max_tokens;not null;default:4096"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt" gorm:"column:created_at;not null"`
	// UpdatedAt 更新时间
	UpdatedAt string `json:"UpdatedAt" gorm:"column:updated_at;not null"`
}

// CreateModelConfigReq 创建模型配置请求
type CreateModelConfigReq struct {
	// ModelName 用户自定义名称
	ModelName string `json:"ModelName" binding:"required"`
	// ModelId API 模型标识
	ModelId string `json:"ModelId" binding:"required"`
	// ApiUrl 大模型 API 地址
	ApiUrl string `json:"ApiUrl" binding:"required"`
	// ApiKey API 访问密钥
	ApiKey string `json:"ApiKey" binding:"required"`
	// Temperature 温度参数，默认 0.7
	Temperature *float64 `json:"Temperature"`
	// MaxTokens 最大生成 token 数，默认 4096
	MaxTokens *int32 `json:"MaxTokens"`
}

// UpdateModelConfigReq 更新模型配置请求（指针类型区分未传与零值）
type UpdateModelConfigReq struct {
	// ModelName 用户自定义名称
	ModelName *string `json:"ModelName"`
	// ModelId API 模型标识
	ModelId *string `json:"ModelId"`
	// ApiUrl 大模型 API 地址
	ApiUrl *string `json:"ApiUrl"`
	// ApiKey API 访问密钥
	ApiKey *string `json:"ApiKey"`
	// Temperature 温度参数
	Temperature *float64 `json:"Temperature"`
	// MaxTokens 最大生成 token 数
	MaxTokens *int32 `json:"MaxTokens"`
}

// TestModelConfigResp 连通性测试响应
type TestModelConfigResp struct {
	// Latency 响应耗时（毫秒）
	Latency int `json:"Latency"`
}
