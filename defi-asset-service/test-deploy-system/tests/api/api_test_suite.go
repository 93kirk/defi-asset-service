package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// APITestSuite is the base test suite for API tests
type APITestSuite struct {
	suite.Suite
	server *httptest.Server
	client *http.Client
}

// SetupSuite runs once before all tests
func (suite *APITestSuite) SetupSuite() {
	suite.client = &http.Client{
		Timeout: 10 * time.Second,
	}
}

// TearDownSuite runs once after all tests
func (suite *APITestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// TestUserAPIs tests user-related APIs
type TestUserAPIs struct {
	APITestSuite
}

// TestGetUserAssets tests GET /users/{address}/assets
func (suite *TestUserAPIs) TestGetUserAssets() {
	tests := []struct {
		name           string
		address        string
		queryParams    map[string]string
		expectedStatus int
		expectedCode   int
		setupAuth      func(req *http.Request)
	}{
		{
			name:    "valid request with API key",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"chain_id":    "1",
				"protocol_id": "aave",
			},
			expectedStatus: http.StatusOK,
			expectedCode:   0,
			setupAuth: func(req *http.Request) {
				req.Header.Set("X-API-Key", "test-api-key-123")
			},
		},
		{
			name:           "invalid address format",
			address:        "invalid-address",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   2002,
			setupAuth: func(req *http.Request) {
				req.Header.Set("X-API-Key", "test-api-key-123")
			},
		},
		{
			name:           "missing authentication",
			address:        "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   1001,
			setupAuth:      func(req *http.Request) {},
		},
		{
			name:    "with refresh parameter",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"refresh": "true",
			},
			expectedStatus: http.StatusOK,
			expectedCode:   0,
			setupAuth: func(req *http.Request) {
				req.Header.Set("X-API-Key", "test-api-key-123")
			},
		},
		{
			name:    "multiple chain IDs",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"chain_id": "1,137,42161",
			},
			expectedStatus: http.StatusOK,
			expectedCode:   0,
			setupAuth: func(req *http.Request) {
				req.Header.Set("X-API-Key", "test-api-key-123")
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Build URL
			url := fmt.Sprintf("%s/users/%s/assets", suite.server.URL, tt.address)
			if len(tt.queryParams) > 0 {
				url += "?"
				first := true
				for key, value := range tt.queryParams {
					if !first {
						url += "&"
					}
					url += fmt.Sprintf("%s=%s", key, value)
					first = false
				}
			}

			// Create request
			req, err := http.NewRequest("GET", url, nil)
			suite.NoError(err)

			// Setup authentication
			tt.setupAuth(req)

			// Send request
			resp, err := suite.client.Do(req)
			suite.NoError(err)
			defer resp.Body.Close()

			// Verify status code
			suite.Equal(tt.expectedStatus, resp.StatusCode)

			// Parse response
			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			// Verify response code
			code, ok := response["code"].(float64)
			suite.True(ok)
			suite.Equal(float64(tt.expectedCode), code)

			// Additional assertions for successful requests
			if tt.expectedStatus == http.StatusOK {
				suite.Equal("success", response["message"])
				suite.Contains(response, "data")
				suite.Contains(response, "timestamp")
			}
		})
	}
}

// TestGetUserPositions tests GET /users/{address}/positions
func (suite *TestUserAPIs) TestGetUserPositions() {
	tests := []struct {
		name           string
		address        string
		queryParams    map[string]string
		expectedStatus int
		expectedFields []string
	}{
		{
			name:    "basic position query",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"chain_id": "1",
			},
			expectedStatus: http.StatusOK,
			expectedFields: []string{"address", "chain_id", "positions", "cached", "updated_at"},
		},
		{
			name:    "filter by protocol",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"protocol_id": "aave",
			},
			expectedStatus: http.StatusOK,
			expectedFields: []string{"address", "positions"},
		},
		{
			name:    "filter by position type",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"position_type": "supply",
			},
			expectedStatus: http.StatusOK,
			expectedFields: []string{"address", "positions"},
		},
		{
			name:    "force refresh",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"refresh": "true",
			},
			expectedStatus: http.StatusOK,
			expectedFields: []string{"address", "cached"},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Build URL
			url := fmt.Sprintf("%s/users/%s/positions", suite.server.URL, tt.address)
			if len(tt.queryParams) > 0 {
				url += "?"
				first := true
				for key, value := range tt.queryParams {
					if !first {
						url += "&"
					}
					url += fmt.Sprintf("%s=%s", key, value)
					first = false
				}
			}

			// Create request
			req, err := http.NewRequest("GET", url, nil)
			suite.NoError(err)
			req.Header.Set("X-API-Key", "test-api-key-123")

			// Send request
			resp, err := suite.client.Do(req)
			suite.NoError(err)
			defer resp.Body.Close()

			// Verify status code
			suite.Equal(tt.expectedStatus, resp.StatusCode)

			if tt.expectedStatus == http.StatusOK {
				// Parse response
				var response map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&response)
				suite.NoError(err)

				// Verify response structure
				suite.Equal(float64(0), response["code"])
				suite.Equal("success", response["message"])

				// Verify data contains expected fields
				data, ok := response["data"].(map[string]interface{})
				suite.True(ok)
				for _, field := range tt.expectedFields {
					suite.Contains(data, field)
				}

				// Verify positions array if present
				if positions, ok := data["positions"].([]interface{}); ok {
					for _, pos := range positions {
						position := pos.(map[string]interface{})
						suite.Contains(position, "protocol_id")
						suite.Contains(position, "position_type")
						suite.Contains(position, "token_symbol")
						suite.Contains(position, "value_usd")
					}
				}
			}
		})
	}
}

