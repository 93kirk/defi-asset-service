package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置结构体
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Redis       RedisConfig       `mapstructure:"redis"`
	Cache       CacheConfig       `mapstructure:"cache"`
	External    ExternalConfig    `mapstructure:"external"`
	Queue       QueueConfig       `mapstructure:"queue"`
	Auth        AuthConfig        `mapstructure:"auth"`
	Monitoring  MonitoringConfig  `mapstructure:"monitoring"`
	Log         LogConfig         `mapstructure:"log"`
	Cron        CronConfig        `mapstructure:"cron"`
	Business    BusinessConfig    `mapstructure:"business"`
}

// ServerConfig 服务配置
type ServerConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	Mode           string `mapstructure:"mode"`
	ReadTimeout    int    `mapstructure:"read_timeout"`
	WriteTimeout   int    `mapstructure:"write_timeout"`
	MaxConnections int    `mapstructure:"max_connections"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	MySQL MySQLConfig `mapstructure:"mysql"`
}

// MySQLConfig MySQL配置
type MySQLConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	Charset         string `mapstructure:"charset"`
	ParseTime       bool   `mapstructure:"parse_time"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime int    `mapstructure:"conn_max_idle_time"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
	MaxRetries   int    `mapstructure:"max_retries"`
	DialTimeout  int    `mapstructure:"dial_timeout"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	PoolTimeout  int    `mapstructure:"pool_timeout"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	PositionTTL int `mapstructure:"position_ttl"`
	ProtocolTTL int `mapstructure:"protocol_ttl"`
	PriceTTL    int `mapstructure:"price_ttl"`
	ApyTTL      int `mapstructure:"apy_ttl"`
	EmptyTTL    int `mapstructure:"empty_ttl"`
}

// ExternalConfig 外部服务配置
type ExternalConfig struct {
	ServiceA ExternalServiceConfig `mapstructure:"service_a"`
	ServiceB ExternalServiceConfig `mapstructure:"service_b"`
	Debank   ExternalServiceConfig `mapstructure:"debank"`
}

// ExternalServiceConfig 外部服务配置
type ExternalServiceConfig struct {
	BaseURL    string `mapstructure:"base_url"`
	APIKey     string `mapstructure:"api_key"`
	Timeout    int    `mapstructure:"timeout"`
	MaxRetries int    `mapstructure:"max_retries"`
	RetryDelay int    `mapstructure:"retry_delay"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	PositionUpdates QueueStreamConfig `mapstructure:"position_updates"`
	DelayedTasks    QueueZSetConfig   `mapstructure:"delayed_tasks"`
}

// QueueStreamConfig Redis Stream队列配置
type QueueStreamConfig struct {
	StreamName    string `mapstructure:"stream_name"`
	ConsumerGroup string `mapstructure:"consumer_group"`
	ConsumerName  string `mapstructure:"consumer_name"`
	BatchSize     int    `mapstructure:"batch_size"`
	BlockTime     int    `mapstructure:"block_time"`
	MaxRetries    int    `mapstructure:"max_retries"`
	RetryDelay    int    `mapstructure:"retry_delay"`
}

// QueueZSetConfig Redis ZSet队列配置
type QueueZSetConfig struct {
	ZSetName     string `mapstructure:"zset_name"`
	PollInterval int    `mapstructure:"poll_interval"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret          string `mapstructure:"jwt_secret"`
	APIKeyHeader       string `mapstructure:"api_key_header"`
	JWTExpireHours     int    `mapstructure:"jwt_expire_hours"`
	RateLimitPerMinute int    `mapstructure:"rate_limit_per_minute"`
	RateLimitPerHour   int    `mapstructure:"rate_limit_per_hour"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Prometheus   PrometheusConfig   `mapstructure:"prometheus"`
	HealthCheck  HealthCheckConfig  `mapstructure:"health_check"`
	RequestLogging RequestLoggingConfig `mapstructure:"request_logging"`
}

