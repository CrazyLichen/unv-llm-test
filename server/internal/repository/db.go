package repository

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"llm-test-server/internal/config"
	"llm-test-server/internal/model"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// InitDB 初始化数据库连接并自动迁移表结构
func InitDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	// 确保 DSN 的父目录存在
	dir := filepath.Dir(cfg.DSN)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	// 使用 glebarez/sqlite 作为 GORM 的 SQLite Dialector
	db, err := gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// SQLite 写入是单写锁，限制最大打开连接数
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层数据库连接失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// 自动迁移表结构
	if err := db.AutoMigrate(&model.ModelConfig{}, &model.MaterialLibrary{}, &model.MaterialFile{}); err != nil {
		return nil, fmt.Errorf("自动迁移表结构失败: %w", err)
	}

	slog.Info("数据库初始化成功", "driver", cfg.Driver, "dsn", cfg.DSN)
	return db, nil
}
