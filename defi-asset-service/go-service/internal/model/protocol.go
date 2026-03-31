package model

import (
	"time"
)

// Protocol 协议模型
type Protocol struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProtocolID      string    `gorm:"size:100;not null;uniqueIndex:uk_protocol_id" json:"protocol_id"`
	Name            string    `gorm:"size:200;not null" json:"name"`
	Description     string    `gorm:"type:text" json:"description,omitempty"`
	Category        string    `gorm:"size:50;not null;index:idx_category" json:"category"`
	LogoURL         string    `gorm:"size:500" json:"logo_url,omitempty"`
	WebsiteURL      string    `gorm:"size:500" json:"website_url,omitempty"`
	TwitterURL      string    `gorm:"size:500" json:"twitter_url,omitempty"`
	GithubURL       string    `gorm:"size:500" json:"github_url,omitempty"`
	TvlUSD          float64   `gorm:"type:decimal(30,6);default:0;index:idx_tvl" json:"tvl_usd"`
	RiskLevel       int8      `gorm:"default:3" json:"risk_level"`
	IsActive        bool      `gorm:"default:true;index:idx_is_active" json:"is_active"`
	SupportedChains JSON      `gorm:"type:json" json:"supported_chains"`
	Metadata        JSON      `gorm:"type:json" json:"metadata,omitempty"`
	SyncSource      string    `gorm:"size:50;default:'debank'" json:"sync_source"`
	SyncVersion     int       `gorm:"default:1" json:"sync_version"`
	LastSyncedAt    time.Time `gorm:"index:idx_last_synced" json:"last_synced_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName 返回表名
func (Protocol) TableName() string {
	return "protocols"
}

// ProtocolToken 协议代币模型
type ProtocolToken struct {
	ID                  uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProtocolID          string    `gorm:"size:100;not null;index:idx_protocol_id" json:"protocol_id"`
	ChainID             int       `gorm:"not null;index:idx_chain_id" json:"chain_id"`
	TokenAddress        string    `gorm:"size:42;not null;index:idx_token_address" json:"token_address"`
	TokenSymbol         string    `gorm:"size:20;not null" json:"token_symbol"`
	TokenName           string    `gorm:"size:100;not null" json:"token_name"`
	TokenDecimals       int       `gorm:"not null;default:18" json:"token_decimals"`
	IsCollateral        bool      `gorm:"default:false;index:idx_is_collateral" json:"is_collateral"`
	IsBorrowable        bool      `gorm:"default:false;index:idx_is_borrowable" json:"is_borrowable"`
	IsSupply            bool      `gorm:"default:false" json:"is_supply"`
	SupplyApy           float64   `gorm:"type:decimal(10,4);default:0" json:"supply_apy"`
	BorrowApy           float64   `gorm:"type:decimal(10,4);default:0" json:"borrow_apy"`
	LiquidationThreshold float64   `gorm:"type:decimal(10,4);default:0" json:"liquidation_threshold"`
	CollateralFactor    float64   `gorm:"type:decimal(10,4);default:0" json:"collateral_factor"`
	PriceUSD            float64   `gorm:"type:decimal(30,6);default:0" json:"price_usd"`
	TvlUSD              float64   `gorm:"type:decimal(30,6);default:0" json:"tvl_usd"`
	Metadata            JSON      `gorm:"type:json" json:"metadata,omitempty"`
	LastUpdatedAt       time.Time `json:"last_updated_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// TableName 返回表名
func (ProtocolToken) TableName() string {
	return "protocol_tokens"
}

// ProtocolResponse 协议响应
type ProtocolResponse struct {
	ProtocolID      string           `json:"protocol_id"`
	Name            string           `json:"name"`
	Description     string           `json:"description,omitempty"`
	Category        string           `json:"category"`
	LogoURL         string           `json:"logo_url,omitempty"`
	WebsiteURL      string           `json:"website_url,omitempty"`
	TwitterURL      string           `json:"twitter_url,omitempty"`
	GithubURL       string           `json:"github_url,omitempty"`
	TvlUSD          float64          `json:"tvl_usd"`
	RiskLevel       int              `json:"risk_level"`
	IsActive        bool             `json:"is_active"`
	SupportedChains []int            `json:"supported_chains"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Tokens          []TokenResponse  `json:"tokens,omitempty"`
	Statistics      ProtocolStatistics `json:"statistics,omitempty"`
	LastSyncedAt    string           `json:"last_synced_at,omitempty"`
	CreatedAt       string           `json:"created_at"`
	UpdatedAt       string           `json:"updated_at"`
}

// TokenResponse 代币响应
type TokenResponse struct {
	TokenAddress        string  `json:"token_address"`
	TokenSymbol         string  `json:"token_symbol"`
	TokenName           string  `json:"token_name"`
	TokenDecimals       int     `json:"token_decimals"`
	IsCollateral        bool    `json:"is_collateral"`
	IsBorrowable        bool    `json:"is_borrowable"`
	IsSupply            bool    `json:"is_supply"`
	SupplyApy           float64 `json:"supply_apy"`
	BorrowApy           float64 `json:"borrow_apy"`
	LiquidationThreshold float64 `json:"liquidation_threshold"`
	CollateralFactor    float64 `json:"collateral_factor"`
	PriceUSD            float64 `json:"price_usd"`
	TvlUSD              float64 `json:"tvl_usd"`
	LastUpdatedAt       string  `json:"last_updated_at"`
}

// ProtocolStatistics 协议统计
type ProtocolStatistics struct {
	UserCount            int     `json:"user_count"`
	PositionCount        int     `json:"position_count"`
	TotalPositionValue   float64 `json:"total_position_value"`
	AvgApy               float64 `json:"avg_apy"`
	LastUpdatedAt        string  `json:"last_updated_at"`
}

// ProtocolListResponse 协议列表响应
type ProtocolListResponse struct {
	Protocols  []ProtocolResponse `json:"protocols"`
	Pagination Pagination         `json:"pagination"`
}

// Pagination 分页信息
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// ProtocolQuery 协议查询参数
type ProtocolQuery struct {
	Category  string `form:"category"`
	ChainID   int    `form:"chain_id"`
	IsActive  bool   `form:"is_active"`
	Page      int    `form:"page" binding:"min=1"`
	PageSize  int    `form:"page_size" binding:"min=1,max=100"`
}

// TokenQuery 代币查询参数
type TokenQuery struct {
	ChainID      int  `form:"chain_id"`
	IsCollateral bool `form:"is_collateral"`
	IsBorrowable bool `form:"is_borrowable"`
}

// SyncProtocolRequest 同步协议请求
type SyncProtocolRequest struct {
	ForceFullSync bool     `json:"force_full_sync"`
	ProtocolIDs   []string `json:"protocol_ids"`
}

// SyncResponse 同步响应
type SyncResponse struct {
	SyncID        string `json:"sync_id"`
	Status        string `json:"status"`
	EstimatedTime int    `json:"estimated_time"`
	StartedAt     string `json:"started_at"`
}

// SyncStatusResponse 同步状态响应
type SyncStatusResponse struct {
	SyncID        string `json:"sync_id"`
	SyncType      string `json:"sync_type"`
	SyncSource    string `json:"sync_source"`
	Status        string `json:"status"`
	TotalCount    int    `json:"total_count"`
	SuccessCount  int    `json:"success_count"`
	FailedCount   int    `json:"failed_count"`
	Progress      string `json:"progress"`
	StartedAt     string `json:"started_at"`
	EstimatedFinishAt string `json:"estimated_finish_at,omitempty"`
}