package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"

	"defi-asset-service/queue-system/config"
	"defi-asset-service/queue-system/consumer"
	"defi-asset-service/queue-system/monitoring"
	"defi-asset-service/queue-system/producer"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logger := log.New(os.Stdout, "[QueueSystem] ", log.LstdFlags|log.Lshortfile)

	// 初始化Redis客户端
	redisClient := initRedis(cfg, logger)

	// 初始化MySQL数据库
	db := initMySQL(cfg, logger)
	defer db.Close()

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 根据命令行参数决定运行模式
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "producer":
			runProducer(ctx, cfg, redisClient, logger)
		case "consumer":
			runConsumer(ctx, cfg, redisClient, db, logger)
		case "monitor":
			runMonitor(ctx, cfg, redisClient, logger)
		case "api":
			runAPI(ctx, cfg, redisClient, db, logger)
		default:
			logger.Printf("Unknown command: %s", os.Args[1])
			logger.Println("Available commands: producer, consumer, monitor, api")
			os.Exit(1)
		}
	} else {
		// 默认运行所有组件
		runAll(ctx, cfg, redisClient, db, logger)
	}
}

// initRedis 初始化Redis客户端
func initRedis(cfg *config.AppConfig, logger *log.Logger) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatalf("Failed to connect to Redis: %v", err)
	}

	logger.Printf("Connected to Redis at %s (DB: %d)", cfg.Redis.Address, cfg.Redis.DB)
	return redisClient
}

// initMySQL 初始化MySQL数据库
func initMySQL(cfg *config.AppConfig, logger *log.Logger) *sql.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true",
		cfg.MySQL.User,
		cfg.MySQL.Password,
		cfg.MySQL.Host,
		cfg.MySQL.Port,
		cfg.MySQL.Database,
		cfg.MySQL.Charset,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logger.Fatalf("Failed to connect to MySQL: %v", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		logger.Fatalf("Failed to ping MySQL: %v", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	logger.Printf("Connected to MySQL at %s:%d/%s", cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)
	return db
}

// runProducer 运行生产者
func runProducer(ctx context.Context, cfg *config.AppConfig, redisClient *redis.Client, logger *log.Logger) {
	logger.Println("Starting producer...")

	// 创建生产者
	prod := producer.NewProducer(redisClient, &cfg.Queue, logger)

	// 创建HTTP服务器
	httpServer := producer.NewHTTPServer(prod, "8081", logger)

	// 启动HTTP服务器
	go func() {
		if err := httpServer.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	logger.Println("Producer started on :8081")

	// 等待终止信号
	waitForShutdown(ctx, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Printf("Error shutting down HTTP server: %v", err)
		}
	})
}

// runConsumer 运行消费者
func runConsumer(ctx context.Context, cfg *config.AppConfig, redisClient *redis.Client, db *sql.DB, logger *log.Logger) {
	logger.Println("Starting consumer...")

	// 创建消费者
	cons, err := consumer.NewConsumer(redisClient, db, &cfg.Queue, logger)
	if err != nil {
		logger.Fatalf("Failed to create consumer: %v", err)
	}

	// 启动消费者
	if err := cons.Start(ctx); err != nil {
		logger.Fatalf("Failed to start consumer: %v", err)
	}

	logger.Printf("Consumer started with %d workers", cfg.Workers)

	// 等待终止信号
	waitForShutdown(ctx, func() {
		if err := cons.Stop(); err != nil {
			logger.Printf("Error stopping consumer: %v", err)
		}
	})
}

// runMonitor 运行监控器
func runMonitor(ctx context.Context, cfg *config.AppConfig, redisClient *redis.Client, logger *log.Logger) {
	logger.Println("Starting monitor...")

	// 创建监控器
	monitor := monitoring.NewQueueMonitor(redisClient, &cfg.Queue, logger)

	// 启动监控器
	if err := monitor.Start(ctx); err != nil {
		logger.Fatalf("Failed to start monitor: %v", err)
	}

	// 设置告警
	alertConfig := monitoring.DefaultAlertConfig()
	if err := monitor.SetupAlerting(ctx, alertConfig); err != nil {
		logger.Printf("Failed to setup alerting: %v", err)
	}

	// 创建HTTP处理器
	handler := monitoring.NewHTTPHandler(monitor)

	// 启动HTTP服务器
	server := &http.Server{
		Addr:    ":9090",
		Handler: handler,
	}

	go func() {
		logger.Println("Monitor HTTP server started on :9090")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start monitor HTTP server: %v", err)
		}
	}()

	// 等待终止信号
	waitForShutdown(ctx, func() {
		monitor.Stop()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Printf("Error shutting down monitor HTTP server: %v", err)
		}
	})
}

