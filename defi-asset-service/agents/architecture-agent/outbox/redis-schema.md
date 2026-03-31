# DeFi资产展示服务 - Redis数据结构设计

## 1. Redis配置概述

### 1.1 Redis版本要求
- **最低版本**: Redis 7.0+
- **推荐配置**: Redis Cluster或Sentinel模式
- **内存规划**: 根据数据量预估，建议16GB+内存

### 1.2 数据分区策略
| 数据类型 | 数据库编号 | TTL策略 | 内存预估 |
|----------|------------|---------|----------|
| 用户仓位缓存 | DB 0 | 10分钟 | 主要内存占用 |
| 协议元数据缓存 | DB 1 | 1小时 | 中等 |
| 实时数据缓存 | DB 2 | 5分钟 | 中等 |
| 队列数据 | DB 3 | 根据消息处理时间 | 较小 |
| 会话和锁 | DB 4 | 短期 | 较小 |

### 1.3 键命名规范
```
{namespace}:{entity_type}:{identifier}:{sub_identifier}
```
- **namespace**: `defi` (固定前缀)
- **entity_type**: 实体类型，如 `position`, `protocol`, `token`
- **identifier**: 主标识符，如用户地址、协议ID
- **sub_identifier**: 子标识符，可选

## 2. 缓存数据结构设计

### 2.1 用户仓位缓存 (DB 0)
存储服务B查询的协议仓位数据，避免频繁查询MySQL。

#### 2.1.1 仓位数据缓存
**键格式**: `defi:position:{user_address}:{protocol_id}`

**数据结构**: Hash
```redis
# 设置仓位缓存
HSET defi:position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:aave \
  data '{"positions": [...], "total_value": "40000.10"}' \
  cached_at '1678886400' \
  ttl '600'

# 设置过期时间
EXPIRE defi:position:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae:aave 600
```

**字段说明**:
| 字段 | 类型 | 说明 |
|------|------|------|
| `data` | string | JSON格式的仓位数据 |
| `cached_at` | string | 缓存时间戳（Unix秒） |
| `ttl` | string | TTL时间（秒） |

**TTL策略**:
- 默认TTL: 600秒（10分钟）
- 活跃用户: 300秒（5分钟）
- VIP用户: 60秒（1分钟）

#### 2.1.2 用户所有仓位缓存
**键格式**: `defi:positions:{user_address}`

**数据结构**: Hash
```redis
# 设置用户所有仓位
HSET defi:positions:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae \
  data '{"total_value": "125430.25", "positions": [...], "protocols": [...]}' \
  cached_at '1678886400' \
  ttl '300'

EXPIRE defi:positions:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae 300
```

### 2.2 协议元数据缓存 (DB 1)
存储协议基础信息，减少数据库查询。

#### 2.2.1 协议详情缓存
**键格式**: `defi:protocol:{protocol_id}`

**数据结构**: Hash
```redis
# 设置协议缓存
HSET defi:protocol:aave \
  data '{"name": "Aave", "category": "lending", ...}' \
  cached_at '1678886400' \
  version '2' \
  ttl '3600'

EXPIRE defi:protocol:aave 3600
```

**字段说明**:
| 字段 | 类型 | 说明 |
|------|------|------|
| `data` | string | JSON格式的协议数据 |
| `cached_at` | string | 缓存时间戳 |
| `version` | string | 数据版本号 |
| `ttl` | string | TTL时间（秒） |

**TTL策略**:
- 默认TTL: 3600秒（1小时）
- 热门协议: 1800秒（30分钟）
- 更新频繁协议: 900秒（15分钟）

#### 2.2.2 协议列表缓存
**键格式**: `defi:protocols:list:{category}:{page}`

**数据结构**: String (JSON数组)
```redis
# 设置协议列表缓存
SET defi:protocols:list:lending:1 '[...]'
EXPIRE defi:protocols:list:lending:1 1800
```

### 2.3 实时数据缓存 (DB 2)
存储价格、APY等实时变化的数据。

#### 2.3.1 代币价格缓存
**键格式**: `defi:price:{token_address}`

