package unit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthService represents a mock authentication service
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) ValidateAPIKey(apiKey string) (string, error) {
	args := m.Called(apiKey)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) ValidateJWT(token string) (string, error) {
	args := m.Called(token)
	return args.String(0), args.Error(1)
}

// MockRateLimiter represents a mock rate limiter
type MockRateLimiter struct {
	mock.Mock
}

func (m *MockRateLimiter) Allow(key string) bool {
	args := m.Called(key)
	return args.Bool(0)
}

// MockMetricsCollector represents a mock metrics collector
type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) RecordRequest(method, path string, statusCode int, duration time.Duration) {
	m.Called(method, path, statusCode, duration)
}

func (m *MockMetricsCollector) RecordError(apiPath, errorType string) {
	m.Called(apiPath, errorType)
}

// APIGateway represents the API gateway
type APIGateway struct {
	authService      *MockAuthService
	rateLimiter      *MockRateLimiter
	metricsCollector *MockMetricsCollector
	serviceA         *ServiceA
	serviceB         *ServiceB
}

// NewAPIGateway creates a new API gateway instance
func NewAPIGateway(authService *MockAuthService, rateLimiter *MockRateLimiter, metricsCollector *MockMetricsCollector, serviceA *ServiceA, serviceB *ServiceB) *APIGateway {
	return &APIGateway{
		authService:      authService,
		rateLimiter:      rateLimiter,
		metricsCollector: metricsCollector,
		serviceA:         serviceA,
		serviceB:         serviceB,
	}
}

// APIResponse represents the standard API response
type APIResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// HandleGetUserAssets handles GET /users/{address}/assets
func (g *APIGateway) HandleGetUserAssets(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	// Extract address from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		g.sendErrorResponse(w, http.StatusBadRequest, 2001, "Invalid address")
		return
	}
	address := pathParts[3]
	
	// Validate address format
	if !isValidEthereumAddress(address) {
		g.sendErrorResponse(w, http.StatusBadRequest, 2002, "Invalid Ethereum address")
		return
	}
	
	// Extract query parameters
	chainID := 1 // default
	if chainIDStr := r.URL.Query().Get("chain_id"); chainIDStr != "" {
		// Parse chain ID
		// In real implementation, we would parse and validate
	}
	
	protocolID := r.URL.Query().Get("protocol_id")
	
	// Call ServiceA
	ctx := r.Context()
	response, err := g.serviceA.QueryAssets(ctx, address, chainID, protocolID)
	if err != nil {
		g.metricsCollector.RecordError("/users/{address}/assets", "service_a_error")
		g.sendErrorResponse(w, http.StatusInternalServerError, 4001, "Failed to query assets: "+err.Error())
		return
	}
	
	// Send success response
	g.sendSuccessResponse(w, response)
	
	// Record metrics
	duration := time.Since(startTime)
	g.metricsCollector.RecordRequest("GET", "/users/{address}/assets", http.StatusOK, duration)
}

// HandleGetUserPositions handles GET /users/{address}/positions
func (g *APIGateway) HandleGetUserPositions(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	// Extract address from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		g.sendErrorResponse(w, http.StatusBadRequest, 2001, "Invalid address")
		return
	}
	address := pathParts[3]
	
	// Validate address format
	if !isValidEthereumAddress(address) {
		g.sendErrorResponse(w, http.StatusBadRequest, 2002, "Invalid Ethereum address")
		return
	}
	
	// Extract query parameters
	chainID := 1 // default
	if chainIDStr := r.URL.Query().Get("chain_id"); chainIDStr != "" {
		// Parse chain ID
	}
	
	protocolID := r.URL.Query().Get("protocol_id")
	forceRefresh := r.URL.Query().Get("refresh") == "true"
	
	// Call ServiceB
	ctx := r.Context()
	positions, err := g.serviceB.QueryPositions(ctx, address, chainID, protocolID, forceRefresh)
	if err != nil {
		g.metricsCollector.RecordError("/users/{address}/positions", "service_b_error")
		g.sendErrorResponse(w, http.StatusInternalServerError, 4002, "Failed to query positions: "+err.Error())
		return
	}
	
	// Prepare response
	response := map[string]interface{}{
		"address":    address,
		"chain_id":   chainID,
		"positions":  positions,
		"cached":     !forceRefresh,
		"updated_at": time.Now().Format(time.RFC3339),
	}
	
	// Send success response
	g.sendSuccessResponse(w, response)
	
	// Record metrics
	duration := time.Since(startTime)
	g.metricsCollector.RecordRequest("GET", "/users/{address}/positions", http.StatusOK, duration)
}

