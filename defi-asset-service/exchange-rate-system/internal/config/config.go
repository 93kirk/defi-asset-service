package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Cache     CacheConfig     `yaml:"cache"`
	Providers ProvidersConfig `yaml:"providers"`
	Logging   LoggingConfig   `yaml:"logging"`
	Exchange  ExchangeConfig  `yaml:"exchange"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port         int           `yaml:"port"`
	Host         string        `yaml:"host"`
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
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	MaxConns int    `yaml:"max_conns"`
	MaxIdle  int    `yaml:"max_idle"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Redis  RedisCacheConfig  `yaml:"redis"`
	Memory MemoryCacheConfig `yaml:"memory"`
}

// RedisCacheConfig Redis缓存配置
type RedisCacheConfig struct {
	Enabled  bool          `yaml:"enabled"`
	URL      string        `yaml:"url"`
	Password string        `yaml:"password"`
	DB       int           `yaml:"db"`
	TTL      time.Duration `yaml:"ttl"`
}

// MemoryCacheConfig 内存缓存配置
type MemoryCacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	Size    int           `yaml:"size"`
	TTL     time.Duration `yaml:"ttl"`
}

// ProvidersConfig 数据提供者配置
type ProvidersConfig struct {
	Chainlink        ProviderConfig `yaml:"chainlink"`
	ProtocolAPI      ProviderConfig `yaml:"protocol_api"`
	CustomCalculator ProviderConfig `yaml:"custom_calculator"`
	Debank           ProviderConfig `yaml:"debank"`
}