// PrometheusConfig Prometheus配置
type PrometheusConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// RequestLoggingConfig 请求日志配置
type RequestLoggingConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Level   string `mapstructure:"level"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

// CronConfig 定时任务配置
type CronConfig struct {
	SyncProtocols CronJobConfig `mapstructure:"sync_protocols"`
	CleanupCache  CronJobConfig `mapstructure:"cleanup_cache"`
	UpdatePrices  CronJobConfig `mapstructure:"update_prices"`
}

// CronJobConfig 定时任务配置
type CronJobConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Schedule  string `mapstructure:"schedule"`
	BatchSize int    `mapstructure:"batch_size"`
}

// BusinessConfig 业务配置
type BusinessConfig struct {
	DefaultChainID        int       `mapstructure:"default_chain_id"`
	SupportedChains       []int     `mapstructure:"supported_chains"`
	MaxBatchAddresses     int       `mapstructure:"max_batch_addresses"`
	PositionRefreshThreshold float64 `mapstructure:"position_refresh_threshold"`
	FallbackEnabled       bool      `mapstructure:"fallback_enabled"`
	FallbackDataTTL       int       `mapstructure:"fallback_data_ttl"`
}

// LoadConfig 加载配置
func LoadConfig(configPath string) (*Config, error) {
	var config Config

	// 设置默认值
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "release")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.max_connections", 1000)

	// 从环境变量读取
	viper.AutomaticEnv()
	viper.SetEnvPrefix("DEFI")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 如果提供了配置文件路径，则从文件加载
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// 默认配置文件路径
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath(".")
	}

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件不存在，使用环境变量
			fmt.Println("Config file not found, using environment variables")
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// 绑定环境变量到配置结构
	bindEnvVars()

	// 解析配置到结构体
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// bindEnvVars 绑定环境变量到配置项
func bindEnvVars() {
	// 服务配置
	viper.BindEnv("server.host", "SERVER_HOST")
	viper.BindEnv("server.port", "SERVER_PORT")
	viper.BindEnv("server.mode", "SERVER_MODE")

	// 数据库配置
	viper.BindEnv("database.mysql.host", "DB_HOST")
	viper.BindEnv("database.mysql.port", "DB_PORT")
	viper.BindEnv("database.mysql.user", "DB_USER")
	viper.BindEnv("database.mysql.password", "DB_PASSWORD")
	viper.BindEnv("database.mysql.dbname", "DB_NAME")

	// Redis配置
	viper.BindEnv("redis.host", "REDIS_HOST")
	viper.BindEnv("redis.port", "REDIS_PORT")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("redis.db", "REDIS_DB")

	// 外部服务配置
	viper.BindEnv("external.service_a.api_key", "SERVICE_A_API_KEY")
	viper.BindEnv("external.service_b.api_key", "SERVICE_B_API_KEY")
	viper.BindEnv("external.debank.api_key", "DEBANK_API_KEY")

	// 认证配置
	viper.BindEnv("auth.jwt_secret", "JWT_SECRET")
	viper.BindEnv("auth.api_key_header", "API_KEY_HEADER")

	// 日志配置
	viper.BindEnv("log.level", "LOG_LEVEL")
	viper.BindEnv("log.format", "LOG_FORMAT")
	viper.BindEnv("log.output", "LOG_OUTPUT")

	// 监控配置
	viper.BindEnv("monitoring.prometheus.enabled", "PROMETHEUS_ENABLED")
	viper.BindEnv("monitoring.health_check.enabled", "HEALTH_CHECK_ENABLED")

	// 定时任务配置
	viper.BindEnv("cron.sync_protocols.enabled", "CRON_SYNC_PROTOCOLS_ENABLED")
	viper.BindEnv("cron.cleanup_cache.enabled", "CRON_CLEANUP_CACHE_ENABLED")
	viper.BindEnv("cron.update_prices.enabled", "CRON_UPDATE_PRICES_ENABLED")
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	// 验证服务配置
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.Server.Mode != "debug" && config.Server.Mode != "release" && config.Server.Mode != "test" {
		return fmt.Errorf("invalid server mode: %s", config.Server.Mode)
	}

	// 验证数据库配置
	if config.Database.MySQL.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if config.Database.MySQL.User == "" {
		return fmt.Errorf("database user is required")
	}

	// 验证Redis配置
	if config.Redis.Host == "" {
		return fmt.Errorf("redis host is required")
	}

	// 验证认证配置
	if config.Auth.JWTSecret == "" {
		return fmt.Errorf("jwt secret is required")
	}

	// 验证业务配置
	if config.Business.DefaultChainID <= 0 {
		return fmt.Errorf("default chain id must be positive")
	}

	if len(config.Business.SupportedChains) == 0 {
		return fmt.Errorf("supported chains cannot be empty")
	}

	return nil
}

// GetConfigPath 获取配置文件路径
func GetConfigPath() string {
	// 检查环境变量
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}

	// 检查当前目录
	configPaths := []string{
		"./configs/config.yaml",
		"./config.yaml",
		"/etc/defi-asset-service/config.yaml",
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	return ""
}