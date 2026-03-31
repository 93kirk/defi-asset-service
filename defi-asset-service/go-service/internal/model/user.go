package model

import (
	"time"
)

// User 用户模型
type User struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Address         string    `gorm:"size:42;not null;uniqueIndex:uk_address_chain" json:"address"`
	ChainID         int       `gorm:"not null;default:1;uniqueIndex:uk_address_chain" json:"chain_id"`
	Nickname        string    `gorm:"size:100" json:"nickname,omitempty"`
	AvatarURL       string    `gorm:"size:500" json:"avatar_url,omitempty"`
	TotalAssetsUSD  float64   `gorm:"type:decimal(30,6);default:0" json:"total_assets_usd"`
	LastUpdatedAt   time.Time `gorm:"index:idx_last_updated" json:"last_updated_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName 返回表名
func (User) TableName() string {
	return "users"
}

// UserAsset 用户资产模型（服务A数据）
type UserAsset struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        uint64    `gorm:"not null;index:idx_user_chain" json:"user_id"`
	ChainID       int       `gorm:"not null;index:idx_user_chain" json:"chain_id"`
	TokenAddress  string    `gorm:"size:42;not null;index:idx_token_address" json:"token_address"`
	TokenSymbol   string    `gorm:"size:20;not null" json:"token_symbol"`
	TokenName     string    `gorm:"size:100;not null" json:"token_name"`
	TokenDecimals int       `gorm:"not null;default:18" json:"token_decimals"`
	BalanceRaw    string    `gorm:"size:100;not null" json:"balance_raw"`
	BalanceDecimal float64   `gorm:"type:decimal(30,18);not null" json:"balance_decimal"`
	PriceUSD      float64   `gorm:"type:decimal(30,6);default:0" json:"price_usd"`
	ValueUSD      float64   `gorm:"type:decimal(30,6);default:0" json:"value_usd"`
	ProtocolID    string    `gorm:"size:100;index:idx_user_protocol" json:"protocol_id,omitempty"`
	AssetType     string    `gorm:"size:50;default:'token'" json:"asset_type"`
	Source        string    `gorm:"size:20;default:'service_a'" json:"source"`
	QueriedAt     time.Time `gorm:"index:idx_queried_at" json:"queried_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// TableName 返回表名
func (UserAsset) TableName() string {
	return "user_assets"
}

