package model

import "database/sql"

// ──────────────────────────── 结构体 ────────────────────────────

// Task 任务实体（对应 tasks 表）
type Task struct {
	// Id 任务唯一标识
	Id string `json:"Id" gorm:"primaryKey;column:id"`
	// Name 任务名称
	Name string `json:"Name" gorm:"column:name;not null"`
	// Type 任务类型：Image / Video
	Type string `json:"Type" gorm:"column:type;not null"`
	// Status 任务状态：Pending / Analyzing / Paused / Completed
	Status string `json:"Status" gorm:"column:status;not null;default:Pending"`
	// ModelConfigId 使用的模型配置 ID
	ModelConfigId string `json:"ModelConfigId" gorm:"column:model_config_id;not null"`
	// MaterialLibraryId 关联素材库 ID
	MaterialLibraryId string `json:"MaterialLibraryId" gorm:"column:material_library_id;not null"`
	// Prompt 下发给大模型的提示词
	Prompt string `json:"Prompt" gorm:"column:prompt;not null"`
	// Target 检测目标名称
	Target string `json:"Target" gorm:"column:target;not null"`
	// FrameInterval 抽帧间隔（秒），Type=Video 时有值
	FrameInterval *int32 `json:"FrameInterval" gorm:"column:frame_interval"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt" gorm:"column:created_at;not null"`
	// UpdatedAt 最后更新时间
	UpdatedAt string `json:"UpdatedAt" gorm:"column:updated_at;not null"`
}

// Image 检测素材实体（对应 images 表）
type Image struct {
	// Id 素材唯一标识
	Id string `json:"Id" gorm:"primaryKey;column:id"`
	// TaskId 所属任务 ID
	TaskId string `json:"TaskId" gorm:"column:task_id;not null;index"`
	// AccessUrl 图片/帧浏览器访问 URL
	AccessUrl string `json:"AccessUrl" gorm:"column:access_url;not null"`
	// MaterialFileId 关联原始素材文件 ID，图片集有值，视频帧为 null
	MaterialFileId *string `json:"MaterialFileId" gorm:"column:material_file_id"`
	// FrameIndex 视频帧序号，视频帧有值
	FrameIndex *int32 `json:"FrameIndex" gorm:"column:frame_index"`
	// Status 检测状态：Pending / Detected / NotDetected / Failed
	Status string `json:"Status" gorm:"column:status;not null;default:Pending"`
	// Detection 检测结果（JSON）
	Detection sql.NullString `json:"Detection" gorm:"column:detection;type:text"`
	// FailReason 失败原因
	FailReason *string `json:"FailReason" gorm:"column:fail_reason"`
	// Correction 矫正标记：null / FalsePositive / DeletedFp
	Correction *string `json:"Correction" gorm:"column:correction"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt" gorm:"column:created_at;not null"`
}

// Box 检测框
type Box struct {
	// X1 左上角 X（0-1000 归一化）
	X1 int32 `json:"X1"`
	// Y1 左上角 Y（0-1000 归一化）
	Y1 int32 `json:"Y1"`
	// X2 右下角 X（0-1000 归一化）
	X2 int32 `json:"X2"`
	// Y2 右下角 Y（0-1000 归一化）
	Y2 int32 `json:"Y2"`
	// Confidence 置信度描述
	Confidence string `json:"Confidence"`
	// Label 目标标签
	Label string `json:"Label"`
}

// Detection 检测结果
type Detection struct {
	// HasTarget 是否检测到目标
	HasTarget bool `json:"HasTarget"`
	// Boxes 检测框列表
	Boxes []Box `json:"Boxes"`
	// RawResponse 大模型原始返回
	RawResponse string `json:"RawResponse"`
	// AnalyzedAt 分析完成时间
	AnalyzedAt string `json:"AnalyzedAt"`
}

// ──────────────────────────── 请求/响应 DTO ────────────────────────────

// CreateTaskReq 创建任务请求
type CreateTaskReq struct {
	// Name 任务名称
	Name string `json:"Name" binding:"required"`
	// Type 任务类型
	Type string `json:"Type" binding:"required,oneof=Image Video"`
	// ModelConfigId 模型配置 ID
	ModelConfigId string `json:"ModelConfigId" binding:"required"`
	// MaterialLibraryId 关联素材库 ID
	MaterialLibraryId string `json:"MaterialLibraryId" binding:"required"`
	// Prompt 下发给大模型的提示词
	Prompt string `json:"Prompt" binding:"required"`
	// Target 检测目标名称
	Target string `json:"Target" binding:"required"`
	// FrameInterval 抽帧间隔（秒），Type=Video 时必填
	FrameInterval *int32 `json:"FrameInterval"`
}

// UpdateTaskReq 更新任务请求（暂停/恢复）
type UpdateTaskReq struct {
	// Status 任务状态：Paused / Analyzing
	Status string `json:"Status" binding:"required,oneof=Paused Analyzing"`
}

// TaskItem 任务列表项（包含关联名称和进度）
type TaskItem struct {
	// Id 任务唯一标识
	Id string `json:"Id"`
	// Name 任务名称
	Name string `json:"Name"`
	// Type 任务类型
	Type string `json:"Type"`
	// Status 任务状态
	Status string `json:"Status"`
	// ModelConfigId 使用的模型配置 ID
	ModelConfigId string `json:"ModelConfigId"`
	// ModelConfigName 使用的模型配置名称
	ModelConfigName string `json:"ModelConfigName"`
	// MaterialLibraryId 关联素材库 ID
	MaterialLibraryId string `json:"MaterialLibraryId"`
	// MaterialLibraryName 关联素材库名称
	MaterialLibraryName string `json:"MaterialLibraryName"`
	// Prompt 下发给大模型的提示词
	Prompt string `json:"Prompt"`
	// Target 检测目标名称
	Target string `json:"Target"`
	// FrameInterval 抽帧间隔（秒）
	FrameInterval *int32 `json:"FrameInterval"`
	// Progress 检测进度与统计
	Progress TaskProgress `json:"Progress"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt"`
}

// TaskProgress 检测进度与统计
type TaskProgress struct {
	// Total 素材总数
	Total int `json:"Total"`
	// Completed 已完成检测数
	Completed int `json:"Completed"`
	// CompletedDetail 已完成明细
	CompletedDetail CompletedDetail `json:"CompletedDetail"`
	// Pending 待检测数
	Pending int `json:"Pending"`
}

// CompletedDetail 已完成明细
type CompletedDetail struct {
	// Detected 已检出目标数
	Detected int `json:"Detected"`
	// DetectedDetail 检出明细
	DetectedDetail DetectedDetail `json:"DetectedDetail"`
	// NotDetected 未检出目标数
	NotDetected int `json:"NotDetected"`
	// Failed 检测失败数
	Failed int `json:"Failed"`
}

// DetectedDetail 检出明细
type DetectedDetail struct {
	// TruePositive 正报数
	TruePositive int `json:"TruePositive"`
	// FalsePositive 误报数
	FalsePositive int `json:"FalsePositive"`
}
