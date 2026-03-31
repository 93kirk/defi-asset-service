package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	External ExternalConfig `yaml:"external"`
	Sync     SyncConfig     `yaml:"sync"`
	Cache    CacheConfig    `yaml:"cache"`
	Queue    QueueConfig    `yaml:"queue"`
	Log      LogConfig      `yaml:"log"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Alert     AlertConfig   `yaml:"alert"`
	Security  SecurityConfig `yaml:"security"`
	Development DevelopmentConfig `yaml:"development"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Environment string `yaml:"environment"`
	Debug       bool   `yaml:"debug"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	HTTPPort     int           `yaml:"http_port"`
	GRPCPort     int           `yaml:"grpc_port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	MySQL MySQLConfig `yaml:"mysql"`
	Redis RedisConfig `yaml:"redis"`
}

// MySQLConfig MySQL配置
type MySQLConfig struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	Username          string        `yaml:"username"`
	Password          string        `yaml:"password"`
	Database          string        `yaml:"database"`
	Charset           string        `yaml:"charset"`
	ParseTime         bool          `yaml:"parse_time"`
	MaxOpenConns      int           `yaml:"max_open_conns"`
	MaxIdleConns      int           `yaml:"max_idle_conns"`
	ConnMaxLifetime   time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime   time.Duration `yaml:"conn_max_idle_time"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host          string        `yaml:"host"`
	Port          int           `yaml:"port"`
	Password      string        `yaml:"password"`
	DB            int           `yaml:"db"`
	PoolSize      int           `yaml:"pool_size"`
	MinIdleConns  int           `yaml:"min_idle_conns"`
	DialTimeout   time.Duration `yaml:"dial_timeout"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
	PoolTimeout   time.Duration `yaml:"pool_timeout"`
}

// ExternalConfig 外部服务配置
type ExternalConfig struct {
	Debank   DebankConfig   `yaml:"debank"`
	ServiceA ServiceConfig  `yaml:"service_a"`
	ServiceB ServiceConfig  `yaml:"service_b"`
}

// DebankConfig DeBank配置
type DebankConfig struct {
	BaseURL    string        `yaml:"base_url"`
	Timeout    time.Duration `yaml:"timeout"`
	UserAgent  string        `yaml:"user_agent"`
	MaxRetries int           `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`
	RateLimit  int           `yaml:"rate_limit"`
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	BaseURL    string        `yaml:"base_url"`
	APIKey     string        `yaml:"api_key"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxRetries int           `yaml:"max_retries"`
}

// SyncConfig 同步配置
type SyncConfig struct {
	ProtocolMetadata SyncTaskConfig `yaml:"protocol_metadata"`
	ProtocolTokens   SyncTaskConfig `yaml:"protocol_tokens"`
	UserPositions    SyncTaskConfig `yaml:"user_positions"`
	Incremental      IncrementalConfig `yaml:"incremental"`
}

// SyncTaskConfig 同步任务配置
type SyncTaskConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Schedule    string        `yaml:"schedule"`
	FullSyncDays int          `yaml:"full_sync_days"`
	BatchSize   int           `yaml:"batch_size"`
	Concurrency int           `yaml:"concurrency"`
	Timeout     time.Duration `yaml:"timeout"`
}

// IncrementalConfig 增量同步配置
type IncrementalConfig struct {
	Enabled      bool          `yaml:"enabled"`
	CheckInterval time.Duration `yaml:"check_interval"`
	MaxBackoff   time.Duration `yaml:"max_backoff"`
	RetryCount   int           `yaml:"retry_count"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Redis  RedisCacheConfig  `yaml:"redis"`
	Memory MemoryCacheConfig `yaml:"memory"`
}

// RedisCacheConfig Redis缓存配置
type RedisCacheConfig struct {
	DefaultTTL   time.Duration `yaml:"default_ttl"`
	PositionTTL  time.Duration `yaml:"position_ttl"`
	ProtocolTTL  time.Duration `yaml:"protocol_ttl"`
	TokenTTL     time.Duration `yaml:"token_ttl"`
}