**数据结构**: Hash
```redis
# 设置代币价格
HSET defi:price:0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2 \
  price '3200.50' \
  source 'coingecko' \
  updated_at '1678886400' \
  ttl '300'

EXPIRE defi:price:0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2 300
```

**TTL策略**: 300秒（5分钟）

#### 2.3.2 APY数据缓存
**键格式**: `defi:apy:{protocol_id}:{token_address}`

**数据结构**: Hash
```redis
# 设置APY数据
HSET defi:apy:aave:0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2 \
  supply_apy '2.15' \
  borrow_apy '3.45' \
  updated_at '1678886400' \
  ttl '600'

EXPIRE defi:apy:aave:0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2 600
```

**TTL策略**: 600秒（10分钟）

### 2.4 防缓存穿透设计

#### 2.4.1 空值缓存
**键格式**: `defi:empty:{cache_key}`

**数据结构**: String
```redis
# 设置空值缓存（防止缓存穿透）
SET defi:empty:position:0x123...:invalid_protocol "1"
EXPIRE defi:empty:position:0x123...:invalid_protocol 60
```

**TTL策略**: 60秒（短暂缓存，避免频繁查询不存在的数据）

#### 2.4.2 布隆过滤器
**键格式**: `defi:bloom:positions`

**使用场景**: 快速判断用户是否有仓位数据
```redis
# 添加用户到布隆过滤器
BF.ADD defi:bloom:positions 0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae

# 检查用户是否存在
BF.EXISTS defi:bloom:positions 0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae
```

## 3. 队列数据结构设计 (DB 3)

### 3.1 Redis Streams消息队列
使用Redis Streams作为消息队列，处理服务B的实时更新。

#### 3.1.1 仓位更新队列
**Stream名称**: `defi:stream:position_updates`

**消息格式**:
```redis
# 添加消息到队列
XADD defi:stream:position_updates * \
  event_id "evt_1234567890" \
  event_type "position_update" \
  user_address "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae" \
  protocol_id "aave" \
  position_data '{"token": "USDC", "amount": "10000"}' \
  timestamp "1678886400"
```

**消费者组**:
```redis
# 创建消费者组
XGROUP CREATE defi:stream:position_updates position_workers $ MKSTREAM

# 消费者读取消息
XREADGROUP GROUP position_workers worker1 COUNT 1 STREAMS defi:stream:position_updates >
```

#### 3.1.2 死信队列
**Stream名称**: `defi:stream:dlq:position_updates`

**用途**: 存储处理失败的消息
```redis
# 将失败消息转移到死信队列
XADD defi:stream:dlq:position_updates * \
  original_message_id "1678886400-0" \
  error_reason "service_b_unavailable" \
  retry_count "3" \
  failed_at "1678886500" \
  original_data '...'
```

### 3.2 延迟队列设计
使用Sorted Set实现延迟队列。

#### 3.2.1 延迟任务队列
**键格式**: `defi:zset:delayed_tasks`

**数据结构**: Sorted Set (score为执行时间戳)
```redis
# 添加延迟任务
ZADD defi:zset:delayed_tasks 1678887000 '{"task_id": "task_123", "type": "cache_refresh", "data": {...}}'

# 获取到期的任务
ZRANGEBYSCORE defi:zset:delayed_tasks 0 1678886400 WITHSCORES
```

## 4. 分布式锁设计 (DB 4)

### 4.1 数据同步锁
防止多个实例同时执行数据同步任务。

**键格式**: `defi:lock:sync:{resource}`

**使用Redlock算法**:
```redis
# 尝试获取锁（SET with NX and PX）
SET defi:lock:sync:protocols "worker1" NX PX 30000

# 释放锁（需要验证持有者）
GET defi:lock:sync:protocols
DEL defi:lock:sync:protocols
```

**锁超时**: 30秒（根据任务复杂度调整）

### 4.2 用户数据更新锁
防止并发更新同一用户的数据。

**键格式**: `defi:lock:user:{user_address}`

```redis
# 获取用户锁
SET defi:lock:user:0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae "1" NX PX 5000
```

**锁超时**: 5秒（用户数据更新通常很快）

