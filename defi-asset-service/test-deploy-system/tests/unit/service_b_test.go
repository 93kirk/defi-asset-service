package unit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCache represents a mock cache interface
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

// MockDatabase represents a mock database interface
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) SavePosition(ctx context.Context, position *Position) error {
	args := m.Called(ctx, position)
	return args.Error(0)
}

func (m *MockDatabase) GetPosition(ctx context.Context, userAddress, protocolID, positionID string) (*Position, error) {
	args := m.Called(ctx, userAddress, protocolID, positionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Position), args.Error(1)
}

// MockExternalService represents a mock external service B
type MockExternalService struct {
	mock.Mock
}

func (m *MockExternalService) QueryPositions(ctx context.Context, address string, chainID int) ([]Position, error) {
	args := m.Called(ctx, address, chainID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Position), args.Error(1)
}

// Position represents a user's protocol position
type Position struct {
	ProtocolID          string  `json:"protocol_id"`
	ProtocolName        string  `json:"protocol_name"`
	PositionID          string  `json:"position_id"`
	PositionType        string  `json:"position_type"`
	TokenAddress        string  `json:"token_address"`
	TokenSymbol         string  `json:"token_symbol"`
	Amount              string  `json:"amount"`
	PriceUSD            float64 `json:"price_usd"`
	ValueUSD            float64 `json:"value_usd"`
	APY                 float64 `json:"apy"`
	HealthFactor        float64 `json:"health_factor"`
	LiquidationThreshold float64 `json:"liquidation_threshold"`
	IsActive            bool    `json:"is_active"`
	LastUpdatedAt       string  `json:"last_updated_at"`
}

// ServiceB represents the service for querying position-based protocol data
type ServiceB struct {
	cache           *MockCache
	db              *MockDatabase
	externalService *MockExternalService
	defaultTTL      time.Duration
}

// NewServiceB creates a new ServiceB instance
func NewServiceB(cache *MockCache, db *MockDatabase, externalService *MockExternalService, defaultTTL time.Duration) *ServiceB {
	return &ServiceB{
		cache:           cache,
		db:              db,
		externalService: externalService,
		defaultTTL:      defaultTTL,
	}
}

// QueryPositions queries positions for a user with caching
func (s *ServiceB) QueryPositions(ctx context.Context, address string, chainID int, protocolID string, forceRefresh bool) ([]Position, error) {
	cacheKey := s.generateCacheKey(address, chainID, protocolID)

	// Check cache first if not forcing refresh
	if !forceRefresh {
		cachedData, err := s.cache.Get(ctx, cacheKey)
		if err == nil && cachedData != "" {
			var positions []Position
			if err := json.Unmarshal([]byte(cachedData), &positions); err == nil {
				return positions, nil
			}
		}
	}

	// Query external service
	positions, err := s.externalService.QueryPositions(ctx, address, chainID)
	if err != nil {
		return nil, err
	}

	// Filter by protocol if specified
	if protocolID != "" {
		filtered := make([]Position, 0)
		for _, pos := range positions {
			if pos.ProtocolID == protocolID {
				filtered = append(filtered, pos)
			}
		}
		positions = filtered
	}

	// Save to database
	for _, position := range positions {
		if err := s.db.SavePosition(ctx, &position); err != nil {
			// Log error but continue
			continue
		}
	}

	// Cache the results
	if data, err := json.Marshal(positions); err == nil {
		s.cache.Set(ctx, cacheKey, string(data), s.defaultTTL)
	}

	return positions, nil
}

// generateCacheKey generates a cache key for positions
func (s *ServiceB) generateCacheKey(address string, chainID int, protocolID string) string {
	if protocolID != "" {
		return "position:" + address + ":" + string(rune(chainID)) + ":" + protocolID
	}
	return "position:" + address + ":" + string(rune(chainID))
}

