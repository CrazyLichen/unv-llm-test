package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// ──────────────────────────── 结构体 ────────────────────────────

// Config 应用配置
type Config struct {
	// Server 服务配置
	Server ServerConfig `yaml:"server"`
	// Database 数据库配置
	Database DatabaseConfig `yaml:"database"`
	// Log 日志配置
	Log LogConfig `yaml:"log"`
}

// ServerConfig 服务配置
type ServerConfig struct {
	// Port 服务端口
	Port int `yaml:"port"`
	// Mode 运行模式（debug/release）
	Mode string `yaml:"mode"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	// Driver 数据库驱动
	Driver string `yaml:"driver"`
	// DSN 数据源名称
	DSN string `yaml:"dsn"`
}

// LogConfig 日志配置
type LogConfig struct {
	// Level 日志级别
	Level string `yaml:"level"`
	// Format 日志格式
	Format string `yaml:"format"`
}

// ──────────────────────────── 导出函数 ────────────────────────────

// Load 从 YAML 文件加载配置，环境变量可覆盖
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "release"
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "sqlite"
	}
	if cfg.Database.DSN == "" {
		cfg.Database.DSN = "./data/llm-test.db"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}

	// 环境变量覆盖
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("SERVER_MODE"); v != "" {
		cfg.Server.Mode = v
	}
	if v := os.Getenv("DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}

	return &cfg, nil
}
