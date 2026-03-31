package state

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/models"
)

// StateManager 同步状态管理器
type StateManager struct {
	db     *gorm.DB
	logger *slog.Logger
	mu     sync.RWMutex
	states map[string]*SyncState
}

// SyncState 同步状态
type SyncState struct {
	ProtocolID      string    `json:"protocol_id"`
	SyncType        string    `json:"sync_type"`
	LastSyncTime    time.Time `json:"last_sync_time"`
	LastSuccessTime time.Time `json:"last_success_time"`
	LastError       string    `json:"last_error"`
	ErrorCount      int       `json:"error_count"`
	SuccessCount    int       `json:"success_count"`
	TotalDuration   time.Duration `json:"total_duration"`
	AvgDuration     time.Duration `json:"avg_duration"`
	IsSyncing       bool      `json:"is_syncing"`
	LastChangeTime  time.Time `json:"last_change_time"`
}

// NewStateManager 创建新的状态管理器
func NewStateManager(db *gorm.DB, logger *slog.Logger) *StateManager {
	return &StateManager{
		db:     db,
		logger: logger,
		states: make(map[string]*SyncState),
	}
}

// Initialize 初始化状态管理器
func (sm *StateManager) Initialize(ctx context.Context) error {
	sm.logger.Info("初始化同步状态管理器")

	// 从数据库加载历史同步记录
	if err := sm.loadHistoricalStates(ctx); err != nil {
		return fmt.Errorf("加载历史状态失败: %w", err)
	}

	sm.logger.Info("同步状态管理器初始化完成", "state_count", len(sm.states))
	return nil
}

// StartSync 开始同步
func (sm *StateManager) StartSync(protocolID, syncType string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := sm.getStateKey(protocolID, syncType)
	state, exists := sm.states[key]
	
	if !exists {
		state = &SyncState{
			ProtocolID: protocolID,
			SyncType:   syncType,
		}
		sm.states[key] = state
	}

	state.IsSyncing = true
	state.LastSyncTime = time.Now()
	state.LastChangeTime = time.Now()

	sm.logger.Debug("开始同步", 
		"protocol_id", protocolID, 
		"sync_type", syncType)
	return nil
}

// CompleteSync 完成同步（成功）
func (sm *StateManager) CompleteSync(protocolID, syncType string, duration time.Duration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := sm.getStateKey(protocolID, syncType)
	state, exists := sm.states[key]
	
	if !exists {
		return fmt.Errorf("未找到同步状态: %s", key)
	}

	state.IsSyncing = false
	state.LastSuccessTime = time.Now()
	state.LastError = ""
	state.SuccessCount++
	state.TotalDuration += duration
	
	// 计算平均持续时间
	if state.SuccessCount > 0 {
		state.AvgDuration = state.TotalDuration / time.Duration(state.SuccessCount)
	}
	
	state.LastChangeTime = time.Now()

	// 保存到数据库
	if err := sm.saveStateToDB(context.Background(), state); err != nil {
		sm.logger.Warn("保存同步状态到数据库失败", "error", err)
	}

	sm.logger.Debug("同步完成", 
		"protocol_id", protocolID, 
		"sync_type", syncType,
		"duration", duration,
		"success_count", state.SuccessCount)
	return nil
}

// FailSync 同步失败
func (sm *StateManager) FailSync(protocolID, syncType string, err error, duration time.Duration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := sm.getStateKey(protocolID, syncType)
	state, exists := sm.states[key]
	
	if !exists {
		state = &SyncState{
			ProtocolID: protocolID,
			SyncType:   syncType,
		}
		sm.states[key] = state
	}

	state.IsSyncing = false
	state.LastError = err.Error()
	state.ErrorCount++
	state.LastChangeTime = time.Now()

	// 保存到数据库
	if err := sm.saveStateToDB(context.Background(), state); err != nil {
		sm.logger.Warn("保存失败状态到数据库失败", "error", err)
	}

	sm.logger.Warn("同步失败", 
		"protocol_id", protocolID, 
		"sync_type", syncType,
		"error", err,
		"error_count", state.ErrorCount)
	return nil
}

// GetState 获取同步状态
func (sm *StateManager) GetState(protocolID, syncType string) (*SyncState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key := sm.getStateKey(protocolID, syncType)
	state, exists := sm.states[key]
	return state, exists
}

// GetAllStates 获取所有同步状态
func (sm *StateManager) GetAllStates() map[string]*SyncState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 返回副本
	states := make(map[string]*SyncState)
	for key, state := range sm.states {
		states[key] = sm.copyState(state)
	}
	return states
}

// GetProtocolStates 获取协议的所有同步状态
func (sm *StateManager) GetProtocolStates(protocolID string) map[string]*SyncState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	states := make(map[string]*SyncState)
	for key, state := range sm.states {
		if state.ProtocolID == protocolID {
			states[state.SyncType] = sm.copyState(state)
		}
	}
	return states
}

