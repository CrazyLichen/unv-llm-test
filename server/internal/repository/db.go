package repository

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"llm-test-server/internal/config"
)

// ──────────────────────────── 导出函数 ────────────────────────────

// InitDB 初始化数据库连接并建表
func InitDB(cfg *config.DatabaseConfig) (*sql.DB, error) {
	// 确保 DSN 的父目录存在
	dir := filepath.Dir(cfg.DSN)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// SQLite 写入是单写锁，限制最大打开连接数
	db.SetMaxOpenConns(1)

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("创建数据表失败: %w", err)
	}

	return db, nil
}

// ──────────────────────────── 非导出函数 ────────────────────────────

// createTables 执行建表 SQL
func createTables(db *sql.DB) error {
	const sql = `
	CREATE TABLE IF NOT EXISTS model_configs (
		id          TEXT PRIMARY KEY,
		model_name  TEXT NOT NULL,
		model_id    TEXT NOT NULL,
		api_url     TEXT NOT NULL,
		api_key     TEXT NOT NULL,
		temperature REAL NOT NULL DEFAULT 0.7,
		max_tokens  INTEGER NOT NULL DEFAULT 4096,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	);`

	_, err := db.Exec(sql)
	return err
}
