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
	// Upload 上传配置
	Upload UploadConfig `yaml:"upload"`
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
	// Format 日志格式（json/text）
	Format string `yaml:"format"`
	// File 日志文件路径，为空则仅输出到控制台
	File string `yaml:"file"`
}

// UploadConfig 上传配置
type UploadConfig struct {
	// Dir 上传文件存储目录
	Dir string `yaml:"dir"`
	// MaxImageSize 单个图片文件最大大小（字节）
	MaxImageSize int64 `yaml:"max_image_size"`
	// MaxImageCount 单次最多上传图片数量
	MaxImageCount int `yaml:"max_image_count"`
	// MaxImageBatchSize 单次上传请求总体积最大值（字节）
	MaxImageBatchSize int64 `yaml:"max_image_batch_size"`
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
	if cfg.Log.File == "" {
		cfg.Log.File = ""
	}
	if cfg.Upload.Dir == "" {
		cfg.Upload.Dir = "./data/uploads"
	}
	if cfg.Upload.MaxImageSize == 0 {
		cfg.Upload.MaxImageSize = 10 * 1024 * 1024 // 10MB
	}
	if cfg.Upload.MaxImageCount == 0 {
		cfg.Upload.MaxImageCount = 20
	}
	if cfg.Upload.MaxImageBatchSize == 0 {
		cfg.Upload.MaxImageBatchSize = 50 * 1024 * 1024 // 50MB
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
	if v := os.Getenv("LOG_FILE"); v != "" {
		cfg.Log.File = v
	}

	return &cfg, nil
}
