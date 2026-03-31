# DeFi资产展示服务 - API接口规范

## 1. 概述

### 1.1 API基础信息
- **基础URL**: `https://api.defi-asset-service.com/v1`
- **协议**: HTTPS
- **数据格式**: JSON
- **字符编码**: UTF-8

### 1.2 认证方式
所有API请求都需要在Header中携带认证信息：

```http
Authorization: Bearer {api_key}
X-API-Key: {api_key}
```

### 1.3 公共Header
| Header | 说明 | 必填 |
|--------|------|------|
| `Content-Type` | 请求体类型，固定为 `application/json` | 是 |
| `Accept` | 响应类型，固定为 `application/json` | 是 |
| `X-Request-ID` | 请求唯一ID，用于追踪 | 否 |
| `X-Timestamp` | 请求时间戳（Unix秒） | 否 |

### 1.4 响应格式
#### 成功响应
```json
{
  "code": 0,
  "message": "success",
  "data": {...},
  "timestamp": 1678886400
}
```

#### 错误响应
```json
{
  "code": 1001,
  "message": "错误描述",
  "data": null,
  "timestamp": 1678886400
}
```

### 1.5 错误码体系
| 错误码范围 | 类别 | 说明 |
|-----------|------|------|
| 0 | 成功 | 请求成功 |
| 1-999 | 系统错误 | 系统内部错误 |
| 1000-1999 | 认证错误 | 认证、权限相关错误 |
| 2000-2999 | 参数错误 | 请求参数错误 |
| 3000-3999 | 业务错误 | 业务逻辑错误 |
| 4000-4999 | 外部服务错误 | 依赖服务错误 |

## 2. 用户相关API

### 2.1 获取用户资产总览
获取用户在DeFi协议中的总资产情况。

**Endpoint**: `GET /users/{address}/summary`

**Path Parameters**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `address` | string | 是 | 用户钱包地址 |

**Query Parameters**:
| 参数 | 类型 | 必填 | 说明 | 默认值 |
|------|------|------|------|--------|
| `chain_id` | integer | 否 | 链ID | 1（以太坊主网） |
| `include_assets` | boolean | 否 | 是否包含资产详情 | false |
| `include_positions` | boolean | 否 | 是否包含仓位详情 | false |

**Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user": {
      "address": "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
      "chain_id": 1,
      "total_value_usd": "125430.25",
      "total_asset_value_usd": "85430.15",
      "total_position_value_usd": "40000.10",
      "protocol_count": 8,
      "position_count": 12,
      "last_updated_at": "2026-03-29T10:30:00Z"
    },
    "assets": [
      {
        "token_address": "0x...",
        "token_symbol": "ETH",
        "token_name": "Ethereum",
        "balance": "2.5",
        "price_usd": "3200.50",
        "value_usd": "8001.25",
        "protocol_id": null,
        "asset_type": "token"
      }
    ],
    "positions": [
      {
        "protocol_id": "aave",
        "protocol_name": "Aave",
        "position_type": "supply",
        "token_symbol": "USDC",
        "amount": "10000",
        "value_usd": "10000",
        "apy": "3.25",
        "health_factor": "2.5"
      }
    ]
  },
  "timestamp": 1678886400
}
```

### 2.2 获取用户实时资产（服务A）
获取用户有balance概念的协议资产（实时查询）。

**Endpoint**: `GET /users/{address}/assets`

**Path Parameters**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `address` | string | 是 | 用户钱包地址 |

**Query Parameters**:
| 参数 | 类型 | 必填 | 说明 | 默认值 |
|------|------|------|------|--------|
| `chain_id` | integer | 否 | 链ID | 1 |
| `protocol_id` | string | 否 | 协议ID过滤 | 空 |
| `token_address` | string | 否 | 代币地址过滤 | 空 |

**Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "address": "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
    "chain_id": 1,
    "total_value_usd": "85430.15",
    "assets": [
      {
        "token_address": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
        "token_symbol": "WETH",
        "token_name": "Wrapped Ether",
        "token_decimals": 18,
        "balance_raw": "2500000000000000000",
        "balance": "2.5",
        "price_usd": "3200.50",
        "value_usd": "8001.25",
        "protocol_id": null,
        "asset_type": "token",
        "queried_at": "2026-03-29T10:30:00Z"
      }
    ],
    "queried_at": "2026-03-29T10:30:00Z"
  },
  "timestamp": 1678886400
}
```