// IsSyncing 检查是否正在同步
func (sm *StateManager) IsSyncing(protocolID, syncType string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key := sm.getStateKey(protocolID, syncType)
	state, exists := sm.states[key]
	return exists && state.IsSyncing
}

// ShouldSync 检查是否应该同步
func (sm *StateManager) ShouldSync(protocolID, syncType string, minInterval time.Duration) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key := sm.getStateKey(protocolID, syncType)
	state, exists := sm.states[key]
	
	if !exists {
		return true // 从未同步过，应该同步
	}

	if state.IsSyncing {
		return false // 正在同步中
	}

	// 检查上次成功同步时间
	if time.Since(state.LastSuccessTime) >= minInterval {
		return true
	}

	// 如果有错误，根据错误计数决定是否重试
	if state.LastError != "" {
		// 指数退避重试
		backoffTime := sm.calculateBackoffTime(state.ErrorCount)
		if time.Since(state.LastChangeTime) >= backoffTime {
			return true
		}
	}

	return false
}

// GetSyncHealth 获取同步健康状态
func (sm *StateManager) GetSyncHealth(protocolID string) *SyncHealth {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	states := sm.GetProtocolStates(protocolID)
	if len(states) == 0 {
		return &SyncHealth{
			ProtocolID: protocolID,
			Status:     HealthStatusUnknown,
			Message:    "无同步记录",
		}
	}

	health := &SyncHealth{
		ProtocolID: protocolID,
		Status:     HealthStatusHealthy,
		States:     states,
	}

	// 检查各个同步类型的健康状态
	for syncType, state := range states {
		health.TotalSyncs++
		
		if state.IsSyncing {
			health.ActiveSyncs++
			if time.Since(state.LastSyncTime) > 30*time.Minute {
				// 同步超过30分钟，可能有问题
				health.Status = HealthStatusWarning
				health.Message = fmt.Sprintf("%s同步已运行超过30分钟", syncType)
			}
		}

		if state.LastError != "" {
			health.FailedSyncs++
			
			// 最近有错误
			if time.Since(state.LastChangeTime) < 1*time.Hour {
				if state.ErrorCount > 3 {
					health.Status = HealthStatusCritical
					health.Message = fmt.Sprintf("%s同步连续失败%d次", syncType, state.ErrorCount)
				} else if health.Status != HealthStatusCritical {
					health.Status = HealthStatusWarning
					health.Message = fmt.Sprintf("%s同步最近失败", syncType)
				}
			}
		} else {
			health.SuccessfulSyncs++
		}

		// 检查同步频率
		if state.LastSuccessTime.IsZero() {
			// 从未成功同步
			health.Status = HealthStatusCritical
			health.Message = fmt.Sprintf("%s从未成功同步", syncType)
		} else if time.Since(state.LastSuccessTime) > 24*time.Hour {
			// 超过24小时未同步
			if health.Status != HealthStatusCritical {
				health.Status = HealthStatusWarning
				health.Message = fmt.Sprintf("%s超过24小时未同步", syncType)
			}
		}
	}

	return health
}

// GetSystemHealth 获取系统健康状态
func (sm *StateManager) GetSystemHealth() *SystemHealth {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	health := &SystemHealth{
		Status:       HealthStatusHealthy,
		TotalSyncs:   0,
		ActiveSyncs:  0,
		FailedSyncs:  0,
		Protocols:    make(map[string]*ProtocolHealth),
	}

	// 按协议分组统计
	protocolStats := make(map[string]*ProtocolHealth)
	
	for _, state := range sm.states {
		// 更新协议统计
		protoHealth, exists := protocolStats[state.ProtocolID]
		if !exists {
			protoHealth = &ProtocolHealth{
				ProtocolID: state.ProtocolID,
				Status:     HealthStatusHealthy,
			}
			protocolStats[state.ProtocolID] = protoHealth
		}

		protoHealth.TotalSyncs++
		health.TotalSyncs++
		
		if state.IsSyncing {
			protoHealth.ActiveSyncs++
			health.ActiveSyncs++
		}

		if state.LastError != "" && time.Since(state.LastChangeTime) < 1*time.Hour {
			protoHealth.FailedSyncs++
			health.FailedSyncs++
			
			if state.ErrorCount > 3 {
				protoHealth.Status = HealthStatusCritical
			} else if protoHealth.Status != HealthStatusCritical {
				protoHealth.Status = HealthStatusWarning
			}
		} else {
			protoHealth.SuccessfulSyncs++
		}

		// 更新协议状态
		if protoHealth.Status == HealthStatusCritical {
			health.Status = HealthStatusCritical
			health.Message = fmt.Sprintf("协议%s同步严重异常", state.ProtocolID)
		} else if protoHealth.Status == HealthStatusWarning && health.Status == HealthStatusHealthy {
			health.Status = HealthStatusWarning
			health.Message = fmt.Sprintf("协议%s同步警告", state.ProtocolID)
		}
	}

	health.Protocols = protocolStats
	
	// 计算成功率
	if health.TotalSyncs > 0 {
		health.SuccessRate = float64(health.TotalSyncs-health.FailedSyncs) / float64(health.TotalSyncs) * 100
	}

	return health
}