// Middleware: Authentication
func (g *APIGateway) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check API key in header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			// Check Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				g.sendErrorResponse(w, http.StatusUnauthorized, 1001, "Authentication required")
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := g.authService.ValidateJWT(token)
			if err != nil {
				g.sendErrorResponse(w, http.StatusUnauthorized, 1002, "Invalid token")
				return
			}
			// Add user ID to context
			ctx := context.WithValue(r.Context(), "user_id", userID)
			r = r.WithContext(ctx)
		} else {
			userID, err := g.authService.ValidateAPIKey(apiKey)
			if err != nil {
				g.sendErrorResponse(w, http.StatusUnauthorized, 1003, "Invalid API key")
				return
			}
			// Add user ID to context
			ctx := context.WithValue(r.Context(), "user_id", userID)
			r = r.WithContext(ctx)
		}
		
		next(w, r)
	}
}

// Middleware: Rate limiting
func (g *APIGateway) RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		clientIP := r.RemoteAddr
		if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			clientIP = strings.Split(forwardedFor, ",")[0]
		}
		
		// Check rate limit
		if !g.rateLimiter.Allow(clientIP) {
			g.sendErrorResponse(w, http.StatusTooManyRequests, 1004, "Rate limit exceeded")
			return
		}
		
		next(w, r)
	}
}