// TestGetUserSummary tests GET /users/{address}/summary
func (suite *TestUserAPIs) TestGetUserSummary() {
	tests := []struct {
		name           string
		address        string
		queryParams    map[string]string
		expectedStatus int
		validateFunc   func(data map[string]interface{})
	}{
		{
			name:    "basic summary",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"chain_id": "1",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(data map[string]interface{}) {
				suite.Contains(data, "user")
				suite.Contains(data, "total_value_usd")
				user := data["user"].(map[string]interface{})
				suite.Equal("0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", user["address"])
			},
		},
		{
			name:    "summary with assets",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"include_assets": "true",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(data map[string]interface{}) {
				suite.Contains(data, "assets")
				assets := data["assets"].([]interface{})
				suite.Greater(len(assets), 0)
			},
		},
		{
			name:    "summary with positions",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"include_positions": "true",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(data map[string]interface{}) {
				suite.Contains(data, "positions")
				positions := data["positions"].([]interface{})
				suite.Greater(len(positions), 0)
			},
		},
		{
			name:    "summary with both assets and positions",
			address: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
			queryParams: map[string]string{
				"include_assets":    "true",
				"include_positions": "true",
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(data map[string]interface{}) {
				suite.Contains(data, "assets")
				suite.Contains(data, "positions")
				assets := data["assets"].([]interface{})
				positions := data["positions"].([]interface{})
				suite.Greater(len(assets), 0)
				suite.Greater(len(positions), 0)
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Build URL
			url := fmt.Sprintf("%s/users/%s/summary", suite.server.URL, tt.address)
			if len(tt.queryParams) > 0 {
				url += "?"
				first := true
				for key, value := range tt.queryParams {
					if !first {
						url += "&"
					}
					url += fmt.Sprintf("%s=%s", key, value)
					first = false
				}
			}

			// Create request
			req, err := http.NewRequest("GET", url, nil)
			suite.NoError(err)
			req.Header.Set("X-API-Key", "test-api-key-123")

			// Send request
			resp, err := suite.client.Do(req)
			suite.NoError(err)
			defer resp.Body.Close()

			// Verify status code
			suite.Equal(tt.expectedStatus, resp.StatusCode)

			if tt.expectedStatus == http.StatusOK {
				// Parse response
				var response map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&response)
				suite.NoError(err)

				// Verify response structure
				suite.Equal(float64(0), response["code"])
				suite.Equal("success", response["message"])

				// Validate data
				data, ok := response["data"].(map[string]interface{})
				suite.True(ok)
				tt.validateFunc(data)
			}
		})
	}
}

// TestBatchUserAssets tests POST /users/batch/assets
func (suite *TestUserAPIs) TestBatchUserAssets() {
	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		validateFunc   func(data map[string]interface{})
	}{
		{
			name: "batch query two addresses",
			requestBody: map[string]interface{}{
				"addresses": []string{
					"0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
					"0x742d35Cc6634C0532925a3b844Bc9e90F1A904Af",
				},
				"chain_id": 1,
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(data map[string]interface{}) {
				suite.Contains(data, "results")
				results := data["results"].([]interface{})
				suite.Equal(2, len(results))
				for _, result := range results {
					res := result.(map[string]interface{})
					suite.Contains(res, "address")
					suite.Contains(res, "total_value_usd")
				}
			},
		},
		{
			name: "batch query with assets included",
			requestBody: map[string]interface{}{
				"addresses": []string{
					"0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
				},
				"chain_id":       1,
				"include_assets": true,
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(data map[string]interface{}) {
				suite.Contains(data, "results")
				results := data["results"].([]interface{})
				result := results[0].(map[string]interface{})
				suite.Contains(result, "assets")
				assets := result["assets"].([]interface{})
				suite.Greater(len(assets), 0)
			},
		},
		{
			name: "empty addresses array",
			requestBody: map[string]interface{}{
				"addresses": []string{},
				"chain_id":  1,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "too many addresses",
			requestBody: map[string]interface{}{
				"addresses": make([]string, 101),
				"chain_id":  1,
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Marshal request body
			body, err := json.Marshal(tt.requestBody)
			suite.NoError(err)

			// Create request
			req, err := http.NewRequest("POST", suite.server.URL+"/users/batch/assets", bytes.NewBuffer(body))
			suite.NoError(err)
			req.Header.Set("X-API-Key", "test-api-key-123")
			req.Header.Set("Content-Type", "application/json")

			// Send request
			resp, err := suite.client.Do(req)
			suite.NoError(err)
			defer resp.Body.Close()

			// Verify status code
			suite.Equal(tt.expectedStatus, resp.StatusCode)

			if tt.expectedStatus == http.StatusOK {
				// Parse response
				var response map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&response)
				suite.NoError(err)

				// Verify response structure
				suite.Equal(float64(0), response["code"])
				suite.Equal("success", response["message"])

				// Validate data
				data, ok := response["data"].(map[string]interface{})
				suite.True(ok)
				tt.validateFunc(data)
			}
		})
	}
}

//