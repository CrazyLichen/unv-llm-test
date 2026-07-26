package model

// ──────────────────────────── 结构体 ────────────────────────────

// MaterialLibrary 素材库实体（对应 material_libraries 表）
type MaterialLibrary struct {
	// Id 素材库唯一标识
	Id string `json:"Id" gorm:"primaryKey;column:id"`
	// Name 素材库名称
	Name string `json:"Name" gorm:"column:name;not null"`
	// Type 素材类型：Image / Video
	Type string `json:"Type" gorm:"column:type;not null"`
	// Description 描述
	Description *string `json:"Description" gorm:"column:description"`
	// FileCount 文件数量
	FileCount int32 `json:"FileCount" gorm:"column:file_count;not null;default:0"`
	// TotalSize 文件总大小（字节）
	TotalSize int64 `json:"TotalSize" gorm:"column:total_size;not null;default:0"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt" gorm:"column:created_at;not null"`
	// UpdatedAt 最后更新时间
	UpdatedAt string `json:"UpdatedAt" gorm:"column:updated_at;not null"`
}

// MaterialFile 素材文件实体（对应 material_files 表）
type MaterialFile struct {
	// Id 文件唯一标识
	Id string `json:"Id" gorm:"primaryKey;column:id"`
	// LibraryId 所属素材库 ID
	LibraryId string `json:"LibraryId" gorm:"column:library_id;not null;index"`
	// FileName 原始文件名
	FileName string `json:"FileName" gorm:"column:file_name;not null"`
	// StoragePath 后端存储路径（相对路径）
	StoragePath string `json:"StoragePath" gorm:"column:storage_path;not null"`
	// AccessUrl 浏览器访问 URL
	AccessUrl string `json:"AccessUrl" gorm:"column:access_url;not null"`
	// FileSize 文件大小（字节）
	FileSize int64 `json:"FileSize" gorm:"column:file_size;not null;default:0"`
	// MimeType MIME 类型
	MimeType string `json:"MimeType" gorm:"column:mime_type;not null"`
	// UploadStatus 上传状态：Uploading / Merging / Completed / Failed
	UploadStatus string `json:"UploadStatus" gorm:"column:upload_status;not null;default:Completed"`
	// FailReason 失败原因
	FailReason *string `json:"FailReason" gorm:"column:fail_reason"`
	// TotalChunks 分片总数（仅视频文件有值）
	TotalChunks *int32 `json:"TotalChunks" gorm:"column:total_chunks"`
	// UploadedChunks 已上传分片数（仅视频文件有值）
	UploadedChunks *int32 `json:"UploadedChunks" gorm:"column:uploaded_chunks"`
	// UploadId 上传标识（分片上传用，关联初始化时生成的 ID）
	UploadId *string `json:"UploadId" gorm:"column:upload_id"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt" gorm:"column:created_at;not null"`
}

// CreateMaterialLibraryReq 创建素材库请求
type CreateMaterialLibraryReq struct {
	// Name 素材库名称
	Name string `json:"Name" binding:"required"`
	// Type 素材类型
	Type string `json:"Type" binding:"required,oneof=Image Video"`
	// Description 描述
	Description *string `json:"Description"`
}

// UpdateMaterialLibraryReq 更新素材库请求
type UpdateMaterialLibraryReq struct {
	// Name 素材库名称
	Name string `json:"Name" binding:"required"`
	// Description 描述
	Description *string `json:"Description"`
}

// InitVideoUploadReq 初始化视频上传请求
type InitVideoUploadReq struct {
	// FileName 视频文件名
	FileName string `json:"FileName" binding:"required"`
	// FileSize 文件总大小（字节）
	FileSize int64 `json:"FileSize" binding:"required,gt=0"`
	// ChunkSize 分片大小（字节）
	ChunkSize int32 `json:"ChunkSize" binding:"required,gt=0"`
}

// InitVideoUploadResp 初始化视频上传响应
type InitVideoUploadResp struct {
	// UploadId 上传标识
	UploadId string `json:"UploadId"`
	// ChunkCount 分片总数
	ChunkCount int32 `json:"ChunkCount"`
}

// UploadImageResp 批量上传图片响应
type UploadImageResp struct {
	// UploadedCount 成功上传数量
	UploadedCount int `json:"UploadedCount"`
	// Files 上传文件列表
	Files []MaterialFile `json:"Files"`
}

// CompleteVideoUploadReq 完成视频上传请求
type CompleteVideoUploadReq struct {
	// UploadId 上传标识
	UploadId string `json:"UploadId" binding:"required"`
}

// MaterialFileProgress 素材文件进度视图（用于响应中添加 Progress 字段）
type MaterialFileProgress struct {
	// Id 文件唯一标识
	Id string `json:"Id"`
	// LibraryId 所属素材库 ID
	LibraryId string `json:"LibraryId"`
	// FileName 原始文件名
	FileName string `json:"FileName"`
	// StoragePath 后端存储路径
	StoragePath string `json:"StoragePath"`
	// AccessUrl 浏览器访问 URL
	AccessUrl string `json:"AccessUrl"`
	// FileSize 文件大小（字节）
	FileSize int64 `json:"FileSize"`
	// MimeType MIME 类型
	MimeType string `json:"MimeType"`
	// UploadStatus 上传状态
	UploadStatus string `json:"UploadStatus"`
	// FailReason 失败原因
	FailReason *string `json:"FailReason"`
	// Progress 上传进度（0-1）
	Progress float64 `json:"Progress"`
	// TotalChunks 分片总数
	TotalChunks *int32 `json:"TotalChunks"`
	// UploadedChunks 已上传分片数
	UploadedChunks *int32 `json:"UploadedChunks"`
	// CreatedAt 创建时间
	CreatedAt string `json:"CreatedAt"`
}