// runAPI 运行API服务器（包含所有功能）
func runAPI(ctx context.Context, cfg *config.AppConfig, redisClient *redis.Client, db *sql.DB, logger *log.Logger) {
	logger.Println("Starting API server with all components...")

	// 创建生产者
	prod := producer.NewProducer(redisClient, &cfg.Queue, logger)

	// 创建消费者
	cons, err := consumer.NewConsumer(redisClient, db, &cfg.Queue, logger)
	if err != nil {
		logger.Fatalf("Failed to create consumer: %v", err)
	}

	// 创建监控器
	monitor := monitoring.NewQueueMonitor(redisClient, &cfg.Queue, logger)

	// 启动所有组件
	if err := cons.Start(ctx); err != nil {
		logger.Fatalf("Failed to start consumer: %v", err)
	}

	if err := monitor.Start(ctx); err != nil {
		logger.Fatalf("Failed to start monitor: %v", err)
	}

	// 设置告警
	alertConfig := monitoring.DefaultAlertConfig()
	if err := monitor.SetupAlerting(ctx, alertConfig); err != nil {
		logger.Printf("Failed to setup alerting: %v", err)
	}

	// 创建组合HTTP处理器
	mux := http.NewServeMux()

	// 生产者API
	prodHandler := producer.NewHTTPServer(prod, "8081", logger)
	mux.Handle("/producer/", http.StripPrefix("/producer", prodHandler))

	// 监控API
	monitorHandler := monitoring.NewHTTPHandler(monitor)
	mux.Handle("/monitor/", http.StripPrefix("/monitor", monitorHandler))

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// 检查所有组件健康状态
		status := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"components": map[string]string{
				"producer": "running",
				"consumer": "running",
				"monitor":  "running",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// 根路径
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		info := map[string]interface{}{
			"service":   "DeFi Asset Queue System",
			"version":   "1.0.0",
			"endpoints": []string{"/producer", "/monitor", "/health"},
			"timestamp": time.Now().Unix(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	})

	// 启动HTTP服务器
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		logger.Println("API server started on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// 等待终止信号
	waitForShutdown(ctx, func() {
		// 停止消费者
		if err := cons.Stop(); err != nil {
			logger.Printf("Error stopping consumer: %v", err)
		}

		// 停止监控器
		monitor.Stop()

		// 关闭HTTP服务器
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Printf("Error shutting down API server: %v", err)
		}
	})
}

// runAll 运行所有组件（独立进程）
func runAll(ctx context.Context, cfg *config.AppConfig, redisClient *redis.Client, db *sql.DB, logger *log.Logger) {
	logger.Println("Starting all components in separate goroutines...")

	// 创建消费者
	cons, err := consumer.NewConsumer(redisClient, db, &cfg.Queue, logger)
	if err != nil {
		logger.Fatalf("Failed to create consumer: %v", err)
	}

	// 创建监控器
	monitor := monitoring.NewQueueMonitor(redisClient, &cfg.Queue, logger)

	// 启动消费者
	if err := cons.Start(ctx); err != nil {
		logger.Fatalf("Failed to start consumer: %v", err)
	}

	// 启动监控器
	if err := monitor.Start(ctx); err != nil {
		logger.Fatalf("Failed to start monitor: %v", err)
	}

	// 设置告警
	alertConfig := monitoring.DefaultAlertConfig()
	if err := monitor.SetupAlerting(ctx, alertConfig); err != nil {
		logger.Printf("Failed to setup alerting: %v", err)
	}

	// 创建生产者HTTP服务器
	prod := producer.NewProducer(redisClient, &cfg.Queue, logger)
	prodServer := producer.NewHTTPServer(prod, "8081", logger)

	// 创建监控HTTP服务器
	monitorHandler := monitoring.NewHTTPHandler(monitor)
	monitorMux := http.NewServeMux()
	monitorMux.Handle("/", monitorHandler)
	monitorServer := &http.Server{
		Addr:    ":9090",
		Handler: monitorMux,
	}

	// 启动HTTP服务器
	go func() {
		logger.Println("Producer API started on :8081")
		if err := prodServer.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start producer API: %v", err)
		}
	}()

	go func() {
		logger.Println("Monitor API started on :9090")
		if err := monitorServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start monitor API: %v", err)
		}
	}()

	// 等待终止信号
	waitForShutdown(ctx, func() {
		// 停止消费者
		if err := cons.Stop(); err != nil {
			logger.Printf("Error stopping consumer: %v", err)
		}

		// 停止监控器
		monitor.Stop()

		// 关闭HTTP服务器
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := prodServer.Shutdown(shutdownCtx); err != nil {
			logger.Printf("Error shutting down producer API: %v", err)
		}

		if err := monitorServer.Shutdown(shutdownCtx); err != nil {
			logger.Printf("Error shutting down monitor API: %v", err)
		}
	})
}

// waitForShutdown 等待终止信号
func waitForShutdown(ctx context.Context, shutdownFunc func()) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan
	log.Println("Shutdown signal received")

	if shutdownFunc != nil {
		shutdownFunc()
	}

	log.Println("Shutdown completed")
}

// 导入JSON包
import "encoding/json"