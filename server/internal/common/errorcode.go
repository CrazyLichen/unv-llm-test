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
	// ErrPathNotFound 路径不存在
	ErrPathNotFound = 40004
	// ErrTaskStatusConflict 任务状态不允许此操作
	ErrTaskStatusConflict = 40005
	// ErrModelConfigNotFound 模型配置不存在
	ErrModelConfigNotFound = 40006
	// ErrModelCallFailed 大模型调用失败
	ErrModelCallFailed = 50001
	// ErrVideoFrameFailed 视频抽帧失败
	ErrVideoFrameFailed = 50002
)
