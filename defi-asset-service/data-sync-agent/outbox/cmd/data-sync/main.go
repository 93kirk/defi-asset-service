package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/config"
	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/service"
	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/state"
	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/sync"
)

func main() {
	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		slog.Info("收到退出信号", "signal", sig)
		cancel()
	}()

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	// 初始化日志
	initLogger(cfg)

	slog.Info("启动DeFi数据同步服务", 
		"version", cfg.App.Version,
		"environment", cfg.App.Environment)

	// 初始化数据库连接
	db, err := initDatabase(cfg)
	if err != nil {
		slog.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}
	defer closeDatabase(db)

	// 初始化Redis连接
	redisClient, err := initRedis(cfg)
	if err != nil {
		slog.Error("初始化Redis失败", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	// 创建同步服务
	syncService, err := service.NewSyncService(db, cfg, slog.Default())
	if err != nil {
		slog.Error("创建同步服务失败", "error", err)
		os.Exit(1)
	}
	defer syncService.Close()

	// 创建状态管理器
	stateManager := state.NewStateManager(db, slog.Default())
	if err := stateManager.Initialize(ctx); err != nil {
		slog.Error("初始化状态管理器失败", "error", err)
		os.Exit(1)
	}

	// 创建定时任务调度器
	scheduler := sync.NewScheduler(db, cfg, slog.Default(), syncService)

	// 启动调度器
	if err := scheduler.Start(ctx); err != nil {
		slog.Error("启动调度器失败", "error", err)
		os.Exit(1)
	}
	defer scheduler.Stop()

	slog.Info("数据同步服务启动完成")

	// 等待上下文取消
	<-ctx.Done()
	
	slog.Info("正在关闭数据同步服务...")
	
	// 等待一段时间让服务优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	// 这里可以添加其他资源的关闭逻辑
	
	<-shutdownCtx.Done()
	slog.Info("数据同步服务已关闭")
}

// initLogger 初始化日志
func initLogger(cfg *config.Config) {
	var logLevel slog.Level
	switch cfg.Log.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// 创建日志处理器
	var handler slog.Handler
	if cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})
	}

	// 设置默认logger
	slog.SetDefault(slog.New(handler))
}

// initDatabase 初始化数据库
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	// 构建DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Database.MySQL.Username,
		cfg.Database.MySQL.Password,
		cfg.Database.MySQL.Host,
		cfg.Database.MySQL.Port,
		cfg.Database.MySQL.Database,
		cfg.Database.MySQL.Charset,
	)

	// 配置GORM日志
	gormLogger := logger.New(
		slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// 打开数据库连接
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 获取通用数据库对象
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库对象失败: %w", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(cfg.Database.MySQL.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MySQL.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.MySQL.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.Database.MySQL.ConnMaxIdleTime)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	slog.Info("数据库连接成功",
		"host", cfg.Database.MySQL.Host,
		"database", cfg.Database.MySQL.Database)

	return db, nil
}

// initRedis 初始化Redis
func initRedis(cfg *config.Config) (*redis.Client, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Database.Redis.Host, cfg.Database.Redis.Port),
		Password:     cfg.Database.Redis.Password,
		DB:           cfg.Database.Redis.DB,
		PoolSize:     cfg.Database.Redis.PoolSize,
		MinIdleConns: cfg.Database.Redis.MinIdleConns,
		DialTimeout:  cfg.Database.Redis.DialTimeout,
		ReadTimeout:  cfg.Database.Redis.ReadTimeout,
		WriteTimeout: cfg.Database.Redis.WriteTimeout,
		PoolTimeout:  cfg.Database.Redis.PoolTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接测试失败: %w", err)
	}

	slog.Info("Redis连接成功",
		"host", cfg.Database.Redis.Host,
		"port", cfg.Database.Redis.Port)

	return redisClient, nil
}

// closeDatabase 关闭数据库连接
func closeDatabase(db *gorm.DB) {
	if db != nil {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
			slog.Info("数据库连接已关闭")
		}
	}
}