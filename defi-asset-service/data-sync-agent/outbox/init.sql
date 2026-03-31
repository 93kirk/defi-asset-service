-- DeFi资产展示服务 - 数据库初始化脚本
-- 版本: v1.0

-- 创建数据库（如果不存在）
CREATE DATABASE IF NOT EXISTS defi_asset_service DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 使用数据库
USE defi_asset_service;

-- 创建用户表
CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户ID',
    address VARCHAR(42) NOT NULL COMMENT '用户钱包地址（0x开头）',
    chain_id INT NOT NULL DEFAULT 1 COMMENT '链ID（1=以太坊主网）',
    nickname VARCHAR(100) COMMENT '用户昵称',
    avatar_url VARCHAR(500) COMMENT '头像URL',
    total_assets_usd DECIMAL(30, 6) DEFAULT 0 COMMENT '总资产价值（USD）',
    last_updated_at TIMESTAMP NULL COMMENT '最后更新时间',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_address_chain (address, chain_id),
    KEY idx_last_updated (last_updated_at),
    KEY idx_total_assets (total_assets_usd)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 创建协议表
CREATE TABLE IF NOT EXISTS protocols (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '协议ID',
    protocol_id VARCHAR(100) NOT NULL COMMENT '协议唯一标识（如aave、compound）',
    name VARCHAR(200) NOT NULL COMMENT '协议名称',
    description TEXT COMMENT '协议描述',
    category VARCHAR(50) NOT NULL COMMENT '协议类别（lending、dex、yield等）',
    logo_url VARCHAR(500) COMMENT '协议Logo URL',
    website_url VARCHAR(500) COMMENT '官网URL',
    twitter_url VARCHAR(500) COMMENT 'Twitter URL',
    github_url VARCHAR(500) COMMENT 'GitHub URL',
    tvl_usd DECIMAL(30, 6) DEFAULT 0 COMMENT '总锁仓价值（USD）',
    risk_level TINYINT DEFAULT 3 COMMENT '风险等级（1-5，1最低）',
    is_active BOOLEAN DEFAULT TRUE COMMENT '是否活跃',
    supported_chains JSON COMMENT '支持的链ID列表',
    metadata JSON COMMENT '扩展元数据',
    sync_source VARCHAR(50) DEFAULT 'debank' COMMENT '同步来源',
    sync_version INT DEFAULT 1 COMMENT '同步版本',
    last_synced_at TIMESTAMP NULL COMMENT '最后同步时间',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_protocol_id (protocol_id),
    KEY idx_category (category),
    KEY idx_tvl (tvl_usd DESC),
    KEY idx_last_synced (last_synced_at),
    KEY idx_is_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='协议表';

-- 创建用户资产表
CREATE TABLE IF NOT EXISTS user_assets (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '资产ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    chain_id INT NOT NULL COMMENT '链ID',
    token_address VARCHAR(42) NOT NULL COMMENT '代币合约地址',
    token_symbol VARCHAR(20) NOT NULL COMMENT '代币符号',
    token_name VARCHAR(100) NOT NULL COMMENT '代币名称',
    token_decimals INT NOT NULL DEFAULT 18 COMMENT '代币精度',
    balance_raw VARCHAR(100) NOT NULL COMMENT '原始余额（字符串格式）',
    balance_decimal DECIMAL(30, 18) NOT NULL COMMENT '格式化余额',
    price_usd DECIMAL(30, 6) DEFAULT 0 COMMENT '代币价格（USD）',
    value_usd DECIMAL(30, 6) DEFAULT 0 COMMENT '资产价值（USD）',
    protocol_id VARCHAR(100) COMMENT '所属协议ID',
    asset_type VARCHAR(50) DEFAULT 'token' COMMENT '资产类型（token、lp_token、nft等）',
    source VARCHAR(20) DEFAULT 'service_a' COMMENT '数据来源',
    queried_at TIMESTAMP NOT NULL COMMENT '查询时间',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_user_chain (user_id, chain_id),
    KEY idx_user_protocol (user_id, protocol_id),
    KEY idx_queried_at (queried_at),
    KEY idx_token_address (token_address),
    CONSTRAINT fk_user_assets_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户资产表（服务A数据）';

-- 创建用户仓位表
CREATE TABLE IF NOT EXISTS user_positions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '仓位ID',
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    protocol_id VARCHAR(100) NOT NULL COMMENT '协议ID',
    position_id VARCHAR(200) NOT NULL COMMENT '仓位唯一标识',
    position_type VARCHAR(50) NOT NULL COMMENT '仓位类型（supply、borrow、stake、farm等）',
    token_address VARCHAR(42) NOT NULL COMMENT '代币地址',
    token_symbol VARCHAR(20) NOT NULL COMMENT '代币符号',
    amount_raw VARCHAR(100) NOT NULL COMMENT '原始数量（字符串格式）',
    amount_decimal DECIMAL(30, 18) NOT NULL COMMENT '格式化数量',
    price_usd DECIMAL(30, 6) DEFAULT 0 COMMENT '代币价格（USD）',
    value_usd DECIMAL(30, 6) DEFAULT 0 COMMENT '仓位价值（USD）',
    apy DECIMAL(10, 4) DEFAULT 0 COMMENT '年化收益率',
    health_factor DECIMAL(10, 4) DEFAULT 0 COMMENT '健康因子（借贷协议）',
    liquidation_threshold DECIMAL(10, 4) DEFAULT 0 COMMENT '清算阈值',
    collateral_factor DECIMAL(10, 4) DEFAULT 0 COMMENT '抵押因子',
    position_data JSON COMMENT '原始仓位数据',
    is_active BOOLEAN DEFAULT TRUE COMMENT '是否活跃',
    last_updated_by VARCHAR(50) DEFAULT 'service_b' COMMENT '最后更新来源',
    last_updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后更新时间',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_protocol_position (user_id, protocol_id, position_id),
    KEY idx_user_id (user_id),
    KEY idx_protocol_id (protocol_id),
    KEY idx_position_type (position_type),
    KEY idx_last_updated (last_updated_at DESC),
    KEY idx_is_active (is_active),
    KEY idx_value_usd (value_usd DESC),
    CONSTRAINT fk_user_positions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户仓位表（服务B数据）';

-- 创建协议代币表
CREATE TABLE IF NOT EXISTS protocol_tokens (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '代币ID',
    protocol_id VARCHAR(100) NOT NULL COMMENT '协议ID',
    chain_id INT NOT NULL COMMENT '链ID',
    token_address VARCHAR(42) NOT NULL COMMENT '代币合约地址',
    token_symbol VARCHAR(20) NOT NULL COMMENT '代币符号',
    token_name VARCHAR(100) NOT NULL COMMENT '代币名称',
    token_decimals INT NOT NULL DEFAULT 18 COMMENT '代币精度',
    is_collateral BOOLEAN DEFAULT FALSE COMMENT '是否可作为抵押品',
    is_borrowable BOOLEAN DEFAULT FALSE COMMENT '是否可借出',
    is_supply BOOLEAN DEFAULT FALSE COMMENT '是否可存入',
    supply_apy DECIMAL(10, 4) DEFAULT 0 COMMENT '存款APY',
    borrow_apy DECIMAL(10, 4) DEFAULT 0 COMMENT '借款APY',
    liquidation_threshold DECIMAL(10, 4) DEFAULT 0 COMMENT '清算阈值',
    collateral_factor DECIMAL(10, 4) DEFAULT 0 COMMENT '抵押因子',
    price_usd DECIMAL(30, 6) DEFAULT 0 COMMENT '代币价格（USD）',
    tvl_usd DECIMAL(30, 6) DEFAULT 0 COMMENT '代币TVL（USD）',
    metadata JSON COMMENT '扩展元数据',
    last_updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后更新时间',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_protocol_token (protocol_id, chain_id, token_address),
    KEY idx_protocol_id (protocol_id),
    KEY idx_token_address (token_address),
    KEY idx_chain_id (chain_id),
    KEY idx_is_collateral (is_collateral),
    KEY idx_is_borrowable (is_borrowable)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='协议代币表';

-- 创建同步记录表
CREATE TABLE IF NOT EXISTS sync_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '记录ID',
    sync_type VARCHAR(50) NOT NULL COMMENT '同步类型（protocol_metadata、user_positions等）',
    sync_source VARCHAR(50) NOT NULL COMMENT '同步来源（debank、service_b等）',
    target_id VARCHAR(100) COMMENT '目标ID（协议ID、用户地址等）',
    status VARCHAR(20) NOT NULL COMMENT '状态（pending、success、failed）',
    total_count INT DEFAULT 0 COMMENT '总记录数',
    success_count INT DEFAULT 0 COMMENT '成功数',
    failed_count INT DEFAULT 0 COMMENT '失败数',
    error_message TEXT COMMENT '错误信息',
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开始时间',
    finished_at TIMESTAMP NULL COMMENT '完成时间',
    duration_ms INT DEFAULT 0 COMMENT '耗时（毫秒）',
    metadata JSON COMMENT '扩展信息',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_sync_type (sync_type),
    KEY idx_sync_source (sync_source),
    KEY idx_status (status),
    KEY idx_started_at (started_at DESC),
    KEY idx_target_id (target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='同步记录表';

-- 创建缓存状态表
CREATE TABLE IF NOT EXISTS cache_status (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '记录ID',
    cache_key VARCHAR(500) NOT NULL COMMENT '缓存键',
    cache_type VARCHAR(50) NOT NULL COMMENT '缓存类型（position、protocol、token等）',
    entity_id VARCHAR(100) NOT NULL COMMENT '实体ID',
    ttl_seconds INT NOT NULL DEFAULT 600 COMMENT 'TTL（秒）',
    last_cached_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最后缓存时间',
    expires_at TIMESTAMP NOT NULL COMMENT '过期时间',
    hit_count INT DEFAULT 0 COMMENT '命中次数',
    miss_count INT DEFAULT 0 COMMENT '未命中次数',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_cache_key (cache_key(255)),
    KEY idx_cache_type (cache_type),
    KEY idx_entity_id (entity_id),
    KEY idx_expires_at (expires_at),
    KEY idx_last_cached (last_cached_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='缓存状态表';

-- 创建队列消息表
CREATE TABLE IF NOT EXISTS queue_messages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '消息ID',
    message_id VARCHAR(100) NOT NULL COMMENT '消息唯一ID',
    queue_name VARCHAR(100) NOT NULL COMMENT '队列名称',
    message_type VARCHAR(50) NOT NULL COMMENT '消息类型（position_update、price_update等）',
    payload JSON NOT NULL COMMENT '消息载荷',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' COMMENT '状态（pending、processing、completed、failed）',
    retry_count INT DEFAULT 0 COMMENT '重试次数',
    max_retries INT DEFAULT 3 COMMENT '最大重试次数',
    error_message TEXT COMMENT '错误信息',
    processed_at TIMESTAMP NULL COMMENT '处理时间',
    scheduled_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '计划处理时间',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_message_id (message_id),
    KEY idx_queue_name (queue_name),
    KEY idx_message_type (message_type),
    KEY idx_status (status),
    KEY idx_scheduled_at (scheduled_at),
    KEY idx_created_at (created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='队列消息表';

-- 创建API请求日志表
CREATE TABLE IF NOT EXISTS api_request_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '日志ID',
    request_id VARCHAR(100) NOT NULL COMMENT '请求唯一ID',
    api_path VARCHAR(500) NOT NULL COMMENT 'API路径',
    method VARCHAR(10) NOT NULL COMMENT 'HTTP方法',
    user_id BIGINT UNSIGNED COMMENT '用户ID',
    user_address VARCHAR(42) COMMENT '用户地址',
    ip_address VARCHAR(45) COMMENT 'IP地址',
    user_agent TEXT COMMENT 'User-Agent',
    request_params JSON COMMENT '请求参数',
    request_body JSON COMMENT '请求体',
    response_status INT NOT NULL COMMENT '响应状态码',
    response_time_ms INT NOT NULL COMMENT '响应时间（毫秒）',
    error_code VARCHAR(50) COMMENT '错误码',
    error_message TEXT COMMENT '错误信息',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_request_id (request_id),
    KEY idx_api_path (api_path(100)),
    KEY idx_user_id (user_id),
    KEY idx_user_address (user_address),
    KEY idx_response_status (response_status),
    KEY idx_created_at (created_at DESC),
    KEY idx_response_time (response_time_ms)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='API请求日志表';

-- 创建视图：用户资产汇总视图
CREATE OR REPLACE VIEW user_asset_summary AS
SELECT 
    u.id as user_id,
    u.address,
    u.chain_id,
    COUNT(DISTINCT up.protocol_id) as protocol_count,
    COUNT(DISTINCT up.id) as position_count,
    SUM(up.value_usd) as total_position_value,
    COALESCE(SUM(ua.value_usd), 0) as total_asset_value,
    (SUM(up.value_usd) + COALESCE(SUM(ua.value_usd), 0)) as total_value_usd,
    MAX(GREATEST(up.last_updated_at, ua.queried_at)) as last_updated_at
FROM users u
LEFT JOIN user_positions up ON u.id = up.user_id AND up.is_active = TRUE
LEFT JOIN user_assets ua ON u.id = ua.user_id
GROUP BY u.id, u.address, u.chain_id;

-- 创建视图：协议统计视图
CREATE OR REPLACE VIEW protocol_statistics AS
SELECT 
    p.id as protocol_id,
    p.name,
    p.category,
    p.tvl_usd,
    COUNT(DISTINCT up.user_id) as user_count,
    COUNT(DISTINCT up.id) as position_count,
    SUM(up.value_usd) as total_position_value,
    AVG(up.apy) as avg_apy,
    MAX(up.last_updated_at) as last_updated_at
FROM protocols p
LEFT JOIN user_positions up ON p.protocol_id = up.protocol_id AND up.is_active = TRUE
WHERE p.is_active = TRUE
GROUP BY p.id, p.name, p.category, p.tvl_usd;

-- 插入默认协议数据
INSERT IGNORE INTO protocols (protocol_id, name, description, category, logo_url, website_url, risk_level, is_active) VALUES
('aave', 'Aave', '开源非托管流动性协议', 'lending', 'https://app.aave.com/favicon.ico', 'https://aave.com', 2, TRUE),
('compound', 'Compound', '算法货币市场协议', 'lending', 'https://compound.finance/favicon.ico', 'https://compound.finance', 2, TRUE),
('uniswap', 'Uniswap', '去中心化交易协议', 'dex', 'https://app.uniswap.org/favicon.png', 'https://uniswap.org', 2, TRUE),
('curve', 'Curve Finance', '稳定币交易协议', 'dex', 'https://curve.fi/favicon.ico', 'https://curve.fi', 2, TRUE),
('makerdao', 'MakerDAO', '去中心化稳定币协议', 'lending', 'https://makerdao.com/favicon.ico', 'https://makerdao.com', 2, TRUE),
('yearn', 'Yearn Finance', '收益聚合协议', 'yield', 'https://yearn.finance/favicon.ico', 'https://yearn.finance', 3, TRUE),
('sushiswap', 'SushiSwap', '社区驱动的DEX', 'dex', 'https://app.sushi.com/favicon.ico', 'https://sushi.com', 3, TRUE),
('balancer', 'Balancer', '自动化投资组合管理协议', 'dex', 'https://balancer.fi/favicon.ico', 'https://balancer.fi', 2, TRUE),
('synthetix', 'Synthetix', '合成资产发行协议', 'derivative', 'https://synthetix.io/favicon.ico', 'https://synthetix.io', 4, TRUE),
('instadapp', 'Instadapp', 'DeFi智能账户平台', 'wallet', 'https://instadapp.io/favicon.ico', 'https://instadapp.io', 2, TRUE);

-- 创建数据同步服务用户（用于连接数据库）
CREATE USER IF NOT EXISTS 'defi_sync'@'%' IDENTIFIED BY 'sync_password';
GRANT SELECT, INSERT, UPDATE, DELETE ON defi_asset_service.* TO 'defi_sync'@'%';
FLUSH PRIVILEGES;

-- 创建API服务用户（只读权限）
CREATE USER IF NOT EXISTS 'defi_api'@'%' IDENTIFIED BY 'api_password';
GRANT SELECT ON defi_asset_service.* TO 'defi_api'@'%';
GRANT INSERT, UPDATE ON defi_asset_service.api_request_logs TO 'defi_api'@'%';
FLUSH PRIVILEGES;