// MemoryCacheConfig 内存缓存配置
type MemoryCacheConfig struct {
	Enabled         bool          `yaml:"enabled"`
	DefaultSize     int           `yaml:"default_size"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	RedisStream RedisStreamConfig `yaml:"redis_stream"`
	DeadLetter  DeadLetterConfig  `yaml:"dead_letter"`
}

// RedisStreamConfig Redis Stream配置
type RedisStreamConfig struct {
	Enabled       bool          `yaml:"enabled"`
	StreamName    string        `yaml:"stream_name"`
	ConsumerGroup string        `yaml:"consumer_group"`
	ConsumerName  string        `yaml:"consumer_name"`
	MaxLen        int64         `yaml:"max_len"`
	BlockTimeout  time.Duration `yaml:"block_timeout"`
	ClaimMinIdle  time.Duration `yaml:"claim_min_idle"`
}

// DeadLetterConfig 死信队列配置
type DeadLetterConfig struct {
	Enabled    bool          `yaml:"enabled"`
	StreamName string        `yaml:"stream_name"`
	MaxRetries int           `yaml:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Prometheus  PrometheusConfig  `yaml:"prometheus"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
	Tracing     TracingConfig     `yaml:"tracing"`
}

// PrometheusConfig Prometheus配置
type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
	Port    int    `yaml:"port"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Path     string        `yaml:"path"`
	Interval time.Duration `yaml:"interval"`
}

