package sync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/config"
	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/models"
	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/service"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	cron        *cron.Cron
	db          *gorm.DB
	config      *config.Config
	logger      *slog.Logger
	syncService *service.SyncService
	entries     map[string]cron.EntryID
}

// NewScheduler 创建新的调度器
func NewScheduler(db *gorm.DB, cfg *config.Config, logger *slog.Logger, syncService *service.SyncService) *Scheduler {
	// 使用秒级精度的cron解析器
	c := cron.New(cron.WithSeconds(), cron.WithChain(
		cron.Recover(cron.DefaultLogger),
	))

	return &Scheduler{
		cron:        c,
		db:          db,
		config:      cfg,
		logger:      logger,
		syncService: syncService,
		entries:     make(map[string]cron.EntryID),
	}
}

// Start 启动调度器
func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Info("启动定时任务调度器")

	// 注册定时任务
	if err := s.registerJobs(); err != nil {
		return fmt.Errorf("注册定时任务失败: %w", err)
	}

	// 启动cron调度器
	s.cron.Start()

	// 监听上下文取消
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	s.logger.Info("定时任务调度器启动完成")
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.logger.Info("停止定时任务调度器")
	
	// 停止cron调度器
	if s.cron != nil {
		ctx := s.cron.Stop()
		select {
		case <-ctx.Done():
			s.logger.Info("定时任务调度器已停止")
		case <-time.After(10 * time.Second):
			s.logger.Warn("定时任务调度器停止超时")
		}
	}
}

// registerJobs 注册定时任务
func (s *Scheduler) registerJobs() error {
	// 协议元数据同步任务
	if s.config.Sync.ProtocolMetadata.Enabled {
		schedule := s.config.Sync.ProtocolMetadata.Schedule
		if schedule == "" {
			schedule = "0 0 2 * * *" // 每天凌晨2点
		}
		
		entryID, err := s.cron.AddFunc(schedule, s.protocolMetadataJob)
		if err != nil {
			return fmt.Errorf("添加协议元数据同步任务失败: %w", err)
		}
		s.entries["protocol_metadata"] = entryID
		s.logger.Info("注册协议元数据同步任务", "schedule", schedule)
	}

	// 协议代币同步任务
	if s.config.Sync.ProtocolTokens.Enabled {
		schedule := s.config.Sync.ProtocolTokens.Schedule
		if schedule == "" {
			schedule = "0 0 3 * * *" // 每天凌晨3点
		}
		
		entryID, err := s.cron.AddFunc(schedule, s.protocolTokensJob)
		if err != nil {
			return fmt.Errorf("添加协议代币同步任务失败: %w", err)
		}
		s.entries["protocol_tokens"] = entryID
		s.logger.Info("注册协议代币同步任务", "schedule", schedule)
	}

	// 用户仓位同步任务
	if s.config.Sync.UserPositions.Enabled {
		schedule := s.config.Sync.UserPositions.Schedule
		if schedule == "" {
			schedule = "0 */10 * * * *" // 每10分钟
		}
		
		entryID, err := s.cron.AddFunc(schedule, s.userPositionsJob)
		if err != nil {
			return fmt.Errorf("添加用户仓位同步任务失败: %w", err)
		}
		s.entries["user_positions"] = entryID
		s.logger.Info("注册用户仓位同步任务", "schedule", schedule)
	}

	// 增量同步检查任务
	if s.config.Sync.Incremental.Enabled {
		schedule := fmt.Sprintf("0 */%d * * * *", s.parseDurationMinutes(s.config.Sync.Incremental.CheckInterval))
		
		entryID, err := s.cron.AddFunc(schedule, s.incrementalSyncJob)
		if err != nil {
			return fmt.Errorf("添加增量同步任务失败: %w", err)
		}
		s.entries["incremental_sync"] = entryID
		s.logger.Info("注册增量同步任务", "schedule", schedule)
	}

	// 健康检查任务
	healthCheckSchedule := "0 */5 * * * *" // 每5分钟
	entryID, err := s.cron.AddFunc(healthCheckSchedule, s.healthCheckJob)
	if err != nil {
		return fmt.Errorf("添加健康检查任务失败: %w", err)
	}
	s.entries["health_check"] = entryID
	s.logger.Info("注册健康检查任务", "schedule", healthCheckSchedule)

	return nil
}

