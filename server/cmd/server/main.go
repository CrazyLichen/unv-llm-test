package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/common"
	"llm-test-server/internal/config"
	"llm-test-server/internal/controller"
	"llm-test-server/internal/llm"
	"llm-test-server/internal/repository"
	"llm-test-server/internal/service"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// main 程序入口
func main() {
	// 加载配置
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志（在 DB 之前，日志初始化失败仍用标准 log）
	if err := common.InitLogger(&cfg.Log); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	slog.Info("配置加载成功", "port", cfg.Server.Port, "mode", cfg.Server.Mode)

	// 初始化数据库
	db, err := repository.InitDB(&cfg.Database)
	if err != nil {
		slog.Error("初始化数据库失败", "error", err)
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 关闭底层数据库连接
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("获取底层数据库连接失败", "error", err)
		log.Fatalf("获取底层数据库连接失败: %v", err)
	}
	defer sqlDB.Close()

	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 初始化各层
	mcRepo := repository.NewModelConfigRepo(db)

	// 初始化 LLM 模块
	llmFactory := llm.NewClientFactory()
	llmClient := llm.NewLLMClient(llmFactory)

	mcSvc := service.NewModelConfigService(mcRepo, llmClient)
	mcCtrl := controller.NewModelConfigController(mcSvc)

	mlRepo := repository.NewMaterialLibraryRepo(db)
	mlSvc := service.NewMaterialLibraryService(mlRepo, &cfg.Upload)
	mlCtrl := controller.NewMaterialLibraryController(mlSvc)

	// 注册路由
	r := gin.Default()
	controller.SetupRouter(r, mcCtrl, mlCtrl, cfg.Upload.Dir)

	// 启动服务
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	slog.Info("服务启动", "addr", addr)
	if err := r.Run(addr); err != nil {
		slog.Error("服务启动失败", "error", err)
		log.Fatalf("服务启动失败: %v", err)
	}
}
