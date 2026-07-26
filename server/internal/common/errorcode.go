package common

// ──────────────────────────── 常量 ────────────────────────────

const (
	// Success 成功
	Success = 0
	// ErrParamInvalid 参数校验失败
	ErrParamInvalid = 40001
	// ErrTaskNotFound 任务不存在
	ErrTaskNotFound = 40002
	// ErrImageNotFound 素材不存在
	ErrImageNotFound = 40003
	// ErrTaskStatusConflict 任务状态不允许此操作
	ErrTaskStatusConflict = 40005
	// ErrModelConfigNotFound 模型配置不存在
	ErrModelConfigNotFound = 40006
	// ErrLibraryNotFound 素材库不存在
	ErrLibraryNotFound = 40007
	// ErrLibraryAlreadyBound 素材库已被任务关联
	ErrLibraryAlreadyBound = 40008
	// ErrLibraryTypeMismatch 素材库类型不匹配
	ErrLibraryTypeMismatch = 40009
	// ErrFileNotFound 文件不存在
	ErrFileNotFound = 40010
	// ErrFileUploadIncomplete 文件上传未完成
	ErrFileUploadIncomplete = 40011
	// ErrModelCallFailed 大模型调用失败
	ErrModelCallFailed = 50001
	// ErrVideoFrameFailed 视频抽帧失败
	ErrVideoFrameFailed = 50002
	// ErrFileUploadFailed 文件上传失败
	ErrFileUploadFailed = 50003
	// ErrChunkUploadFailed 分片上传异常
	ErrChunkUploadFailed = 50004
)
