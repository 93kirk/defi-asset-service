package config

import (
	"fmt"
	"os"
	"path/filepath"
	
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*AppConfig, error) {
	// 加载环境变量文件
	_ = godotenv.Load()
	
	// 创建默认配置
	config := &AppConfig{}
	
	// 如果提供了配置文件路径，从YAML加载
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}
	
	// 从环境变量覆盖配置
	loadFromEnv(config)
	
	return config, nil
}

// loadFromEnv 从环境变量加载配置
func loadFromEnv(config *AppConfig) {
	// Redis配置
	if addr := os.Getenv("REDIS_ADDRESS"); addr != "" {
		config.Redis.Address = addr
	}
	if pass := os.Getenv("REDIS_PASSWORD"); pass != "" {
		config.Redis.Password = pass
	}
	if db := os.Getenv("REDIS_DB"); db != "" {
		fmt.Sscanf(db, "%d", &config.Redis.DB)
	}
	if poolSize := os.Getenv("REDIS_POOL_SIZE"); poolSize != "" {
		fmt.Sscanf(poolSize, "%d", &config.Redis.PoolSize)
	}
	
	// 队列配置
	if streamName := os.Getenv("QUEUE_STREAM_NAME"); streamName != "" {
		config.Queue.StreamName = streamName
	}
	if consumerGroup := os.Getenv("QUEUE_CONSUMER_GROUP"); consumerGroup != "" {
		config.Queue.ConsumerGroup = consumerGroup
	}
	if consumerName := os.Getenv("QUEUE_CONSUMER_NAME"); consumerName != "" {
		config.Queue.ConsumerName = consumerName
	}
	if dlqStreamName := os.Getenv("QUEUE_DLQ_STREAM_NAME"); dlqStreamName != "" {
		config.Queue.DLQStreamName = dlqStreamName
	}
	if maxRetries := os.Getenv("QUEUE_MAX_RETRIES"); maxRetries != "" {
		fmt.Sscanf(maxRetries, "%d", &config.Queue.MaxRetries)
	}
	if retryDelay := os.Getenv("QUEUE_RETRY_DELAY"); retryDelay != "" {
		if d, err := time.ParseDuration(retryDelay); err == nil {
			config.Queue.RetryDelay = d
		}
	}
	if batchSize := os.Getenv("QUEUE_BATCH_SIZE"); batchSize != "" {
		fmt.Sscanf(batchSize, "%d", &config.Queue.BatchSize)
	}
	if blockTimeout := os.Getenv("QUEUE_BLOCK_TIMEOUT"); blockTimeout != "" {
		if d, err := time.ParseDuration(blockTimeout); err == nil {
			config.Queue.BlockTimeout = d
		}
	}
	
	// MySQL配置
	if host := os.Getenv("MYSQL_HOST"); host != "" {
		config.MySQL.Host = host
	}
	if port := os.Getenv("MYSQL_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &config.MySQL.Port)
	}
	if user := os.Getenv("MYSQL_USER"); user != "" {
		config.MySQL.User = user
	}
	if pass := os.Getenv("MYSQL_PASSWORD"); pass != "" {
		config.MySQL.Password = pass
	}
	if db := os.Getenv("MYSQL_DATABASE"); db != "" {
		config.MySQL.Database = db
	}
	if charset := os.Getenv("MYSQL_CHARSET"); charset != "" {
		config.MySQL.Charset = charset
	}
	
	// 应用配置
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}
	if workers := os.Getenv("WORKERS"); workers != "" {
		fmt.Sscanf(workers, "%d", &config.Workers)
	}
}

// GetDefaultConfigPath 获取默认配置文件路径
func GetDefaultConfigPath() string {
	// 尝试多个可能的配置文件位置
	paths := []string{
		"config.yaml",
		"config/config.yaml",
		"/etc/defi-queue/config.yaml",
		filepath.Join(os.Getenv("HOME"), ".defi-queue/config.yaml"),
	}
	
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	return ""
}