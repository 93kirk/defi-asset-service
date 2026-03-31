package models

import (
	"time"
)

// Protocol 协议模型
type Protocol struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProtocolID      string    `gorm:"size:100;uniqueIndex;not null" json:"protocol_id"`
	Name            string    `gorm:"size:200;not null" json:"name"`
	Description     string    `gorm:"type:text" json:"description"`
	Category        string    `gorm:"size:50;not null" json:"category"`
	LogoURL         string    `gorm:"size:500" json:"logo_url"`
	WebsiteURL      string    `gorm:"size:500" json:"website_url"`
	TwitterURL      string    `gorm:"size:500" json:"twitter_url"`
	GitHubURL       string    `gorm:"size:500" json:"github_url"`
	TvlUSD          float64   `gorm:"type:decimal(30,6);default:0" json:"tvl_usd"`
	RiskLevel       int8      `gorm:"default:3" json:"risk_level"`
	IsActive        bool      `gorm:"default:true" json:"is_active"`
	SupportedChains JSON      `gorm:"type:json" json:"supported_chains"`
	Metadata        JSON      `gorm:"type:json" json:"metadata"`
	SyncSource      string    `gorm:"size:50;default:'debank'" json:"sync_source"`
	SyncVersion     int       `gorm:"default:1" json:"sync_version"`
	LastSyncedAt    time.Time `json:"last_synced_at"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// ProtocolToken 协议代币模型
type ProtocolToken struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProtocolID      string    `gorm:"size:100;index;not null" json:"protocol_id"`
	ChainID         int       `gorm:"not null" json:"chain_id"`
	TokenAddress    string    `gorm:"size:42;not null" json:"token_address"`
	TokenSymbol     string    `gorm:"size:20;not null" json:"token_symbol"`
	TokenName       string    `gorm:"size:100;not null" json:"token_name"`
	TokenDecimals   int       `gorm:"default:18" json:"token_decimals"`
	IsCollateral    bool      `gorm:"default:false" json:"is_collateral"`
	IsBorrowable    bool      `gorm:"default:false" json:"is_borrowable"`
	IsSupply        bool      `gorm:"default:false" json:"is_supply"`
	SupplyAPY       float64   `gorm:"type:decimal(10,4);default:0" json:"supply_apy"`
	BorrowAPY       float64   `gorm:"type:decimal(10,4);default:0" json:"borrow_apy"`
	LiquidationThreshold float64 `gorm:"type:decimal(10,4);default:0" json:"liquidation_threshold"`
	CollateralFactor float64  `gorm:"type:decimal(10,4);default:0" json:"collateral_factor"`
	PriceUSD        float64   `gorm:"type:decimal(30,6);default:0" json:"price_usd"`
	TvlUSD          float64   `gorm:"type:decimal(30,6);default:0" json:"tvl_usd"`
	Metadata        JSON      `gorm:"type:json" json:"metadata"`
	LastUpdatedAt   time.Time `gorm:"autoUpdateTime" json:"last_updated_at"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// SyncRecord 同步记录模型
type SyncRecord struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SyncType     string    `gorm:"size:50;index;not null" json:"sync_type"`
	SyncSource   string    `gorm:"size:50;index;not null" json:"sync_source"`
	TargetID     string    `gorm:"size:100;index" json:"target_id"`
	Status       string    `gorm:"size:20;index;not null" json:"status"`
	TotalCount   int       `gorm:"default:0" json:"total_count"`
	SuccessCount int       `gorm:"default:0" json:"success_count"`
	FailedCount  int       `gorm:"default:0" json:"failed_count"`
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	StartedAt    time.Time `gorm:"autoCreateTime" json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	DurationMs   int       `gorm:"default:0" json:"duration_ms"`
	Metadata     JSON      `gorm:"type:json" json:"metadata"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// SyncStatus 同步状态常量
const (
	SyncStatusPending   = "pending"
	SyncStatusRunning   = "running"
	SyncStatusSuccess   = "success"
	SyncStatusFailed    = "failed"
	SyncStatusCancelled = "cancelled"
)

// SyncType 同步类型常量
const (
	SyncTypeProtocolMetadata = "protocol_metadata"
	SyncTypeProtocolTokens   = "protocol_tokens"
	SyncTypeUserPositions    = "user_positions"
	SyncTypeFull             = "full"
	SyncTypeIncremental      = "incremental"
)

// JSON 类型别名，用于GORM JSON字段
type JSON []byte

// MarshalJSON 实现JSON序列化
func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON 实现JSON反序列化
func (j *JSON) UnmarshalJSON(data []byte) error {
	if j == nil {
		return nil
	}
	*j = append((*j)[0:0], data...)
	return nil
}

// Scan 实现数据库扫描接口
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	s, ok := value.([]byte)
	if !ok {
		*j = JSON{}
		return nil
	}
	*j = JSON(s)
	return nil
}

// Value 实现数据库值接口
func (j JSON) Value() (interface{}, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}