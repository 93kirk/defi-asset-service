package middleware

import (
	"net/http"
	"strings"
	"time"

	"defi-asset-service/internal/config"
	"defi-asset-service/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	config *config.AuthConfig
	logger *logrus.Logger
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(config *config.AuthConfig, logger *logrus.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		config: config,
		logger: logger,
	}
}

// Authenticate API密钥认证
func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 获取API密钥
		apiKey := m.getAPIKey(ctx)
		if apiKey == "" {
			ctx.JSON(http.StatusUnauthorized, model.NewErrorResponse(model.CodeUnauthorized))
			ctx.Abort()
			return
		}
		
		// 验证API密钥
		if !m.validateAPIKey(apiKey) {
			ctx.JSON(http.StatusUnauthorized, model.NewErrorResponse(model.CodeInvalidAPIKey))
			ctx.Abort()
			return
		}
		
		ctx.Next()
	}
}

// AuthenticateJWT JWT认证
func (m *AuthMiddleware) AuthenticateJWT() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 获取JWT Token
		tokenString := m.getJWTToken(ctx)
		if tokenString == "" {
			ctx.JSON(http.StatusUnauthorized, model.NewErrorResponse(model.CodeUnauthorized))
			ctx.Abort()
			return
		}
		
		// 解析和验证JWT
		claims, err := m.parseJWTToken(tokenString)
		if err != nil {
			m.logger.WithError(err).Debug("JWT validation failed")
			ctx.JSON(http.StatusUnauthorized, model.NewErrorResponse(model.CodeInvalidToken))
			ctx.Abort()
			return
		}
		
		// 将用户信息存储到上下文
		ctx.Set("user_id", claims.UserID)
		ctx.Set("user_address", claims.UserAddress)
		ctx.Set("user_role", claims.Role)
		
		ctx.Next()
	}
}

// RateLimit 限流中间件
func (m *AuthMiddleware) RateLimit() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 获取客户端标识
		clientID := m.getClientID(ctx)
		
		// 检查分钟级限流
		minuteKey := fmt.Sprintf("%s:minute:%d", clientID, time.Now().Minute())
		minuteCount, err := m.incrementRateLimit(ctx, minuteKey, time.Minute)
		if err != nil {
			m.logger.WithError(err).Error("Failed to check minute rate limit")
			ctx.Next()
			return
		}
		
		if minuteCount > int64(m.config.RateLimitPerMinute) {
			ctx.JSON(http.StatusTooManyRequests, model.NewErrorResponse(model.CodeRateLimitExceeded))
			ctx.Abort()
			return
		}
		
		// 检查小时级限流
		hourKey := fmt.Sprintf("%s:hour:%d", clientID, time.Now().Hour())
		hourCount, err := m.incrementRateLimit(ctx, hourKey, time.Hour)
		if err != nil {
			m.logger.WithError(err).Error("Failed to check hour rate limit")
			ctx.Next()
			return
		}
		
		if hourCount > int64(m.config.RateLimitPerHour) {
			ctx.JSON(http.StatusTooManyRequests, model.NewErrorResponse(model.CodeRateLimitExceeded))
			ctx.Abort()
			return
		}
		
		// 设置限流头
		ctx.Header("X-RateLimit-Limit-Minute", fmt.Sprintf("%d", m.config.RateLimitPerMinute))
		ctx.Header("X-RateLimit-Remaining-Minute", fmt.Sprintf("%d", m.config.RateLimitPerMinute-int(minuteCount)))
		ctx.Header("X-RateLimit-Limit-Hour", fmt.Sprintf("%d", m.config.RateLimitPerHour))
		ctx.Header("X-RateLimit-Remaining-Hour", fmt.Sprintf("%d", m.config.RateLimitPerHour-int(hourCount)))
		
		ctx.Next()
	}
}

// getAPIKey 获取API密钥
func (m *AuthMiddleware) getAPIKey(ctx *gin.Context) string {
	// 从Header获取
	apiKey := ctx.GetHeader(m.config.APIKeyHeader)
	if apiKey != "" {
		return apiKey
	}
	
	// 从Query参数获取
	apiKey = ctx.Query("api_key")
	if apiKey != "" {
		return apiKey
	}
	
	// 从Authorization Header获取（Bearer格式）
	authHeader := ctx.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	
	return ""
}

// getJWTToken 获取JWT Token
func (m *AuthMiddleware) getJWTToken(ctx *gin.Context) string {
	// 从Authorization Header获取
	authHeader := ctx.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}
	
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	
	return parts[1]
}

// validateAPIKey 验证API密钥
func (m *AuthMiddleware) validateAPIKey(apiKey string) bool {
	// 这里应该实现API密钥的验证逻辑
	// 可以从数据库或配置文件中验证
	// 暂时返回true用于测试
	return true
}

// parseJWTToken 解析JWT Token
func (m *AuthMiddleware) parseJWTToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.config.JWTSecret), nil
	})
	
	if err != nil {
		return nil, err
	}
	
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		// 检查Token是否过期
		if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
			return nil, fmt.Errorf("token expired")
		}
		return claims, nil
	}
	
	return nil, fmt.Errorf("invalid token")
}

// getClientID 获取客户端标识
func (m *AuthMiddleware) getClientID(ctx *gin.Context) string {
	// 优先使用API密钥
	apiKey := m.getAPIKey(ctx)
	if apiKey != "" {
		return fmt.Sprintf("apikey:%s", apiKey)
	}
	
	// 使用IP地址作为后备
	ip := ctx.ClientIP()
	if ip == "" {
		ip = "unknown"
	}
	
	return fmt.Sprintf("ip:%s", ip)
}

// incrementRateLimit 增加限流计数
func (m *AuthMiddleware) incrementRateLimit(ctx *gin.Context, key string, window time.Duration) (int64, error) {
	// 这里应该实现Redis限流逻辑
	// 暂时返回0用于测试
	return 0, nil
}

// JWTClaims JWT声明
type JWTClaims struct {
	UserID      uint64 `json:"user_id"`
	UserAddress string `json:"user_address"`
	Role        string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateJWTToken 生成JWT Token
func (m *AuthMiddleware) GenerateJWTToken(userID uint64, userAddress, role string) (string, error) {
	expirationTime := time.Now().Add(time.Duration(m.config.JWTExpireHours) * time.Hour)
	
	claims := &JWTClaims{
		UserID:      userID,
		UserAddress: userAddress,
		Role:        role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "defi-asset-service",
		},
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.config.JWTSecret))
}

// 导入必要的包
import (
	"fmt"
)