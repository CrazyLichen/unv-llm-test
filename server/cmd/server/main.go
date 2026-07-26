package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"llm-test-server/internal/config"
	"llm-test-server/internal/controller"
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

	// 初始化数据库
	db, err := repository.InitDB(&cfg.Database)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 初始化各层
	mcRepo := repository.NewModelConfigRepo(db)
	mcSvc := service.NewModelConfigService(mcRepo)
	mcCtrl := controller.NewModelConfigController(mcSvc)

	// 注册路由
	r := gin.Default()
	controller.SetupRouter(r, mcCtrl)

	// 启动服务
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("服务启动于 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