// protocolMetadataJob 协议元数据同步任务
func (s *Scheduler) protocolMetadataJob() {
	s.logger.Info("开始执行协议元数据同步任务")
	
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Sync.ProtocolMetadata.Timeout)
	defer cancel()

	// 创建同步记录
	syncRecord := &models.SyncRecord{
		SyncType:   models.SyncTypeProtocolMetadata,
		SyncSource: "debank",
		Status:     models.SyncStatusRunning,
		StartedAt:  time.Now(),
	}

	// 保存同步记录
	if err := s.db.Create(syncRecord).Error; err != nil {
		s.logger.Error("创建同步记录失败", "error", err)
		return
	}

	// 执行同步
	stats, err := s.syncService.SyncProtocolMetadata(ctx, true) // 全量同步
	if err != nil {
		s.logger.Error("协议元数据同步失败", "error", err)
		syncRecord.Status = models.SyncStatusFailed
		syncRecord.ErrorMessage = err.Error()
	} else {
		s.logger.Info("协议元数据同步完成", 
			"total", stats.Total,
			"created", stats.Created,
			"updated", stats.Updated,
			"failed", stats.Failed)
		syncRecord.Status = models.SyncStatusSuccess
		syncRecord.TotalCount = stats.Total
		syncRecord.SuccessCount = stats.Created + stats.Updated
		syncRecord.FailedCount = stats.Failed
	}

	// 更新同步记录
	syncRecord.FinishedAt = time.Now()
	syncRecord.DurationMs = int(syncRecord.FinishedAt.Sub(syncRecord.StartedAt).Milliseconds())
	if err := s.db.Save(syncRecord).Error; err != nil {
		s.logger.Error("更新同步记录失败", "error", err)
	}
}

// protocolTokensJob 协议代币同步任务
func (s *Scheduler) protocolTokensJob() {
	s.logger.Info("开始执行协议代币同步任务")
	
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Sync.ProtocolTokens.Timeout)
	defer cancel()

	// 创建同步记录
	syncRecord := &models.SyncRecord{
		SyncType:   models.SyncTypeProtocolTokens,
		SyncSource: "debank",
		Status:     models.SyncStatusRunning,
		StartedAt:  time.Now(),
	}

	// 保存同步记录
	if err := s.db.Create(syncRecord).Error; err != nil {
		s.logger.Error("创建同步记录失败", "error", err)
		return
	}

	// 执行同步
	stats, err := s.syncService.SyncProtocolTokens(ctx)
	if err != nil {
		s.logger.Error("协议代币同步失败", "error", err)
		syncRecord.Status = models.SyncStatusFailed
		syncRecord.ErrorMessage = err.Error()
	} else {
		s.logger.Info("协议代币同步完成", 
			"total", stats.Total,
			"created", stats.Created,
			"updated", stats.Updated,
			"failed", stats.Failed)
		syncRecord.Status = models.SyncStatusSuccess
		syncRecord.TotalCount = stats.Total
		syncRecord.SuccessCount = stats.Created + stats.Updated
		syncRecord.FailedCount = stats.Failed
	}

	// 更新同步记录
	syncRecord.FinishedAt = time.Now()
	syncRecord.DurationMs = int(syncRecord.FinishedAt.Sub(syncRecord.StartedAt).Milliseconds())
	if err := s.db.Save(syncRecord).Error; err != nil {
		s.logger.Error("更新同步记录失败", "error", err)
	}
}

// userPositionsJob 用户仓位同步任务
func (s *Scheduler) userPositionsJob() {
	s.logger.Info("开始执行用户仓位同步任务")
	
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Sync.UserPositions.Timeout)
	defer cancel()

	// 创建同步记录
	syncRecord := &models.SyncRecord{
		SyncType:   models.SyncTypeUserPositions,
		SyncSource: "service_b",
		Status:     models.SyncStatusRunning,
		StartedAt:  time.Now(),
	}

	// 保存同步记录
	if err := s.db.Create(syncRecord).Error; err != nil {
		s.logger.Error("创建同步记录失败", "error", err)
		return
	}

	// 执行同步（这里需要实现具体的用户仓位同步逻辑）
	// 暂时记录未实现
	s.logger.Warn("用户仓位同步逻辑未实现")
	syncRecord.Status = models.SyncStatusSuccess
	syncRecord.TotalCount = 0
	syncRecord.SuccessCount = 0
	syncRecord.FailedCount = 0

	// 更新同步记录
	syncRecord.FinishedAt = time.Now()
	syncRecord.DurationMs = int(syncRecord.FinishedAt.Sub(syncRecord.StartedAt).Milliseconds())
	if err := s.db.Save(syncRecord).Error; err != nil {
		s.logger.Error("更新同步记录失败", "error", err)
	}
}

// incrementalSyncJob 增量同步任务
func (s *Scheduler) incrementalSyncJob() {
	s.logger.Info("开始执行增量同步检查任务")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 检查需要增量同步的协议
	protocols, err := s.findProtocolsNeedingSync(ctx)
	if err != nil {
		s.logger.Error("查找需要同步的协议失败", "error", err)
		return
	}

	if len(protocols) == 0 {
		s.logger.Debug("没有需要增量同步的协议")
		return
	}

	s.logger.Info("发现需要增量同步的协议", "count", len(protocols))

	// 对每个协议执行增量同步
	for _, protocol := range protocols {
		s.incrementalSyncProtocol(ctx, protocol)
	}
}