// TestServiceB_QueryPositions tests the QueryPositions method
func TestServiceB_QueryPositions(t *testing.T) {
	t.Run("cache hit returns cached data", func(t *testing.T) {
		mockCache := new(MockCache)
		mockDB := new(MockDatabase)
		mockExternal := new(MockExternalService)

		cachedPositions := []Position{
			{
				ProtocolID:   "aave",
				ProtocolName: "Aave",
				PositionID:   "aave_supply_usdc_123",
				PositionType: "supply",
				TokenSymbol:  "USDC",
				Amount:       "10000",
				ValueUSD:     10000.0,
				APY:          3.25,
				IsActive:     true,
			},
		}
		cachedData, _ := json.Marshal(cachedPositions)

		mockCache.On("Get", mock.Anything, "position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:1").Return(string(cachedData), nil)

		service := NewServiceB(mockCache, mockDB, mockExternal, 10*time.Minute)
		ctx := context.Background()

		positions, err := service.QueryPositions(ctx, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1, "", false)

		assert.NoError(t, err)
		assert.Len(t, positions, 1)
		assert.Equal(t, "aave", positions[0].ProtocolID)
		assert.Equal(t, "USDC", positions[0].TokenSymbol)
		mockCache.AssertExpectations(t)
		mockExternal.AssertNotCalled(t, "QueryPositions")
	})

	t.Run("cache miss queries external service", func(t *testing.T) {
		mockCache := new(MockCache)
		mockDB := new(MockDatabase)
		mockExternal := new(MockExternalService)

		// Cache miss
		mockCache.On("Get", mock.Anything, "position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:1").Return("", errors.New("not found"))

		// External service returns data
		externalPositions := []Position{
			{
				ProtocolID:   "compound",
				ProtocolName: "Compound",
				PositionID:   "compound_supply_dai_456",
				PositionType: "supply",
				TokenSymbol:  "DAI",
				Amount:       "5000",
				ValueUSD:     5000.0,
				APY:          2.15,
				IsActive:     true,
			},
		}
		mockExternal.On("QueryPositions", mock.Anything, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1).Return(externalPositions, nil)

		// Database save
		mockDB.On("SavePosition", mock.Anything, mock.AnythingOfType("*unit.Position")).Return(nil)

		// Cache set
		mockCache.On("Set", mock.Anything, "position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:1", mock.Anything, 10*time.Minute).Return(nil)

		service := NewServiceB(mockCache, mockDB, mockExternal, 10*time.Minute)
		ctx := context.Background()

		positions, err := service.QueryPositions(ctx, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1, "", false)

		assert.NoError(t, err)
		assert.Len(t, positions, 1)
		assert.Equal(t, "compound", positions[0].ProtocolID)
		assert.Equal(t, "DAI", positions[0].TokenSymbol)
		mockCache.AssertExpectations(t)
		mockExternal.AssertExpectations(t)
		mockDB.AssertExpectations(t)
	})

	t.Run("force refresh bypasses cache", func(t *testing.T) {
		mockCache := new(MockCache)
		mockDB := new(MockDatabase)
		mockExternal := new(MockExternalService)

		// External service returns data (cache should not be checked)
		externalPositions := []Position{
			{
				ProtocolID:   "uniswap",
				ProtocolName: "Uniswap V3",
				PositionID:   "uniswap_lp_789",
				PositionType: "lp",
				TokenSymbol:  "ETH/USDC",
				Amount:       "1.5",
				ValueUSD:     4800.0,
				APY:          15.25,
				IsActive:     true,
			},
		}
		mockExternal.On("QueryPositions", mock.Anything, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1).Return(externalPositions, nil)

		// Database save
		mockDB.On("SavePosition", mock.Anything, mock.AnythingOfType("*unit.Position")).Return(nil)

		// Cache set
		mockCache.On("Set", mock.Anything, "position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:1", mock.Anything, 10*time.Minute).Return(nil)

		service := NewServiceB(mockCache, mockDB, mockExternal, 10*time.Minute)
		ctx := context.Background()

		positions, err := service.QueryPositions(ctx, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1, "", true)

		assert.NoError(t, err)
		assert.Len(t, positions, 1)
		assert.Equal(t, "uniswap", positions[0].ProtocolID)
		mockCache.AssertNotCalled(t, "Get")
		mockExternal.AssertExpectations(t)
	})

	t.Run("protocol filtering works correctly", func(t *testing.T) {
		mockCache := new(MockCache)
		mockDB := new(MockDatabase)
		mockExternal := new(MockExternalService)

		// Cache miss
		mockCache.On("Get", mock.Anything, "position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:1:aave").Return("", errors.New("not found"))

		// External service returns multiple positions
		externalPositions := []Position{
			{
				ProtocolID:   "aave",
				ProtocolName: "Aave",
				PositionID:   "aave_supply_usdc_123",
				PositionType: "supply",
				TokenSymbol:  "USDC",
				Amount:       "10000",
				ValueUSD:     10000.0,
				APY:          3.25,
				IsActive:     true,
			},
			{
				ProtocolID:   "compound",
				ProtocolName: "Compound",
				PositionID:   "compound_supply_dai_456",
				PositionType: "supply",
				TokenSymbol:  "DAI",
				Amount:       "5000",
				ValueUSD:     5000.0,
				APY:          2.15,
				IsActive:     true,
			},
			{
				ProtocolID:   "aave",
				ProtocolName: "Aave",
				PositionID:   "aave_borrow_eth_789",
				PositionType: "borrow",
				TokenSymbol:  "ETH",
				Amount:       "1.5",
				ValueUSD:     4800.0,
				APY:          1.85,
				IsActive:     true,
			},
		}
		mockExternal.On("QueryPositions", mock.Anything, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1).Return(externalPositions, nil)

		// Database save (only for Aave positions)
		mockDB.On("SavePosition", mock.Anything, mock.MatchedBy(func(p *Position) bool {
			return p.ProtocolID == "aave"
		})).Return(nil).Times(2)

		// Cache set with filtered data
		mockCache.On("Set", mock.Anything, "position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:1:aave", mock.Anything, 10*time.Minute).Return(nil)

		service := NewServiceB(mockCache, mockDB, mockExternal, 10*time.Minute)
		ctx := context.Background()

		positions, err := service.QueryPositions(ctx, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1, "aave", false)

		assert.NoError(t, err)
		assert.Len(t, positions, 2)
		for _, pos := range positions {
			assert.Equal(t, "aave", pos.ProtocolID)
		}
		mockExternal.AssertExpectations(t)
		mockDB.AssertExpectations(t)
	})

	t.Run("external service error returns error", func(t *testing.T) {
		mockCache := new(MockCache)
		mockDB := new(MockDatabase)
		mockExternal := new(MockExternalService)

		// Cache miss
		mockCache.On("Get", mock.Anything, "position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:1").Return("", errors.New("not found"))

		// External service error
		mockExternal.On("QueryPositions", mock.Anything, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1).Return(nil, errors.New("service unavailable"))

		service := NewServiceB(mockCache, mockDB, mockExternal, 10*time.Minute)
		ctx := context.Background()

		positions, err := service.QueryPositions(ctx, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1, "", false)

		assert.Error(t, err)
		assert.Nil(t, positions)
		assert.Contains(t, err.Error(), "service unavailable")
		mockExternal.AssertExpectations(t)
		mockDB.AssertNotCalled(t, "SavePosition")
		mockCache.AssertNotCalled(t, "Set")
	})

	t.Run("cache set error doesn't fail the request", func(t *testing.T) {
		mockCache := new(MockCache)
		mockDB := new(MockDatabase)
		mockExternal := new(MockExternalService)

		// Cache miss
		mockCache.On("Get", mock.Anything, "position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:1").Return("", errors.New("not found"))

		// External service returns data
		externalPositions := []Position{
			{
				ProtocolID:   "aave",
				ProtocolName: "Aave",
				PositionID:   "aave_supply_usdc_123",
				PositionType: "supply",
				TokenSymbol:  "USDC",
				Amount:       "10000",
				ValueUSD:     10000.0,
				APY:          3.25,
				IsActive:     true,
			},
		}
		mockExternal.On("QueryPositions", mock.Anything, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1).Return(externalPositions, nil)

		// Database save
		mockDB.On("SavePosition", mock.Anything, mock.AnythingOfType("*unit.Position")).Return(nil)

		// Cache set fails
		mockCache.On("Set", mock.Anything, "position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:1", mock.Any