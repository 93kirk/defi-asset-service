package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"defi-asset-service/exchange-rate-system/api"
	"defi-asset-service/exchange-rate-system/internal/adapter"
	"defi-asset-service/exchange-rate-system/internal/cache"
	"defi-asset-service/exchange-rate-system/internal/calculator"
	"defi-asset-service/exchange-rate-system/internal/config"
	"defi-asset-service/exchange-rate-system/internal/provider"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化组件
	adapterFactory := adapter.NewAdapterFactory()
	rateProviders := initRateProviders(cfg)
	rateCache := initCache(cfg)
	
	// 创建适配器注册表（从DeBank数据加载）
	registry := adapterFactory.GetRegistry()
	loadProtocolAdapters(adapterFactory, cfg)
	
	// 创建汇率计算引擎
	engine := calculator.NewExchangeRateEngine(
		registry,
		rateProviders,
		rateCache,
		calculator.DefaultConfig,
	)
	
	// 创建API控制器
	controller := api.NewExchangeRateController(engine)
	
	// 创建HTTP路由器
	r := chi.NewRouter()
	
	// 中间件
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	
	// CORS配置
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	
	// 注册路由
	r.Route("/api/v1", func(r chi.Router) {
		// 汇率系统路由
		controller.RegisterRoutes(r)
		
		// 集成到现有服务的路由
		r.Route("/protocols", func(r chi.Router) {
			r.Get("/{id}/rates", controller.GetProtocolRates)
			r.Get("/{id}/rates/history", controller.GetHistoricalRates)
		})
		
		r.Route("/users", func(r chi.Router) {
			r.Get("/{id}/assets/with-rates", getUserAssetsWithRates(engine))
		})
		
		r.Route("/assets", func(r chi.Router) {
			r.Post("/calculate-rates", controller.CalculateRate)
			r.Post("/calculate-rates/batch", controller.BatchCalculateRates)
		})
	})
	
	// 健康检查
	r.Get("/health", controller.HealthCheck)
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// 启动服务器
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	// 优雅关机
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	
	go func() {
		log.Printf("Starting exchange rate service on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()
	
	// 等待关机信号
	<-ctx.Done()
	log.Println("Shutting down server...")
	
	// 给服务器时间完成现有请求
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	
	log.Println("Server stopped")
}

func initRateProviders(cfg *config.Config) []provider.ExchangeRateProvider {
	var providers []provider.ExchangeRateProvider
	
	// 链上预言机提供者
	if cfg.Providers.Chainlink.Enabled {
		providers = append(providers, provider.NewChainlinkProvider(cfg.Providers.Chainlink.URL))
	}
	
	// 协议API提供者
	if cfg.Providers.ProtocolAPI.Enabled {
		providers = append(providers, provider.NewProtocolAPIProvider())
	}
	
	// 自定义计算提供者
	if cfg.Providers.CustomCalculator.Enabled {
		providers = append(providers, provider.NewCustomCalculatorProvider())
	}
	
	return providers
}

func initCache(cfg *config.Config) cache.RateCache {
	if cfg.Cache.Redis.Enabled {
		redisCache, err := cache.NewRedisCache(
			cfg.Cache.Redis.URL,
			cfg.Cache.Redis.Password,
			cfg.Cache.Redis.DB,
		)
		if err != nil {
			log.Printf("Failed to connect to Redis, falling back to memory cache: %v", err)
			return cache.NewMemoryCache(
				cfg.Cache.Memory.Size,
				time.Duration(cfg.Cache.Memory.TTL)*time.Second,
			)
		}
		return redisCache
	}
	
	// 默认使用内存缓存
	return cache.NewMemoryCache(
		cfg.Cache.Memory.Size,
		time.Duration(cfg.Cache.Memory.TTL)*time.Second,
	)
}

func loadProtocolAdapters(factory *adapter.AdapterFactory, cfg *config.Config) {
	// 这里应该从数据库或配置文件加载协议数据
	// 暂时使用硬编码的示例数据
	
	protocols := []map[string]interface{}{
		{
			"id":   "eth2",
			"name": "Eth2",
			"chain": "eth",
			"pool_stats": []interface{}{
				map[string]interface{}{"name": "Staked", "rate": 1.02},
			},
		},
		{
			"id":   "aave3",
			"name": "Aave V3",
			"chain": "eth",
			"pool_stats": []interface{}{
				map[string]interface{}{"name": "Lending", "rate": 1.03},
				map[string]interface{}{"name": "Yield", "rate": 1.05},
			},
		},
		{
			"id":   "lido",
			"name": "LIDO",
			"chain": "eth",
			"pool_stats": []interface{}{
				map[string]interface{}{"name": "Staked", "rate": 1.02},
				map[string]interface{}{"name": "Yield", "rate": 1.04},
			},
		},
		{
			"id":   "uniswap_v3",
			"name": "Uniswap V3",
			"chain": "eth",
			"pool_stats": []interface{}{
				map[string]interface{}{"name": "Liquidity Pool", "rate": 1.01},
			},
		},
		{
			"id":   "yearn",
			"name": "Yearn Finance",
			"chain": "eth",
			"pool_stats": []interface{}{
				map[string]interface{}{"name": "Yield", "rate": 1.08},
			},
		},
	}
	
	_, err := factory.CreateAdaptersFromDebankData(protocols)
	if err != nil {
		log.Printf("Failed to create adapters: %v", err)
	}
	
	log.Printf("Loaded %d protocol adapters", len(protocols))
}

// getUserAssetsWithRates 获取用户资产带汇率（集成到现有服务）
func getUserAssetsWithRates(engine *calculator.ExchangeRateEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID := chi.URLParam(r, "id")
		
		if userID == "" {
			http.Error(w, "userID is required", http.StatusBadRequest)
			return
		}
		
		// 这里应该从现有服务获取用户资产
		// 暂时返回示例数据
		assets := []map[string]interface{}{
			{
				"id":           "asset_1",
				"protocol_id":  "lido",
				"token":        "ETH",
				"amount":       10.5,
				"value_usd":    35000,
			},
			{
				"id":           "asset_2",
				"protocol_id":  "aave3",
				"token":        "USDC",
				"amount":       5000,
				"value_usd":    5000,
			},
		}
		
		// 为每个资产计算汇率
		var assetsWithRates []map[string]interface{}
		for _, asset := range assets {
			protocolID := asset["protocol_id"].(string)
			token := asset["token"].(string)
			amount := asset["amount"].(float64)
			
			request := models.RateCalculationRequest{
				ProtocolID:      protocolID,
				UnderlyingToken: token,
				Amount:          amount,
				Timestamp:       &[]time.Time{time.Now()}[0],
			}
			
			response, err := engine.CalculateRate(ctx, request)
			if err != nil {
				// 记录错误但继续处理其他资产
				log.Printf("Failed to calculate rate for asset %s: %v", asset["id"], err)
				continue
			}
			
			assetWithRate := map[string]interface{}{
				"asset":          asset,
				"exchange_rate":  response.ExchangeRate,
				"receipt_amount": response.ReceiptAmount,
				"calculation":    response,
			}
			
			assetsWithRates = append(assetsWithRates, assetWithRate)
		}
		
		// 返回结果
		render.JSON(w, r, map[string]interface{}{
			"user_id": userID,
			"assets":  assetsWithRates,
			"count":   len(assetsWithRates),
			"total_value_usd": calculateTotalValue(assetsWithRates),
			"timestamp": time.Now().Unix(),
		})
	}
}

func calculateTotalValue(assets []map[string]interface{}) float64 {
	var total float64
	for _, asset := range assets {
		if assetMap, ok := asset["asset"].(map[string]interface{}); ok {
			if value, ok := assetMap["value_usd"].(float64); ok {
				total += value
			}
		}
	}
	return total
}

// 需要导入的包
import (
	"defi-asset-service/exchange-rate-system/internal/models"
	"github.com/go-chi/render"
)