// incrementalSyncProtocol 增量同步单个协议
func (s *Scheduler) incrementalSyncProtocol(ctx context.Context, protocol *models.Protocol) {
	s.logger.Info("开始增量同步协议", "protocol_id", protocol.ProtocolID)
	
	// 创建同步记录
	syncRecord := &models.SyncRecord{
		SyncType:   models.SyncTypeIncremental,
		SyncSource: "debank",
		TargetID:   protocol.ProtocolID,
		Status:     models.SyncStatusRunning,
		StartedAt:  time.Now(),
	}

	// 保存同步记录
	if err := s.db.Create(syncRecord).Error; err != nil {
		s.logger.Error("创建增量同步记录失败", "error", err)
		return
	}

	// 执行增量同步
	stats, err := s.syncService.IncrementalSyncProtocol(ctx, protocol.ProtocolID)
	if err != nil {
		s.logger.Error("协议增量同步失败", "protocol_id", protocol.ProtocolID, "error", err)
		syncRecord.Status = models.SyncStatusFailed
		syncRecord.ErrorMessage = err.Error()
	} else {
		s.logger.Info("协议增量同步完成", 
			"protocol_id", protocol.ProtocolID,
			"created", stats.Created,
			"updated", stats.Updated,
			"failed", stats.Failed)
		syncRecord.Status = models.SyncStatusSuccess
		syncRecord.TotalCount = stats.Total
		syncRecord.SuccessCount = stats.Created + stats.Updated
		syncRecord.FailedCount = stats.Failed
	}

	// 更新同步记录
	syncRecord.FinishedAt = time.Now()
	syncRecord.DurationMs = int(syncRecord.FinishedAt.Sub(syncRecord.StartedAt).Milliseconds())
	if err := s.db.Save(syncRecord).Error; err != nil {
		s.logger.Error("更新增量同步记录失败", "error", err)
	}
}

// healthCheckJob 健康检查任务
func (s *Scheduler) healthCheckJob() {
	s.logger.Debug("执行健康检查任务")
	
	// 检查数据库连接
	if err := s.db.Exec("SELECT 1").Error; err != nil {
		s.logger.Error("数据库健康检查失败", "error", err)
		return
	}

	// 检查最近同步任务状态
	var failedSyncs int64
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	
	if err := s.db.Model(&models.SyncRecord{}).
		Where("status = ? AND started_at > ?", models.SyncStatusFailed, oneHourAgo).
		Count(&failedSyncs).Error; err != nil {
		s.logger.Error("检查同步任务状态失败", "error", err)
		return
	}

	if failedSyncs > 0 {
		s.logger.Warn("发现失败的同步任务", "count", failedSyncs)
	}

	s.logger.Debug("健康检查完成")
}

// findProtocolsNeedingSync 查找需要同步的协议
func (s *Scheduler) findProtocolsNeedingSync(ctx context.Context) ([]*models.Protocol, error) {
	var protocols []*models.Protocol
	
	// 查找最后同步时间超过24小时的活跃协议
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	
	err := s.db.Where("is_active = ? AND (last_synced_at IS NULL OR last_synced_at < ?)", true, oneDayAgo).
		Order("last_synced_at ASC NULLS FIRST").
		Limit(10). // 每次最多同步10个协议
		Find(&protocols).Error
	
	if err != nil {
		return nil, fmt.Errorf("查询需要同步的协议失败: %w", err)
	}

	return protocols, nil
}

// parseDurationMinutes 解析持续时间字符串为分钟数
func (s *Scheduler) parseDurationMinutes(durationStr string) int {
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		s.logger.Warn("解析持续时间失败，使用默认值5分钟", "duration", durationStr, "error", err)
		return 5
	}
	return int(duration.Minutes())
}

// TriggerManualSync 手动触发同步
func (s *Scheduler) TriggerManualSync(syncType string, targetID string) error {
	s.logger.Info("手动触发同步", "sync_type", syncType, "target_id", targetID)

	switch syncType {
	case models.SyncTypeProtocolMetadata:
		go s.protocolMetadataJob()
	case models.SyncTypeProtocolTokens:
		go s.protocolTokensJob()
	case models.SyncTypeUserPositions:
		go s.userPositionsJob()
	case models.SyncTypeIncremental:
		if targetID == "" {
			return fmt.Errorf("增量同步需要指定target_id")
		}
		// 查找协议并触发增量同步
		var protocol models.Protocol
		if err := s.db.Where("protocol_id = ?", targetID).First(&protocol).Error; err != nil {
			return fmt.Errorf("查找协议失败: %w", err)
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			s.incrementalSyncProtocol(ctx, &protocol)
		}()
	default:
		return fmt.Errorf("不支持的同步类型: %s", syncType)
	}

	return nil
}

// GetScheduledJobs 获取已调度的任务
func (s *Scheduler) GetScheduledJobs() map[string]cron.Entry {
	jobs := make(map[string]cron.Entry)
	for name, entryID := range s.entries {
		entry := s.cron.Entry(entryID)
		jobs[name] = entry
	}
	return jobs
}