// UserPosition 用户仓位模型（服务B数据）
type UserPosition struct {
	ID                 uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID             uint64    `gorm:"not null;index:idx_user_id" json:"user_id"`
	ProtocolID         string    `gorm:"size:100;not null;index:idx_protocol_id" json:"protocol_id"`
	PositionID         string    `gorm:"size:200;not null;uniqueIndex:uk_user_protocol_position" json:"position_id"`
	PositionType       string    `gorm:"size:50;not null;index:idx_position_type" json:"position_type"`
	TokenAddress       string    `gorm:"size:42;not null" json:"token_address"`
	TokenSymbol        string    `gorm:"size:20;not null" json:"token_symbol"`
	AmountRaw          string    `gorm:"size:100;not null" json:"amount_raw"`
	AmountDecimal      float64   `gorm:"type:decimal(30,18);not null" json:"amount_decimal"`
	PriceUSD           float64   `gorm:"type:decimal(30,6);default:0" json:"price_usd"`
	ValueUSD           float64   `gorm:"type:decimal(30,6);default:0;index:idx_value_usd" json:"value_usd"`
	Apy                float64   `gorm:"type:decimal(10,4);default:0" json:"apy"`
	HealthFactor       float64   `gorm:"type:decimal(10,4);default:0" json:"health_factor"`
	LiquidationThreshold float64 `gorm:"type:decimal(10,4);default:0" json:"liquidation_threshold"`
	CollateralFactor   float64   `gorm:"type:decimal(10,4);default:0" json:"collateral_factor"`
	PositionData       JSON      `gorm:"type:json" json:"position_data,omitempty"`
	IsActive           bool      `gorm:"default:true;index:idx_is_active" json:"is_active"`
	LastUpdatedBy      string    `gorm:"size:50;default:'service_b'" json:"last_updated_by"`
	LastUpdatedAt      time.Time `gorm:"index:idx_last_updated" json:"last_updated_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// TableName 返回表名
func (UserPosition) TableName() string {
	return "user_positions"
}

// UserAssetSummary 用户资产汇总
type UserAssetSummary struct {
	UserID               uint64    `json:"user_id"`
	Address              string    `json:"address"`
	ChainID              int       `json:"chain_id"`
	ProtocolCount        int       `json:"protocol_count"`
	PositionCount        int       `json:"position_count"`
	TotalPositionValue   float64   `json:"total_position_value"`
	TotalAssetValue      float64   `json:"total_asset_value"`
	TotalValueUSD        float64   `json:"total_value_usd"`
	LastUpdatedAt        time.Time `json:"last_updated_at"`
}

// AssetResponse 资产响应
type AssetResponse struct {
	TokenAddress  string  `json:"token_address"`
	TokenSymbol   string  `json:"token_symbol"`
	TokenName     string  `json:"token_name"`
	TokenDecimals int     `json:"token_decimals"`
	Balance       string  `json:"balance"`
	BalanceRaw    string  `json:"balance_raw"`
	PriceUSD      float64 `json:"price_usd"`
	ValueUSD      float64 `json:"value_usd"`
	ProtocolID    string  `json:"protocol_id,omitempty"`
	AssetType     string  `json:"asset_type"`
	QueriedAt     string  `json:"queried_at"`
}

// PositionResponse 仓位响应
type PositionResponse struct {
	ProtocolID          string  `json:"protocol_id"`
	ProtocolName        string  `json:"protocol_name"`
	PositionID          string  `json:"position_id"`
	PositionType        string  `json:"position_type"`
	TokenAddress        string  `json:"token_address"`
	TokenSymbol         string  `json:"token_symbol"`
	TokenName           string  `json:"token_name"`
	Amount              string  `json:"amount"`
	AmountRaw           string  `json:"amount_raw"`
	PriceUSD            float64 `json:"price_usd"`
	ValueUSD            float64 `json:"value_usd"`
	Apy                 float64 `json:"apy"`
	HealthFactor        float64 `json:"health_factor"`
	LiquidationThreshold float64 `json:"liquidation_threshold"`
	CollateralFactor    float64 `json:"collateral_factor"`
	IsActive            bool    `json:"is_active"`
	LastUpdatedAt       string  `json:"last_updated_at"`
}

// UserSummaryResponse 用户汇总响应
type UserSummaryResponse struct {
	User        UserSummary        `json:"user"`
	Assets      []AssetResponse    `json:"assets,omitempty"`
	Positions   []PositionResponse `json:"positions,omitempty"`
}

// UserSummary 用户汇总信息
type UserSummary struct {
	Address              string  `json:"address"`
	ChainID              int     `json:"chain_id"`
	TotalValueUSD        float64 `json:"total_value_usd"`
	TotalAssetValueUSD   float64 `json:"total_asset_value_usd"`
	TotalPositionValueUSD float64 `json:"total_position_value_usd"`
	ProtocolCount        int     `json:"protocol_count"`
	PositionCount        int     `json:"position_count"`
	LastUpdatedAt        string  `json:"last_updated_at"`
}

// BatchAssetRequest 批量资产请求
type BatchAssetRequest struct {
	Addresses       []string `json:"addresses" binding:"required,min=1,max=50"`
	ChainID         int      `json:"chain_id" binding:"required,min=1"`
	IncludeAssets   bool     `json:"include_assets"`
	IncludePositions bool    `json:"include_positions"`
}

// BatchAssetResponse 批量资产响应
type BatchAssetResponse struct {
	Results   []UserSummaryResponse `json:"results"`
	QueriedAt string                `json:"queried_at"`
}

// JSON 自定义JSON类型
type JSON map[string]interface{}

// Scan 实现sql.Scanner接口
func (j *JSON) Scan(value interface{}) error {
	return scanJSON(value, j)
}

// Value 实现driver.Valuer接口
func (j JSON) Value() (interface{}, error) {
	return valueJSON(j)
}

// scanJSON 扫描JSON值
func scanJSON(value interface{}, target *JSON) error {
	if value == nil {
		*target = nil
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSON: value is not []byte")
	}
	
	if len(bytes) == 0 {
		*target = nil
		return nil
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	
	*target = result
	return nil
}

// valueJSON 转换JSON值
func valueJSON(j JSON) (interface{}, error) {
	if j == nil {
		return nil, nil
	}
	
	bytes, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	return string(bytes), nil
}

// 导入必要的包
import (
	"encoding/json"
	"fmt"
)