// Helper function to send success response
func (g *APIGateway) sendSuccessResponse(w http.ResponseWriter, data interface{}) {
	response := APIResponse{
		Code:      0,
		Message:   "success",
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Helper function to send error response
func (g *APIGateway) sendErrorResponse(w http.ResponseWriter, statusCode, errorCode int, message string) {
	response := APIResponse{
		Code:      errorCode,
		Message:   message,
		Data:      nil,
		Timestamp: time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// Helper function to validate Ethereum address
func isValidEthereumAddress(address string) bool {
	// Simple validation - in real implementation, use proper validation
	return strings.HasPrefix(address, "0x") && len(address) == 42
}

// TestAPIGateway_Authentication tests authentication middleware
func TestAPIGateway_Authentication(t *testing.T) {
	t.Run("valid API key passes authentication", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		mockRateLimiter := new(MockRateLimiter)
		mockMetrics := new(MockMetricsCollector)
		mockServiceA := &ServiceA{}
		mockServiceB := &ServiceB{}
		
		mockAuth.On("ValidateAPIKey", "valid-api-key").Return("user-123", nil)
		mockRateLimiter.On("Allow", mock.Anything).Return(true)
		
		gateway := NewAPIGateway(mockAuth, mockRateLimiter, mockMetrics, mockServiceA, mockServiceB)
		
		req := httptest.NewRequest("GET", "/users/0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae/assets", nil)
		req.Header.Set("X-API-Key", "valid-api-key")
		rr := httptest.NewRecorder()
		
		handler := gateway.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Context().Value("user_id")
			assert.Equal(t, "user-123", userID)
			w.WriteHeader(http.StatusOK)
		})
		
		handler(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		mockAuth.AssertExpectations(t)
	})
	
	t.Run("invalid API key fails authentication", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		mockRateLimiter := new(MockRateLimiter)
		mockMetrics := new(MockMetricsCollector)
		mockServiceA := &ServiceA{}
		mockServiceB := &ServiceB{}
		
		mockAuth.On("ValidateAPIKey", "invalid-api-key").Return("", errors.New("invalid key"))
		
		gateway := NewAPIGateway(mockAuth, mockRateLimiter, mockMetrics, mockServiceA, mockServiceB)
		
		req := httptest.NewRequest("GET", "/users/0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae/assets", nil)
		req.Header.Set("X-API-Key", "invalid-api-key")
		rr := httptest.NewRecorder()
		
		handler := gateway.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called")
		})
		
		handler(rr, req)
		
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		var response APIResponse
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Equal(t, 1003, response.Code)
		assert.Contains(t, response.Message, "Invalid API key")
		mockAuth.AssertExpectations(t)
	})
	
	t.Run("valid Bearer token passes authentication", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		mockRateLimiter := new(MockRateLimiter)
		mockMetrics := new(MockMetricsCollector)
		mockServiceA := &ServiceA{}
		mockServiceB := &ServiceB{}
		
		mockAuth.On("ValidateJWT", "valid-jwt-token").Return("user-456", nil)
		mockRateLimiter.On("Allow", mock.Anything).Return(true)
		
		gateway := NewAPIGateway(mockAuth, mockRateLimiter, mockMetrics, mockServiceA, mockServiceB)
		
		req := httptest.NewRequest("GET", "/users/0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae/assets", nil)
		req.Header.Set("Authorization", "Bearer valid-jwt-token")
		rr := httptest.NewRecorder()
		
		handler := gateway.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Context().Value("user_id")
			assert.Equal(t, "user-456", userID)
			w.WriteHeader(http.StatusOK)
		})
		
		handler(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		mockAuth.AssertExpectations(t)
	})
	
	t.Run("no authentication headers fails", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		mockRateLimiter := new(MockRateLimiter)
		mockMetrics := new(MockMetricsCollector)
		mockServiceA := &ServiceA{}
		mockServiceB := &ServiceB{}
		
		gateway := NewAPIGateway(mockAuth, mockRateLimiter, mockMetrics, mockServiceA, mockServiceB)
		
		req := httptest.NewRequest("GET", "/users/0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae/assets", nil)
		rr := httptest.NewRecorder()
		
		handler := gateway.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called")
		})
		
		handler(rr, req)
		
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		var response APIResponse
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Equal(t, 1001, response.Code)
		assert.Contains(t, response.Message, "Authentication required")
	})
}

// TestAPIGateway_RateLimiting tests rate limiting middleware
func TestAPIGateway_RateLimiting(t *testing.T) {
	t.Run("allowed request passes rate limiting", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		mockRateLimiter := new(MockRateLimiter)
		mockMetrics := new(MockMetricsCollector)
		mockServiceA := &ServiceA{}
		mockServiceB := &ServiceB{}
		
		mockAuth.On("ValidateAPIKey", mock.Anything).Return("user-123", nil)
		mockRateLimiter.On("Allow", "127.0.0.1").Return(true)
		
		gateway := NewAPIGateway(mockAuth, mockRateLimiter, mockMetrics, mockServiceA, mockServiceB)
		
		req := httptest.NewRequest("GET", "/users/0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae/assets", nil)
		req.Header.Set("X-API-Key", "test-key")
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()
		
		handler := gateway.RateLimitMiddleware(gateway.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		handler(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		mockRateLimiter.AssertExpectations(t)
	})
	
	t.Run("rate limited request fails", func(t *testing.T) {
		mockAuth := new(MockAuthService)
		mockRateLimiter := new(MockRateLimiter)
		mockMetrics := new(MockMetricsCollector)
		mockServiceA := &ServiceA{}
		mockServiceB := &ServiceB{}
		
		mockRateLimiter.On("Allow", "127.0.0.1").Return(false)
		
		gateway := NewAPIGateway(mockAuth, mockRateLimiter, mockMetrics, mockServiceA, mockServiceB)
		
		req := httptest.NewRequest("GET", "/users/0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae/assets", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()
		
		handler := gateway.RateLimitMiddleware(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler should not be called")
		})
		
		handler(rr, req)
		
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
		var response APIResponse
		json.Unmarshal(rr.Body.Bytes(), &response)
		assert.Equal(t, 1004,