### 2.3 获取用户协议仓位（服务B）
获取用户无balance概念的协议仓位数据（带缓存）。

**Endpoint**: `GET /users/{address}/positions`

**Path Parameters**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `address` | string | 是 | 用户钱包地址 |

**Query Parameters**:
| 参数 | 类型 | 必填 | 说明 | 默认值 |
|------|------|------|------|--------|
| `chain_id` | integer | 否 | 链ID | 1 |
| `protocol_id` | string | 否 | 协议ID过滤 | 空 |
| `position_type` | string | 否 | 仓位类型过滤 | 空 |
| `refresh` | boolean | 否 | 强制刷新缓存 | false |

**Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "address": "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
    "chain_id": 1,
    "total_value_usd": "40000.10",
    "positions": [
      {
        "protocol_id": "aave",
        "protocol_name": "Aave",
        "position_id": "aave_supply_usdc_0x...",
        "position_type": "supply",
        "token_address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
        "token_symbol": "USDC",
        "token_name": "USD Coin",
        "amount_raw": "10000000000",
        "amount": "10000",
        "price_usd": "1.00",
        "value_usd": "10000",
        "apy": "3.25",
        "health_factor": "2.5",
        "liquidation_threshold": "0.85",
        "collateral_factor": "0.80",
        "is_active": true,
        "last_updated_at": "2026-03-29T10:25:00Z"
      }
    ],
    "cached": true,
    "cache_expires_at": "2026-03-29T10:35:00Z",
    "last_updated_at": "2026-03-29T10:25:00Z"
  },
  "timestamp": 1678886400
}
```

### 2.4 批量查询用户资产
批量查询多个用户的资产情况。

**Endpoint**: `POST /users/batch/assets`

**Request Body**:
```json
{
  "addresses": [
    "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
    "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Af"
  ],
  "chain_id": 1,
  "include_assets": true,
  "include_positions": true
}
```

**Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "results": [
      {
        "address": "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
        "total_value_usd": "125430.25",
        "assets": [...],
        "positions": [...]
      }
    ],
    "queried_at": "2026-03-29T10:30:00Z"
  },
  "timestamp": 1678886400
}
```

## 3. 协议相关API

### 3.1 获取协议列表
获取所有支持的协议列表。

**Endpoint**: `GET /protocols`

**Query Parameters**:
| 参数 | 类型 | 必填 | 说明 | 默认值 |
|------|------|------|------|--------|
| `category` | string | 否 | 协议类别过滤 | 空 |
| `chain_id` | integer | 否 | 链ID过滤 | 空 |
| `is_active` | boolean | 否 | 是否只返回活跃协议 | true |
| `page` | integer | 否 | 页码 | 1 |
| `page_size` | integer | 否 | 每页数量 | 20 |

**Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "protocols": [
      {
        "protocol_id": "aave",
        "name": "Aave",
        "description": "开源非托管流动性协议",
        "category": "lending",
        "logo_url": "https://app.aave.com/favicon.ico",
        "website_url": "https://aave.com",
        "twitter_url": "https://twitter.com/aave",
        "github_url": "https://github.com/aave",
        "tvl_usd": "12500000000",
        "risk_level": 2,
        "is_active": true,
        "supported_chains": [1, 137, 42161],
        "last_synced_at": "2026-03-29T02:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "page_size": 20,
      "total": 150,
      "total_pages": 8
    }
  },
  "timestamp": 1678886400
}
```

### 3.2 获取协议详情
获取指定协议的详细信息。

**Endpoint**: `GET /protocols/{protocol_id}`

**Path Parameters**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `protocol_id` | string | 是 | 协议ID |

**Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "protocol": {
      "protocol_id": "aave",
      "name": "Aave",
      "description": "开源非托管流动性协议",
      "category": "lending",
      "logo_url": "https://app.aave.com/favicon.ico",
      "website_url": "https://aave.com",
      "twitter_url": "https://twitter.com/aave",
      "github_url": "https://github.com/aave",
      "tvl_usd": "12500000000",
      "risk_level": 2,
      "is_active": true,
      "supported_chains": [1, 137, 42161],
      "metadata": {
        "audit_reports": [...],
        "contract_addresses": {...},
        "governance_token": "AAVE"
      },
      "tokens": [
        {
          "token_address": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
          "token_symbol": "WETH",
          "token_name": "Wrapped Ether",
          "is_collateral": true,
          "is_borrowable": true,
          "supply_apy": "2.15",
          "borrow_apy": "3.45",
          "liquidation_threshold": "0.85",
          "price_usd": "3200.50",
          "tvl_usd": "4500000000"
        }
      ],
      "statistics": {
        "user_count": 125000,
        "position_count": 350000,
        "total_position_value": "8500000000",
        "avg_apy": "2.85"
      },
      "last_synced_at": "2026-03-29T02:00:00Z",
      "created_at": "2026-01-15T00:00:00Z",
      "updated_at": "2026-03-29T02:00:00Z"
    }
  },
  "timestamp": 1678886400
}
```

### 3.3 获取协议代币列表
获取协议支持的代币列表。

**Endpoint**: `GET /protocols/{protocol_id}/tokens`

**Path Parameters**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `protocol_id` | string | 是 | 协议ID |

**Query Parameters**:
| 参数 | 类型 | 必填 | 说明 | 默认值 |
|------|------|------|------|--------|
| `chain_id` | integer | 否 | 链ID过滤 | 空 |
| `is_collateral` | boolean | 否 | 是否可作为抵押品 | 空 |
| `is_borrowable` | boolean | 否 | 是否可借出 | 空 |

**Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "protocol_id": "aave",
    "tokens": [
      {
        "token_address": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
        "token_symbol": "WETH",
        "token_name": "Wrapped Ether",
        "token_decimals": 18,
        "is_collateral": true,
        "is_borrowable": true,
        "is_supply": true,
        "supply_apy": "2.15",
        "borrow_apy": "3.45",
        "liquidation_threshold": "0.85",
        "collateral_factor": "0.80",
        "price_usd": "3200.50",
        "tvl_usd": "4500000000",
        "last_updated_at": "2026-03-29T10:00:00Z"
      }
    ],
    "total": 25
  },
  "timestamp": 1678886400
}
```

## 4. 管理相关API

### 4.1 触发协议元数据同步
手动触发协议元数据同步。

**Endpoint**: `POST /admin/sync/protocols`

**Request Body**:
```json
{
  "force_full_sync": false,
  "protocol_ids": ["aave", "compound"]
}
```

**Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "sync_id": "sync_1234567890",
    "status": "started",
    "estimated_time": 300,
    "started_at": "2026-03-29T10:30:00Z"
  },
  "timestamp": 1678886400
}
```

### 4.2 获取同步状态
获取同步任务状态。

**Endpoint**: `GET /admin/sync/{sync_id}`

**Path Parameters**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sync_id` | string | 是 | 同步任务ID |

**Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "sync_id": "sync_1234567890",
    "sync_type": "protocol_metadata",
    "sync_source": "debank",
    "status": "processing",
    "total_count": 150,
    "success_count": 85,
    "failed_count": 0,
    "progress": "56.67%",
    "started_at": "2026-03-29T10:30:00Z",
    "estimated_finish_at": "2026-03-29T10:35:00Z"
  },
  "timestamp": 1678886400
}
```

### 4.