// ResetState 重置同步状态
func (sm *StateManager) ResetState(protocolID, syncType string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := sm.getStateKey(protocolID, syncType)
	delete(sm.states, key)

	// 从数据库删除
	if err := sm.deleteStateFromDB(context.Background(), protocolID, syncType); err != nil {
		sm.logger.Warn("从数据库删除状态失败", "error", err)
	}

	sm.logger.Info("重置同步状态", 
		"protocol_id", protocolID, 
		"sync_type", syncType)
	return nil
}

// CleanupOldStates 清理旧的状态记录
func (sm *StateManager) CleanupOldStates(maxAge time.Duration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoffTime := time.Now().Add(-maxAge)
	deletedCount := 0

	for key, state := range sm.states {
		if state.LastChangeTime.Before(cutoffTime) && !state.IsSyncing {
			delete(sm.states, key)
			deletedCount++
		}
	}

	sm.logger.Info("清理旧的状态记录", 
		"max_age", maxAge, 
		"deleted_count", deletedCount,
		"remaining_count", len(sm.states))
	return nil
}

// loadHistoricalStates 从数据库加载历史状态
func (sm *StateManager) loadHistoricalStates(ctx context.Context) error {
	// 这里可以从数据库加载历史同步记录
	// 暂时返回nil，实际实现时需要从数据库查询
	return nil
}

// saveStateToDB 保存状态到数据库
func (sm *StateManager) saveStateToDB(ctx context.Context, state *SyncState) error {
	// 这里可以将状态保存到数据库
	// 暂时返回nil，实际实现时需要保存到数据库
	return nil
}

// deleteStateFromDB 从数据库删除状态
func (sm *StateManager) deleteStateFromDB(ctx context.Context, protocolID, syncType string) error {
	// 这里可以从数据库删除状态
	// 暂时返回nil，实际实现时需要从数据库删除
	return nil
}

// getStateKey 获取状态键
func (sm *StateManager) getStateKey(protocolID, syncType string) string {
	return fmt.Sprintf("%s:%s", protocolID, syncType)
}

// copyState 复制状态
func (sm *StateManager) copyState(state *SyncState) *SyncState {
	return &SyncState{
		ProtocolID:      state.ProtocolID,
		SyncType:        state.SyncType,
		LastSyncTime:    state.LastSyncTime,
		LastSuccessTime: state.LastSuccessTime,
		LastError:       state.LastError,
		ErrorCount:      state.ErrorCount,
		SuccessCount:    state.SuccessCount,
		TotalDuration:   state.TotalDuration,
		AvgDuration:     state.AvgDuration,
		IsSyncing:       state.IsSyncing,
		LastChangeTime:  state.LastChangeTime,
	}
}

// calculateBackoffTime 计算退避时间
func (sm *StateManager) calculateBackoffTime(errorCount int) time.Duration {
	// 指数退避：1分钟, 2分钟, 4分钟, 8分钟, 最大30分钟
	backoff := time.Duration(1<<uint(min(errorCount, 5))) * time.Minute
	if backoff > 30*time.Minute {
		return 30 * time.Minute
	}
	return backoff
}

// min 返回最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SyncHealth 同步健康状态
type SyncHealth struct {
	ProtocolID      string                  `json:"protocol_id"`
	Status          HealthStatus            `json:"status"`
	Message         string                  `json:"message"`
	TotalSyncs      int                     `json:"total_syncs"`
	ActiveSyncs     int                     `json:"active_syncs"`
	FailedSyncs     int                     `json:"failed_syncs"`
	SuccessfulSyncs int                     `json:"successful_syncs"`
	States          map[string]*SyncState   `json:"states"`
}

// SystemHealth 系统健康状态
type SystemHealth struct {
	Status       HealthStatus                  `json:"status"`
	Message      string                        `json:"message"`
	TotalSyncs   int                           `json:"total_syncs"`
	ActiveSyncs  int                           `json:"active_syncs"`
	FailedSyncs  int                           `json:"failed_syncs"`
	SuccessRate  float64                       `json:"success_rate"`
	Protocols    map[string]*ProtocolHealth    `json:"protocols"`
}

// ProtocolHealth 协议健康状态
type ProtocolHealth struct {
	ProtocolID      string       `json:"protocol_id"`
	Status          HealthStatus `json:"status"`
	TotalSyncs      int          `json:"total_syncs"`
	ActiveSyncs     int          `json:"active_syncs"`
	FailedSyncs     int          `json:"failed_syncs"`
	SuccessfulSyncs int          `json:"successful_syncs"`
}

// HealthStatus 健康状态枚举
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusWarning   HealthStatus = "warning"
	HealthStatusCritical  HealthStatus = "critical"
	HealthStatusUnknown   HealthStatus = "unknown"
)