// TracingConfig 追踪配置
type TracingConfig struct {
	Enabled        bool    `yaml:"enabled"`
	JaegerEndpoint string  `yaml:"jaeger_endpoint"`
	SampleRate     float64 `yaml:"sample_rate"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	ErrorRate    AlertThreshold `yaml:"error_rate"`
	SyncLatency  AlertThreshold `yaml:"sync_latency"`
	QueueBacklog AlertThreshold `yaml:"queue_backlog"`
}

// AlertThreshold 告警阈值配置
type AlertThreshold struct {
	Enabled  bool          `yaml:"enabled"`
	Threshold interface{}  `yaml:"threshold"` // 可以是float64, int, 或time.Duration
	Window   time.Duration `yaml:"window"`
	Cooldown time.Duration `yaml:"cooldown"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	APIAuth   APIAuthConfig   `yaml:"api_auth"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	CORS      CORSConfig      `yaml:"cors"`
}

// APIAuthConfig API认证配置
type APIAuthConfig struct {
	Enabled   bool     `yaml:"enabled"`
	APIKeys   []string `yaml:"api_keys"`
	HeaderName string  `yaml:"header_name"`
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerSecond int  `yaml:"requests_per_second"`
	BurstSize         int  `yaml:"burst_size"`
}

// CORSConfig CORS配置
type CORSConfig struct {
	Enabled         bool     `yaml:"enabled"`
	AllowedOrigins  []string `yaml:"allowed_origins"`
	AllowedMethods  []string `yaml:"allowed_methods"`
	AllowedHeaders  []string `yaml:"allowed_headers"`
}

// DevelopmentConfig 开发配置
type DevelopmentConfig struct {
	MockExternalAPIs bool `yaml:"mock_external_apis"`
	SkipAuth         bool `yaml:"skip_auth"`
	LogSQL           bool `yaml:"log_sql"`
	SeedData         bool `yaml:"seed_data"`
}

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	// 查找配置文件
	configPaths := []string{
		"./config/config.yaml",
		"./config.yaml",
		"/etc/defi-data-sync/config.yaml",
		filepath.Join(os.Getenv("HOME"), ".config/defi-data-sync/config.yaml"),
	}

	var configFile string
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			configFile = path
			break
		}
	}

	if configFile == "" {
		return nil, fmt.Errorf("未找到配置文件")
	}

	// 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	cfg.setDefaults()

	// 验证配置
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func (c *Config) setDefaults() {
	// App配置默认值
	if c.App.Name == "" {
		c.App.Name = "defi-data-sync"
	}
	if c.App.Version == "" {
		c.App.Version = "1.0.0"
	}
	if c.App.Environment == "" {
		c.App.Environment = "development"
	}

	// 服务器配置默认值
	if c.Server.HTTPPort == 0 {
		c.Server.HTTPPort = 8080
	}
	if c.Server.GRPCPort == 0 {
		c.Server.GRPCPort = 9090
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 30 * time.Second
	}
	if c.Server.IdleTimeout == 0 {
		c.Server.IdleTimeout = 60 * time.Second
	}

	// 数据库配置默认值
	if c.Database.MySQL.Host == "" {
		c.Database.MySQL.Host = "localhost"
	}
	if c.Database.MySQL.Port == 0 {
		c.Database.MySQL.Port = 3306
	}
	if c.Database.MySQL.Charset == "" {
		c.Database.MySQL.Charset = "utf8mb4"
	}
	if c.Database.MySQL.MaxOpenConns == 0 {
		c.Database.MySQL.MaxOpenConns = 100
	}
	if c.Database.MySQL.MaxIdleConns == 0 {
		c.Database.MySQL.MaxIdleConns = 20
	}
	if c.Database.MySQL.ConnMaxLifetime == 0 {
		c.Database.MySQL.ConnMaxLifetime = 300 * time.Second
	}
	if c.Database.MySQL.ConnMaxIdleTime == 0 {
		c.Database.MySQL.ConnMaxIdleTime = 60 * time.Second
	}

	if c.Database.Redis.Host == "" {
		c.Database.Redis.Host = "localhost"
	}
	if c.Database.Redis.Port == 0 {
		c.Database.Redis.Port = 6379
	}
	if c.Database.Redis.PoolSize == 0 {
		c.Database.Redis.PoolSize = 100
	}
	if c.Database.Redis.MinIdleConns == 0 {
		c.Database.Redis.MinIdleConns = 10
	}
	if c.Database.Redis.DialTimeout == 0 {
		c.Database.Redis.DialTimeout = 5 * time.Second
	}
	if c.Database.Redis.ReadTimeout == 0 {
		c.Database.Redis.ReadTimeout = 3 * time.Second
	}
	if c.Database.Redis.WriteTimeout == 0 {
		c.Database.Redis.WriteTimeout = 3 * time.Second
	}
	if c.Database.Redis.PoolTimeout == 0 {
		c.Database.Redis.PoolTimeout = 4 * time.Second
	}

	// DeBank配置默认值
	if c.External.Debank.BaseURL == "" {
		c.External.Debank.BaseURL = "https://debank.com"
	}
	if c.External.Debank.Timeout == 0 {
		c.External.Debank.Timeout = 30 * time.Second
	}
	if c.External.Debank.UserAgent == "" {
		c.External.Debank.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	if c.External.Debank.MaxRetries == 0 {
		c.External.Debank.MaxRetries = 3
	}
	if c.External.Debank.RetryDelay == 0 {
		c.External.Debank.RetryDelay = 2 * time.Second
	}
	if c.External.Debank.RateLimit == 0 {
		c.External.Debank.RateLimit = 10
	}

	// 同步配置默认值
	if c.Sync.ProtocolMetadata.Schedule == "" {
		c.Sync.ProtocolMetadata.Schedule = "0 0 2 * * *"
	}
	if c.Sync.ProtocolMetadata.FullSyncDays == 0 {
		c.Sync.ProtocolMetadata.FullSyncDays = 7
	}
	if c.Sync.ProtocolMetadata.BatchSize == 0 {
		c.Sync.ProtocolMetadata.BatchSize = 50
	}
	if c.Sync.ProtocolMetadata.Concurrency == 0 {
		c.Sync.ProtocolMetadata.Concurrency = 5
	}
	if c.Sync.ProtocolMetadata.Timeout == 0 {
		c.Sync.ProtocolMetadata.Timeout = 300 * time.Second
	}

	if c.Sync.ProtocolTokens.Schedule == "" {
		c.Sync.ProtocolTokens.Schedule = "0 0 3 * * *"
	}
	if c.Sync.ProtocolTokens.BatchSize == 0 {
		c.Sync.ProtocolTokens.BatchSize = 100
	}
	if c.Sync.ProtocolTokens.Concurrency == 0 {
		c.Sync.ProtocolTokens.Concurrency = 3
	}
	if c.Sync.ProtocolTokens.Timeout == 0 {
		c.Sync.ProtocolTokens.Timeout = 600 * time.Second
	}

	if c.Sync.UserPositions.Schedule == "" {
		c.Sync.UserPositions.Schedule = "0 */10 * * * *"
	}
	if c.Sync.UserPositions.BatchSize == 0 {
		c.Sync.UserPositions.BatchSize = 1000
	}
	if c.Sync.UserPositions.Concurrency == 0 {
		c.Sync.UserPositions.Concurrency = 10
	}
	if c.Sync.UserPositions.Timeout == 0 {
		c.Sync.UserPositions.Timeout = 180 * time.Second
	}

	if c.Sync.Incremental.CheckInterval == 0 {
		c.Sync.Incremental.CheckInterval = 5 * time.Minute
	}
	if c.Sync.Incremental.MaxBackoff == 0 {
		c.Sync.Incremental.MaxBackoff = 1 * time.Hour
	}
	if c.Sync.Incremental.RetryCount == 0 {
		c.Sync.Incremental.RetryCount = 3
	}

	// 缓存配置默认值
	if c.Cache.Redis.DefaultTTL == 0 {
		c.Cache.Redis.DefaultTTL = 600 * time.Second
	}
	if c.Cache.Redis.PositionTTL == 0 {
		c.Cache.Redis.PositionTTL = 300 * time.Second
	}
	if c.Cache.Redis.ProtocolTTL == 0 {
		c.Cache.Redis.ProtocolTTL = 3600 * time.Second
	}
	if c.Cache.Redis.TokenTTL == 0 {
		c.Cache.Redis.TokenTTL = 1800 * time.Second
	}

	if c.Cache.Memory.DefaultSize == 0 {
		c.Cache.Memory.DefaultSize = 10000
	}
	if c.Cache.Memory.CleanupInterval == 0 {
		c.Cache.Memory.CleanupInterval = 1 * time.Minute
	}

	// 队列配置默认值
	if c.Queue.RedisStream.StreamName == "" {
		c.Queue.RedisStream.StreamName = "defi:updates"
	}
	if c.Queue.RedisStream.ConsumerGroup == "" {
		c.Queue.RedisStream.ConsumerGroup = "defi-workers"
	}
	if c.Queue.RedisStream.ConsumerName == "" {
		c.Queue.RedisStream.ConsumerName = "worker-1"
	}
	if c.Queue.RedisStream.MaxLen == 0 {
		c.Queue.RedisStream.MaxLen = 10000
	}
	if c.Queue.RedisStream.BlockTimeout == 0 {
		c.Queue.RedisStream.BlockTimeout = 5 * time.Second
	}
	if c.Queue.RedisStream.ClaimMinIdle == 0 {
		c.Queue.RedisStream.ClaimMinIdle = 30 * time.Second
	}

	if c.Queue.DeadLetter.StreamName == "" {
		c.Queue.DeadLetter.StreamName = "defi:dead-letter"
	}
	if c.Queue.DeadLetter.MaxRetries == 0 {
		c.Queue.DeadLetter.MaxRetries = 3
	}
	if c.Queue.DeadLetter.RetryDelay == 0 {
		c.Queue.DeadLetter.RetryDelay = 5 * time.Minute
	}

	// 日志配置默认值
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Format == "" {
		c.Log.Format = "json"
	}
	if c.Log.Output == "" {
		c.Log.Output = "stdout"
	}
	if c.Log.FilePath == "" {
		c.Log.FilePath = "./logs/data-sync.log"
	}
	if c.Log.MaxSize == 0 {
		c.Log.MaxSize = 100
	}
	if c.Log.MaxBackups == 0 {
		c.Log.MaxBackups = 10
	}
	if c.Log.MaxAge == 0 {
		c.Log.MaxAge = 30
	}

	// 监控配置默认值
	if c.Monitoring.Prometheus.Path == "" {
		c.Monitoring.Prometheus.Path = "/metrics"
	}
	if c.Monitoring.Prometheus.Port == 0 {
		c.Monitoring.Prometheus.Port = 9091
	}

	if c.Monitoring.HealthCheck.Path == "" {
		c.Monitoring.HealthCheck.Path = "/health"
	}
	if c.Monitoring.HealthCheck.Interval == 0 {
		c.Monitoring.HealthCheck.Interval = 30 * time.Second
	}

	// 告警配置默认值
	if c.Alert.ErrorRate.Threshold == nil {
		c.Alert.ErrorRate.Threshold = 0.01 // 1%
	}
	if c.Alert.ErrorRate.Window == 0 {
		c.Alert.ErrorRate.Window = 5 * time.Minute
	}
	if c.Alert.ErrorRate.Cooldown == 0 {
		c.Alert.ErrorRate.Cooldown = 10 * time.Minute
	}

	if c.Alert.SyncLatency.Threshold == nil {
		c.Alert.SyncLatency.Threshold = 5 * time.Minute
	}
	if c.Alert.SyncLatency.Window == 0 {
		c.Alert.SyncLatency.Window = 10 * time.Minute
	}

	if c.Alert.QueueBacklog.Threshold == nil {
		c.Alert.QueueBacklog.Threshold = 1000
	}
	if c.Alert.QueueBacklog.Window == 0 {
		c.Alert.QueueBacklog.Window = 1 * time.Minute
	}

	// 安全配置默认值
	if c.Security.APIAuth.HeaderName == "" {
		c.Security.APIAuth.HeaderName = "X-API-Key"
	}

	if c.Security.RateLimit.RequestsPerSecond == 0 {
		c.Security.RateLimit.RequestsPerSecond = 100
	}
	if c.Security.RateLimit.BurstSize == 0 {
		c.Security.RateLimit.BurstSize = 50
	}

	if len(c.Security.CORS.AllowedOrigins) == 0 {
		c.Security.CORS.AllowedOrigins = []string{"*"}
	}
	if len(c.Security.CORS.AllowedMethods) == 0 {
		c.Security.CORS.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE"}
	}
	if len(c.Security.CORS.AllowedHeaders) == 0 {
		c.Security.CORS.AllowedHeaders = []string{"Content-Type", "Authorization", "X-API-Key"}
	}
}

// validate 验证配置
func (c *Config) validate() error {
	// 验证必需字段
	if c.Database.MySQL.Username == "" {
		return fmt.Errorf("数据库用户名不能为空")
	}
	if c.Database.MySQL.Password == "" {
		return fmt.Errorf("数据库密码不能为空")
	}
	if c.Database.MySQL.Database == "" {
		return fmt.Errorf("数据库名称不能为空")
	}

	// 验证端口范围
	if c.Server.HTTPPort < 1 || c.Server.HTTPPort > 65535 {
		return fmt.Errorf("HTTP端口无效: %d", c.Server.HTTPPort)
	}
	if c.Server.GRPCPort < 1 || c.Server.GRPCPort > 65535 {
		return fmt.Errorf("gRPC端口无效: %d", c.Server.GRPCPort)
	}
	if c.Database.MySQL.Port < 1 || c.Database.MySQL.Port > 65535 {
		return fmt.Errorf("MySQL端口无效: %d", c.Database.MySQL.Port)
	}
	if c.Database.Redis.Port < 1 || c.Database.Redis.Port > 65535 {
		return fmt.Errorf("Redis端口无效: %d", c.Database.Redis.Port)
	}

	// 验证超时设置
	if c.Server.ReadTimeout <= 0 {
		return fmt.Errorf("读取超时必须大于0")
	}
	if c.Server.WriteTimeout <= 0 {
		return fmt.Errorf("写入超时必须大于0")
	}
	if c.Server.IdleTimeout <= 0 {
		return fmt.Errorf("空闲超时必须大于0")
	}

	// 验证数据库连接池设置
	if c.Database.MySQL.MaxOpenConns <= 0 {
		return fmt.Errorf("最大打开连接数必须大于0")
	}
	if c.Database.MySQL.MaxIdleConns <= 0 {
		return fmt.Errorf("最大空闲连接数必须大于0")
	}
	if c.Database.MySQL.ConnMaxLifetime <= 0 {
		return fmt.Errorf("连接最大生命周期必须大于0")
	}
	if c.Database.MySQL.ConnMaxIdleTime <= 0 {
		return fmt.Errorf("连接最大空闲时间必须大于0")
	}

	// 验证Redis连接池设置
	if c.Database.Redis.PoolSize <= 0 {
		return fmt.Errorf("Redis连接池大小必须大于0")
	}
	if c.Database.Redis.MinIdleConns < 0 {
		return fmt.Errorf("Redis最小空闲连接数不能为负数")
	}

	// 验证同步配置
	if c.Sync.ProtocolMetadata.BatchSize <= 0 {
		return fmt.Errorf("协议元数据批量大小必须大于0")
	}
	if c.Sync.ProtocolMetadata.Concurrency <= 0 {
		return fmt.Errorf("协议元数据并发数必须大于0")
	}
	if c.Sync.ProtocolMetadata.Timeout <= 0 {
		return fmt.Errorf("协议元数据超时必须大于0")
	}

	if c.Sync.ProtocolTokens.BatchSize <= 0 {
		return fmt.Errorf("协议代币批量大小必须大于0")
	}
	if c.Sync.ProtocolTokens.Concurrency <= 0 {
		return fmt.Errorf("协议代币并发数必须大于0")
	}
	if c.Sync.ProtocolTokens.Timeout <= 0 {
		return fmt.Errorf("协议代币超时必须大于0")
	}

	if c.Sync.UserPositions.BatchSize <= 0 {
		return fmt.Errorf("用户仓位批量大小必须大于0")
	}
	if c.Sync.UserPositions.Concurrency <= 0 {
		return fmt.Errorf("用户仓位并发数必须大于0")
	}
	if c.Sync.UserPositions.Timeout <= 0 {
		return fmt.Errorf("用户仓位超时必须大于0")
	}

	// 验证缓存TTL
	if c.Cache.Redis.DefaultTTL <= 0 {
		return fmt.Errorf("默认缓存TTL必须大于0")
	}
	if c.Cache.Redis.PositionTTL <= 0 {
		return fmt.Errorf("仓位缓存TTL必须大于0")
	}
	if c.Cache.Redis.ProtocolTTL <= 0 {
		return fmt.Errorf("协议缓存TTL必须大于0")
	}
	if c.Cache.Redis.TokenTTL <= 0 {
		return fmt.Errorf("代币缓存TTL必须大于0")
	}

	// 验证日志配置
	if c.Log.Level != "debug" && c.Log.Level != "info" && c.Log.Level != "warn" && c.Log.Level != "error" {
		return fmt.Errorf("日志级别无效: %s", c.Log.Level)
	}
	if c.Log.Format != "json" && c.Log.Format != "text" {
		return fmt.Errorf("日志格式无效: %s", c.Log.Format)
	}
	if c.Log.Output != "stdout" && c.Log.Output != "file" {
		return fmt.Errorf("日志输出无效: %s", c.Log.Output)
	}
	if c.Log.Output == "file" && c.Log.FilePath == "" {
		return fmt.Errorf("文件日志路径不能为空")
	}

	return nil
}