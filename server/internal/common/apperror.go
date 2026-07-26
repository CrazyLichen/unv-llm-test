package common

import "fmt"

// ──────────────────────────── 结构体 ────────────────────────────

// AppError 业务错误，携带错误码和消息
type AppError struct {
	// Code 错误码
	Code int
	// Message 错误消息
	Message string
	// Detail 详细错误信息（可选）
	Detail string
}

// ──────────────────────────── 常量 ────────────────────────────

// 错误码按领域 1000 递增分段：
// 0xxx 通用
// 1xxx 模型配置领域
// 2xxx 素材库领域
// 3xxx 任务领域
// 5xxx LLM 领域
// 6xxx 文件处理领域
const (
	// ──────── 通用 (0xxx) ────────
	// ErrCodeParamValidation 参数校验失败错误码
	ErrCodeParamValidation = 1
	// ErrCodeServerInternal 内部服务错误码
	ErrCodeServerInternal = 2

	// ──────── 模型配置领域 (1xxx) ────────
	// ErrCodeModelConfigNotFound 模型配置不存在错误码
	ErrCodeModelConfigNotFound = 1001
	// ErrCodeModelConfigBoundByTask 模型配置已被任务关联错误码
	ErrCodeModelConfigBoundByTask = 1002

	// ──────── 素材库领域 (2xxx) ────────
	// ErrCodeMaterialLibNotFound 素材库不存在错误码
	ErrCodeMaterialLibNotFound = 2001
	// ErrCodeMaterialLibBound 素材库已被任务关联错误码
	ErrCodeMaterialLibBound = 2002
	// ErrCodeLibTypeMismatch 素材库类型不匹配错误码
	ErrCodeLibTypeMismatch = 2003
	// ErrCodeMaterialFileNotFound 素材文件不存在错误码
	ErrCodeMaterialFileNotFound = 2004
	// ErrCodeFileUploadIncomplete 文件上传未完成错误码
	ErrCodeFileUploadIncomplete = 2005

	// ──────── 任务领域 (3xxx) ────────
	// ErrCodeTaskNotFound 任务不存在错误码
	ErrCodeTaskNotFound = 3001
	// ErrCodeTaskStatusInvalid 任务状态不允许此操作错误码
	ErrCodeTaskStatusInvalid = 3002

	// ──────── LLM 领域 (5xxx) ────────
	// ErrCodeLLMCallFailed 大模型调用失败错误码
	ErrCodeLLMCallFailed = 5001

	// ──────── 文件处理领域 (6xxx) ────────
	// ErrCodeVideoFrameFailed 视频抽帧失败错误码
	ErrCodeVideoFrameFailed = 6001
	// ErrCodeFileUploadFailed 文件上传失败错误码
	ErrCodeFileUploadFailed = 6002
	// ErrCodeChunkUploadFailed 分片上传异常错误码
	ErrCodeChunkUploadFailed = 6003
)

// ──────────────────────────── 全局变量 ────────────────────────────

var (
	// ──────── 通用 (0xxx) ────────
	// ErrParamValidation 参数校验失败
	ErrParamValidation = AppError{Code: ErrCodeParamValidation, Message: "参数校验失败"}

	// ──────── 模型配置领域 (1xxx) ────────
	// ErrModelConfigNotFound 模型配置不存在
	ErrModelConfigNotFound = AppError{Code: ErrCodeModelConfigNotFound, Message: "模型配置不存在"}
	// ErrModelConfigBoundByTask 模型配置已被任务关联
	ErrModelConfigBoundByTask = AppError{Code: ErrCodeModelConfigBoundByTask, Message: "模型配置已被任务关联"}

	// ──────── 素材库领域 (2xxx) ────────
	// ErrMaterialLibNotFound 素材库不存在
	ErrMaterialLibNotFound = AppError{Code: ErrCodeMaterialLibNotFound, Message: "素材库不存在"}
	// ErrMaterialLibBound 素材库已被任务关联
	ErrMaterialLibBound = AppError{Code: ErrCodeMaterialLibBound, Message: "素材库已被任务关联"}
	// ErrLibTypeMismatch 素材库类型不匹配
	ErrLibTypeMismatch = AppError{Code: ErrCodeLibTypeMismatch, Message: "素材库类型与任务类型不匹配"}
	// ErrMaterialFileNotFound 素材文件不存在
	ErrMaterialFileNotFound = AppError{Code: ErrCodeMaterialFileNotFound, Message: "素材文件不存在"}
	// ErrFileUploadIncomplete 文件上传未完成
	ErrFileUploadIncomplete = AppError{Code: ErrCodeFileUploadIncomplete, Message: "文件上传未完成"}

	// ──────── 任务领域 (3xxx) ────────
	// ErrTaskNotFound 任务不存在
	ErrTaskNotFound = AppError{Code: ErrCodeTaskNotFound, Message: "任务不存在"}
	// ErrTaskStatusInvalid 任务状态不允许此操作
	ErrTaskStatusInvalid = AppError{Code: ErrCodeTaskStatusInvalid, Message: "任务状态不允许此操作"}

	// ──────── LLM 领域 (5xxx) ────────
	// ErrLLMCallFailed 大模型调用失败
	ErrLLMCallFailed = AppError{Code: ErrCodeLLMCallFailed, Message: "大模型调用失败"}

	// ──────── 文件处理领域 (6xxx) ────────
	// ErrVideoFrameFailed 视频抽帧失败
	ErrVideoFrameFailed = AppError{Code: ErrCodeVideoFrameFailed, Message: "视频抽帧失败"}
	// ErrFileUploadFailed 文件上传失败
	ErrFileUploadFailed = AppError{Code: ErrCodeFileUploadFailed, Message: "文件上传失败"}
	// ErrChunkUploadFailed 分片上传异常
	ErrChunkUploadFailed = AppError{Code: ErrCodeChunkUploadFailed, Message: "分片上传异常"}
)

// ──────────────────────────── 导出函数 ────────────────────────────

// Error 实现 error 接口
func (e AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s（%s）", e.Message, e.Detail)
	}
	return e.Message
}

// NewErrParamValidation 创建参数校验失败错误（带详细信息）
func NewErrParamValidation(detail string) AppError {
	return AppError{Code: ErrCodeParamValidation, Message: "参数校验失败", Detail: detail}
}

// NewErrLLMCallFailed 创建大模型调用失败错误（带详细信息）
func NewErrLLMCallFailed(detail string) AppError {
	return AppError{Code: ErrCodeLLMCallFailed, Message: "大模型调用失败", Detail: detail}
}

// NewErrVideoFrameFailed 创建视频抽帧失败错误（带详细信息）
func NewErrVideoFrameFailed(detail string) AppError {
	return AppError{Code: ErrCodeVideoFrameFailed, Message: "视频抽帧失败", Detail: detail}
}

// NewErrFileUploadFailed 创建文件上传失败错误（带详细信息）
func NewErrFileUploadFailed(detail string) AppError {
	return AppError{Code: ErrCodeFileUploadFailed, Message: "文件上传失败", Detail: detail}
}

// NewErrChunkUploadFailed 创建分片上传异常错误（带详细信息）
func NewErrChunkUploadFailed(detail string) AppError {
	return AppError{Code: ErrCodeChunkUploadFailed, Message: "分片上传异常", Detail: detail}
}