## 5. 会话和状态管理 (DB 4)

### 5.1 API限流计数器
**键格式**: `defi:rate_limit:{api_key}:{window}`

**数据结构**: String (计数器)
```redis
# 增加计数
INCR defi:rate_limit:apikey123:1678886400

# 设置过期时间（滑动窗口）
EXPIRE defi:rate_limit:apikey123:1678886400 3600

# 获取当前计数
GET defi:rate_limit:apikey123:1678886400
```

### 5.2 请求ID映射
**键格式**: `defi:request:{request_id}`

**数据结构**: Hash
```redis
# 存储请求信息
HSET defi:request:req_1234567890 \
  api_path "/users/0x.../positions" \
  user_address "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae" \
  started_at "1678886400" \
  status "processing"

EXPIRE defi:request:req_1234567890 86400
```

**TTL策略**: 24小时（用于请求追踪）

## 6. 性能优化策略

### 6.1 内存优化
1. **使用Hash压缩**: 将相关字段存储在Hash中，减少键数量
2. **数据压缩**: 对大型JSON数据使用压缩算法
3. **过期策略**: 合理设置TTL，避免内存泄漏
4. **内存淘汰策略**: 配置`maxmemory-policy`为`allkeys-lru`

### 6.2 查询优化
1. **Pipeline批量操作**: 减少网络往返次数
2. **Lua脚本**: 复杂操作用Lua脚本保证原子性
3. **连接池**: 使用连接池管理Redis连接
4. **读写分离**: 读操作优先访问从节点

### 6.3 监控指标
| 指标 | 监控项 | 告警阈值 |
|------|--------|----------|
| 内存使用率 | `used_memory` | > 80% |
| 连接数 | `connected_clients` | > 1000 |
| 命中率 | `keyspace_hits / total_commands` | < 90% |
| 延迟 | `latency_percentiles_us` | P95 > 10ms |
| 队列积压 | `xlen defi:stream:position_updates` | > 1000 |

## 7. 数据一致性保证

### 7.1 缓存更新策略
1. **Cache-Aside模式**:
   - 读: 先查缓存，未命中查DB，再写缓存
   - 写: 先写DB，再删缓存

2. **Write-Through模式**:
   - 写: 同时写缓存和DB
   - 读: 直接读缓存

### 7.2 缓存失效策略
1. **主动失效**: 数据更新时主动删除缓存
2. **被动失效**: 依赖TTL自动过期
3. **版本控制**: 缓存数据带版本号，版本不匹配时刷新

### 7.3 数据同步机制
1. **双写一致性**: 写DB和写缓存保证原子性
2. **补偿机制**: 缓存更新失败时记录日志，定时补偿
3. **最终一致性**: 通过队列保证数据最终一致

## 8. 灾难恢复

### 8.1 数据备份
1. **RDB快照**: 定时保存内存快照
2. **AOF日志**: 记录所有写操作
3. **混合持久化**: RDB + AOF

### 8.2 故障转移
1. **主从复制**: 配置多个从节点
2. **哨兵模式**: 自动故障检测和转移
3. **集群模式**: 数据分片，高可用

### 8.3 数据迁移
1. **渐进式迁移**: 逐步迁移热点数据
2. **双写策略**: 新旧系统同时写入
3. **数据验证**: 迁移后数据一致性验证

## 9. 运维命令示例

### 9.1 监控命令
```bash
# 查看内存使用
redis-cli INFO memory

# 查看命中率
redis-cli INFO stats | grep -E "(keyspace_hits|keyspace_misses)"

# 查看慢查询
redis-cli SLOWLOG GET 10

# 查看连接信息
redis-cli CLIENT LIST
```

### 9.2 维护命令
```bash
# 清空指定DB
redis-cli -n 0 FLUSHDB

# 查看键数量
redis-cli -n 0 DBSIZE

# 查找大Key
redis-cli --bigkeys

# 内存分析
redis-cli MEMORY USAGE defi:position:0x...
```

---

**文档版本**: v1.0  
**最后更新**: 2026-03-29  
**设计者**: 架构设计代理