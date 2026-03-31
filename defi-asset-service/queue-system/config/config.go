package config

import (
	"time"
)

// Redis配置
type RedisConfig struct {
	Address  string `yaml:"address" env:"REDIS_ADDRESS" env-default:"localhost:6379"`
	Password string `yaml:"password" env:"REDIS_PASSWORD" env-default:""`
	DB       int    `yaml:"db" env:"REDIS_DB" env-default:"3"` // 队列使用DB 3
	PoolSize int    `yaml:"pool_size" env:"REDIS_POOL_SIZE" env-default:"100"`
}

// 队列配置
type QueueConfig struct {
	// 主队列配置
	StreamName          string        `yaml:"stream_name" env:"QUEUE_STREAM_NAME" env-default:"defi:stream:position_updates"`
	ConsumerGroup       string        `yaml:"consumer_group" env:"QUEUE_CONSUMER_GROUP" env-default:"position_workers"`
	ConsumerName        string        `yaml:"consumer_name" env:"QUEUE_CONSUMER_NAME" env-default:"worker"`
	
	// 死信队列配置
	DLQStreamName       string        `yaml:"dlq_stream_name" env:"QUEUE_DLQ_STREAM_NAME" env-default:"defi:stream:dlq:position_updates"`
	MaxRetries          int           `yaml:"max_retries" env:"QUEUE_MAX_RETRIES" env-default:"3"`
	RetryDelay          time.Duration `yaml:"retry_delay" env:"QUEUE_RETRY_DELAY" env-default:"30s"`
	
	// 消费者配置
	BatchSize           int64         `yaml:"batch_size" env:"QUEUE_BATCH_SIZE" env-default:"10"`
	BlockTimeout        time.Duration `yaml:"block_timeout" env:"QUEUE_BLOCK_TIMEOUT" env-default:"5s"`
	AutoAck             bool          `yaml:"auto_ack" env:"QUEUE_AUTO_ACK" env-default:"false"`
	
	// 监控配置
	MetricsEnabled      bool          `yaml:"metrics_enabled" env:"QUEUE_METRICS_ENABLED" env-default:"true"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" env:"QUEUE_HEALTH_CHECK_INTERVAL" env-default:"30s"`
}

// MySQL配置
type MySQLConfig struct {
	Host     string `yaml:"host" env:"MYSQL_HOST" env-default:"localhost"`
	Port     int    `yaml:"port" env:"MYSQL_PORT" env-default:"3306"`
	User     string `yaml:"user" env:"MYSQL_USER" env-default:"root"`
	Password string `yaml:"password" env:"MYSQL_PASSWORD" env-default:""`
	Database string `yaml:"database" env:"MYSQL_DATABASE" env-default:"defi_asset_service"`
	Charset  string `yaml:"charset" env:"MYSQL_CHARSET" env-default:"utf8mb4"`
}

// 应用配置
type AppConfig struct {
	Redis  RedisConfig  `yaml:"redis"`
	Queue  QueueConfig  `yaml:"queue"`
	MySQL  MySQLConfig  `yaml:"mysql"`
	
	// 应用配置
	LogLevel string `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	Workers  int    `yaml:"workers" env:"WORKERS" env-default:"5"`
}