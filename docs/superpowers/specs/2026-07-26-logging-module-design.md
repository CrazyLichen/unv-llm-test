# 日志模块设计

> 日期：2026-07-26
> 范围：增加 slog 日志模块，配置驱动，补充现有代码日志

---

## 1. 技术选型

| 项目 | 选择 | 理由 |
|------|------|------|
| 日志库 | log/slog | Go 1.21+ 内置，零依赖，支持结构化、级别、JSON/Text 格式 |

## 2. 配置扩展

### config.yaml 新增 file 字段

```yaml
log:
  level: info             # debug / info / warn / error
  format: json            # json / text
  file: "./data/app.log"  # 日志文件路径，空则仅控制台
```

### LogConfig 新增字段

```go
type LogConfig struct {
    Level  string `yaml:"level"`
    Format string `yaml:"format"`
    File   string `yaml:"file"`
}
```

- `File` 默认值为 `""`（仅控制台输出）
- 环境变量 `LOG_FILE` 可覆盖

## 3. 日志初始化

### 新增文件：common/logger.go

导出函数 `InitLogger(cfg *config.LogConfig) error`：

1. 解析 `Level` → `slog.Level`（debug=Debug, info=Info, warn=Warn, error=Error）
2. 根据 `Format` 选择 Handler：
   - `"json"` → `slog.NewJSONHandler(writer, opts)`
   - `"text"` → `slog.NewTextHandler(writer, opts)`
3. 根据 `File` 选择 Writer：
   - 非空 → `os.OpenFile(file, O_CREATE|O_WRONLY|O_APPEND, 0644)`，然后用 `io.MultiWriter(os.Stdout, file)` 双输出
   - 空 → 仅 `os.Stdout`
4. 调用 `slog.SetDefault(logger)` 设置全局默认 Logger
5. 返回 error（文件创建失败时）

### main.go 调用顺序

```go
cfg, _ := config.Load("config.yaml")
common.InitLogger(&cfg.Log)  // 在 DB 初始化之前
slog.Info("配置加载成功")
```

## 4. 各层日志补充策略

| 层 | 日志内容 | 级别 |
|---|---------|------|
| main | 启动信息、关闭信息、致命错误 | Info/Error |
| repository | DB 操作结果（成功/失败）、错误详情 | Info/Error |
| service | 业务关键节点：创建配置、删除检查、连通性测试结果 | Info/Warn/Error |
| controller | 请求入口（方法+路径）、参数校验失败 | Info/Warn |

### 具体日志点

**main.go**：
- `slog.Info("服务启动", "port", cfg.Server.Port, "mode", cfg.Server.Mode)`
- 致命错误保留 `log.Fatalf`（slog 初始化失败时仍需标准 log）

**repository/model_config_repo.go**：
- Create 成功：`slog.Info("创建模型配置", "id", mc.Id)`
- GetByID 未找到：`slog.Warn("模型配置不存在", "id", id)`
- List 查询：`slog.Info("查询模型配置列表", "page", page, "pageSize", pageSize, "total", total)`
- Update 成功：`slog.Info("更新模型配置", "id", id)`
- Delete 成功：`slog.Info("删除模型配置", "id", id)`
- 所有 fmt.Errorf 的错误：`slog.Error("操作失败", "op", "xxx", "error", err)`

**service/model_config_svc.go**：
- Create 成功：`slog.Info("创建模型配置成功", "id", id, "modelId", req.ModelId)`
- Delete 关联任务检查：`slog.Warn("删除模型配置被拒绝，存在关联任务", "id", id)`
- TestConnectivity 结果：`slog.Info("模型连通性测试", "id", id, "latency", elapsed, "success", true/false)`

**controller/model_config_ctrl.go**：
- 每个请求入口：`slog.Info("收到请求", "method", c.Request.Method, "path", c.Request.URL.Path)`
- 参数校验失败：`slog.Warn("参数校验失败", "error", err.Error())`

## 5. 日志格式示例

JSON 格式输出：
```json
{"time":"2026-07-26T14:30:00.123Z","level":"INFO","msg":"创建模型配置","id":"mc_gpt-4o_abc123","modelId":"gpt-4o"}
```

Text 格式输出：
```
time=2026-07-26T14:30:00.123Z level=INFO msg="创建模型配置" id=mc_gpt-4o_abc123 modelId=gpt-4o
```
