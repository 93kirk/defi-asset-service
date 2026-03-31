package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"defi-asset-service/internal/api"
	"defi-asset-service/internal/config"
	"defi-asset-service/internal/repository"
	"defi-asset-service/internal/service"
	"defi-asset-service/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// 1. 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化日志
	log := initLogger(cfg.Log)
	defer func() {
		if logFile, ok := log.Out.(*os.File); ok && logFile != os.Stdout {
			logFile.Close()
		}
	}()

	// 3. 初始化数据库
	db, err := initDatabase(cfg.Database.MySQL, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize database")
	}
	defer func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}()

	// 4. 初始化Redis
	redisClient, err := initRedis(cfg.Redis, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize Redis")
	}
	defer redisClient.Close()

	// 5. 初始化仓库
	userRepo := repository.NewUserRepository(db)
	protocolRepo := repository.NewProtocolRepository(db)
	redisRepo := repository.NewRedisRepository(redisClient, "defi")

	// 6. 初始化服务
	serviceAClient := service.NewServiceAClient(&cfg.External.ServiceA, log)
	serviceBClient := service.NewServiceBClient(&cfg.External.ServiceB, log)
	
	serviceASvc := service.NewServiceAService(
		serviceAClient,
		userRepo,
		redisRepo,
		&cfg.Cache,
		log,
	)
	
	serviceBSvc := service.NewServiceBService(
		serviceBClient,
		userRepo,
		redisRepo,
		&cfg.Cache,
		&cfg.Business,
		log,
	)
	
	protocolClient := service.NewProtocolClient(&cfg.External.Debank, log)
	protocolSvc := service.NewProtocolService(
		protocolClient,
		protocolRepo,
		redisRepo,
		&cfg.Cache,
		&cfg.Cron,
		log,
	)
	
	queueWorker := service.NewQueueWorker(
		&cfg.Queue,
		redisRepo,
		serviceBSvc,
		log,
	)

	// 7. 初始化中间件
	authMiddleware := middleware.NewAuthMiddleware(&cfg.Auth, log)
	loggingMiddleware := middleware.NewLoggingMiddleware(log)

	// 8. 初始化控制器
	userController := api.NewUserController(serviceASvc, serviceBSvc, log)
	protocolController := api.NewProtocolController(protocolSvc, log)

	// 9. 初始化Gin引擎
	router := initRouter(cfg, authMiddleware, loggingMiddleware, userController, protocolController)

	// 10. 启动队列Worker
	ctx := context.Background()
	if err := queueWorker.Start(ctx); err != nil {
		log.WithError(err).Error("Failed to start queue worker")
	}
	defer queueWorker.Stop()

	// 11. 启动定时任务
	cronScheduler := initCronScheduler(protocolSvc, redisRepo, log)
	cronScheduler.Start()
	defer cronScheduler.Stop()

	// 12. 启动HTTP服务器
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 13. 优雅关机
	go func() {
		log.Infof("Server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Failed to start server")
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// 设置关机超时
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Fatal("Server forced to shutdown")
	}

	log.Info("Server exited properly")
}

// initLogger 初始化日志
func initLogger(cfg config.LogConfig) *logrus.Logger {
	log := logrus.New()
	
	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)
	
	// 设置日志格式
	if cfg.Format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	}
	
	// 设置日志输出
	if cfg.Output == "file" && cfg.FilePath != "" {
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.WithError(err).Warn("Failed to open log file, using stdout")
			log.SetOutput(os.Stdout)
		} else {
			log.SetOutput(file)
		}
	} else {
		log.SetOutput(os.Stdout)
	}
	
	return log
}

// initDatabase 初始化数据库
func initDatabase(cfg config.MySQLConfig, log *logrus.Logger) (*gorm.DB, error) {
	// 构建DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.Charset,
		cfg.ParseTime,
	)

	// 配置GORM日志
	gormLogger := logger.New(
		log,
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 获取通用数据库对象
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connection established")
	return db, nil
}

// initRedis 初始化Redis
func initRedis(cfg config.RedisConfig, log *logrus.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		PoolTimeout:  time.Duration(cfg.PoolTimeout) * time.Second,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Info("Redis connection established")
	return client, nil
}

// initRouter 初始化路由器
func initRouter(
	cfg *config.Config,
	authMiddleware *middleware.AuthMiddleware,
	loggingMiddleware *middleware.LoggingMiddleware,
	userController *api.UserController,
	protocolController *api.ProtocolController,
) *gin.Engine {
	// 设置Gin模式
	gin.SetMode(cfg.Server.Mode)
	
	router := gin.New()
	
	// 全局中间件
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware.RequestLogging())
	
	// 健康检查端点
	if cfg.Monitoring.HealthCheck.Enabled {
		router.GET(cfg.Monitoring.HealthCheck.Path, func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"timestamp": time.Now().Format(time.RFC3339),
			})
		})
	}
	
	// Prometheus指标端点
	if cfg.Monitoring.Prometheus.Enabled {
		router.GET(cfg.Monitoring.Prometheus.Path, gin.WrapH(promhttp.Handler()))
	}
	
	// API路由组
	apiGroup := router.Group("/v1")
	apiGroup.Use(authMiddleware.Authenticate())
	apiGroup.Use(authMiddleware.RateLimit())
	
	// 注册控制器路由
	userController.RegisterRoutes(apiGroup)
	protocolController.RegisterRoutes(apiGroup)
	
	// 404处理
	router.NoRoute(func(ctx *gin.Context) {
		ctx.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Not Found",
		})
	})
	
	return router
}

// initCronScheduler 初始化定时任务调度器
func initCronScheduler(
	protocolSvc service.ProtocolService,
	redisRepo repository.RedisRepository,
	log *logrus.Logger,
) *cron.Cron {
	c := cron.New()
	
	// 协议同步任务
	c.AddFunc("0 2 * * *", func() {
		log.Info("Starting scheduled protocol sync")
		ctx := context.Background()
		
		// 触发全量同步
		if _, err := protocolSvc.SyncProtocols(ctx, false, nil); err != nil {
			log.WithError(err).Error("Failed to start protocol sync")
		}
	})
	
	// 缓存清理任务
	c.AddFunc("0 */6 * * *", func() {
		log.Info("Starting scheduled cache cleanup")
		ctx := context.Background()
		
		if err := redisRepo.CleanupExpiredCache(ctx); err != nil {
			log.WithError(err).Error("Failed to cleanup cache")
		}
		
		if err := protocolSvc.CleanupProtocolCache(ctx); err != nil {
			log.WithError(err).Error("Failed to cleanup protocol cache")
		}
	})
	
	return c
}

// 导入必要的包
import (
	"github.com/robfig/cron/v3"
)