// ProviderConfig 提供者配置
type ProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	APIKey  string `yaml:"api_key"`
	Timeout int    `yaml:"timeout"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// ExchangeConfig 汇率配置
type ExchangeConfig struct {
	DefaultCacheTTL      time.Duration `yaml:"default_cache_ttl"`
	MaxRetries           int           `yaml:"max_retries"`
	RetryDelay           time.Duration `yaml:"retry_delay"`
	ValidationThreshold  float64       `yaml:"validation_threshold"`
	RateLimitPerSecond   int           `yaml:"rate_limit_per_second"`
	BatchSizeLimit       int           `yaml:"batch_size_limit"`
	HistoricalDataDays   int           `yaml:"historical_data_days"`
	UpdateInterval       time.Duration `yaml:"update_interval"`
	Protocols            ProtocolsConfig `yaml:"protocols"`
}

// ProtocolsConfig 协议配置
type ProtocolsConfig struct {
	LiquidStaking   ProtocolTypeConfig `yaml:"liquid_staking"`
	Lending         ProtocolTypeConfig `yaml:"lending"`
	AMM             ProtocolTypeConfig `yaml:"amm"`
	YieldAggregator ProtocolTypeConfig `yaml:"yield_aggregator"`
	LSDRewards      ProtocolTypeConfig `yaml:"lsd_rewards"`
	Restaking       ProtocolTypeConfig `yaml:"restaking"`
}

// ProtocolTypeConfig 协议类型配置
type ProtocolTypeConfig struct {
	UpdateInterval time.Duration `yaml:"update_interval"`
	Priority       int           `yaml:"priority"`
	Enabled        bool          `yaml:"enabled"`
}

// DefaultConfig 默认配置
var DefaultConfig = Config{
	Server: ServerConfig{
		Port:         8080,
		Host:         "0.0.0.0",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	},
	Database: DatabaseConfig{
		MySQL: MySQLConfig{
			Host:     "localhost",
			Port:     3306,
			User:     "root",
			Password: "",
			Database: "defi_exchange",
			MaxConns: 100,
			MaxIdle:  10,
		},
		Redis: RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       0,
		},
	},
	Cache: CacheConfig{
		Redis: RedisCacheConfig{
			Enabled:  true,
			URL:      "redis://localhost:6379",
			Password: "",
			DB:       0,
			TTL:      5 * time.Minute,
		},
		Memory: MemoryCacheConfig{
			Enabled: true,
			Size:    1000,
			TTL:     1 * time.Minute,
		},
	},
	Providers: ProvidersConfig{
		Chainlink: ProviderConfig{
			Enabled: true,
			URL:     "https://api.chain.link",
			Timeout: 10,
		},
		ProtocolAPI: ProviderConfig{
			Enabled: true,
			Timeout: 5,
		},
		CustomCalculator: ProviderConfig{
			Enabled: true,
		},
		Debank: ProviderConfig{
			Enabled: true,
			URL:     "https://api.debank.com",
			Timeout: 5,
		},
	},
	Logging: LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	},
	Exchange: ExchangeConfig{
		DefaultCacheTTL:      5 * time.Minute,
		MaxRetries:           3,
		RetryDelay:           1 * time.Second,
		ValidationThreshold:  0.8,
		RateLimitPerSecond:   10,
		BatchSizeLimit:       100,
		HistoricalDataDays:   30,
		UpdateInterval:       1 * time.Minute,
		Protocols: ProtocolsConfig{
			LiquidStaking: ProtocolTypeConfig{
				UpdateInterval: 10 * time.Second,
				Priority:       1,
				Enabled:        true,
			},
			Lending: ProtocolTypeConfig{
				UpdateInterval: 30 * time.Second,
				Priority:       1,
				Enabled:        true,
			},
			AMM: ProtocolTypeConfig{
				UpdateInterval: 5 * time.Second,
				Priority:       2,
				Enabled:        true,
			},
			YieldAggregator: ProtocolTypeConfig{
				UpdateInterval: 1 * time.Minute,
				Priority:       2,
				Enabled:        true,
			},
			LSDRewards: ProtocolTypeConfig{
				UpdateInterval: 5 * time.Minute,
				Priority:       3,
				Enabled:        true,
			},
			Restaking: ProtocolTypeConfig{
				UpdateInterval: 5 * time.Minute,
				Priority:       3,
				Enabled:        true,
			},
		},
	},
}

// LoadConfig 加载配置
func LoadConfig() (*Config, error) {
	config := DefaultConfig
	
	// 从环境变量覆盖配置
	if port := os.Getenv("SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}
	
	if host := os.Getenv("SERVER_HOST"); host != "" {
		config.Server.Host = host
	}
	
	// 数据库配置
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		config.Database.MySQL.Host = dbHost
	}
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		if p, err := strconv.Atoi(dbPort); err == nil {
			config.Database.MySQL.Port = p
		}
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		config.Database.MySQL.User = dbUser
	}
	if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
		config.Database.MySQL.Password = dbPass
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		config.Database.MySQL.Database = dbName
	}
	
	// Redis配置
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		config.Cache.Redis.URL = redisURL
	}
	if redisPass := os.Getenv("REDIS_PASSWORD"); redisPass != "" {
		config.Cache.Redis.Password = redisPass
	}
	
	// 日志配置
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}
	
	// 尝试从配置文件加载
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.yaml"
	}
	
	if _, err := os.Stat(configFile); err == nil {
		data, err := os.ReadFile(configFile)
		if err == nil {
			if err := yaml.Unmarshal(data, &config); err != nil {
				return nil, err
			}
		}
	}
	
	return &config, nil
}

// GetMySQLDSN 获取MySQL DSN
func (c *Config) GetMySQLDSN() string {
	return c.Database.MySQL.User + ":" + c.Database.MySQL.Password + "@tcp(" +
		c.Database.MySQL.Host + ":" + strconv.Itoa(c.Database.MySQL.Port) + ")/" +
		c.Database.MySQL.Database + "?parseTime=true"
}

// GetRedisAddr 获取Redis地址
func (c *Config) GetRedisAddr() string {
	return c.Database.Redis.Host + ":" + strconv.Itoa(c.Database.Redis.Port)
}