package common

// ──────────────────────────── 常量 ────────────────────────────

const (
	// Success 成功
	Success = 0

	// 以下为旧版错误码常量，映射到新版分段错误码，保持向后兼容
	// 新代码应使用 apperror.go 中的 ErrCodeXXX 常量和 ErrXXX 变量

	// LegacyErrParamInvalid 参数校验失败（旧版）
	LegacyErrParamInvalid = ErrCodeParamValidation
	// LegacyErrTaskNotFound 任务不存在（旧版）
	LegacyErrTaskNotFound = ErrCodeTaskNotFound
	// LegacyErrImageNotFound 素材不存在（旧版）
	LegacyErrImageNotFound = ErrCodeMaterialFileNotFound
	// LegacyErrTaskStatusConflict 任务状态不允许此操作（旧版）
	LegacyErrTaskStatusConflict = ErrCodeTaskStatusInvalid
	// LegacyErrModelConfigNotFound 模型配置不存在（旧版）
	LegacyErrModelConfigNotFound = ErrCodeModelConfigNotFound
	// LegacyErrLibraryNotFound 素材库不存在（旧版）
	LegacyErrLibraryNotFound = ErrCodeMaterialLibNotFound
	// LegacyErrLibraryAlreadyBound 素材库已被任务关联（旧版）
	LegacyErrLibraryAlreadyBound = ErrCodeMaterialLibBound
	// LegacyErrLibraryTypeMismatch 素材库类型不匹配（旧版）
	LegacyErrLibraryTypeMismatch = ErrCodeLibTypeMismatch
	// LegacyErrFileNotFound 文件不存在（旧版）
	LegacyErrFileNotFound = ErrCodeMaterialFileNotFound
	// LegacyErrFileUploadIncomplete 文件上传未完成（旧版）
	LegacyErrFileUploadIncomplete = ErrCodeFileUploadIncomplete
	// LegacyErrModelCallFailed 大模型调用失败（旧版）
	LegacyErrModelCallFailed = ErrCodeLLMCallFailed
	// LegacyErrVideoFrameFailed 视频抽帧失败（旧版）
	LegacyErrVideoFrameFailed = ErrCodeVideoFrameFailed
	// LegacyErrFileUploadFailed 文件上传失败（旧版）
	LegacyErrFileUploadFailed = ErrCodeFileUploadFailed
	// LegacyErrChunkUploadFailed 分片上传异常（旧版）
	LegacyErrChunkUploadFailed = ErrCodeChunkUploadFailed
)
