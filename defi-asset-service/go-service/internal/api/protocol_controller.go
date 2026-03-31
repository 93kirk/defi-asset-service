package api

import (
	"net/http"
	"strconv"

	"defi-asset-service/internal/model"
	"defi-asset-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ProtocolController 协议控制器
type ProtocolController struct {
	protocolSvc service.ProtocolService
	logger      *logrus.Logger
}

// NewProtocolController 创建协议控制器
func NewProtocolController(
	protocolSvc service.ProtocolService,
	logger *logrus.Logger,
) *ProtocolController {
	return &ProtocolController{
		protocolSvc: protocolSvc,
		logger:      logger,
	}
}

// RegisterRoutes 注册路由
func (c *ProtocolController) RegisterRoutes(router *gin.RouterGroup) {
	protocolGroup := router.Group("/protocols")
	{
		protocolGroup.GET("", c.GetProtocols)
		protocolGroup.GET("/:protocol_id", c.GetProtocol)
		protocolGroup.GET("/:protocol_id/tokens", c.GetProtocolTokens)
	}
	
	adminGroup := router.Group("/admin")
	{
		adminGroup.POST("/sync/protocols", c.SyncProtocols)
		adminGroup.GET("/sync/:sync_id", c.GetSyncStatus)
	}
}

// GetProtocols 获取协议列表
// @Summary 获取协议列表
// @Description 获取所有支持的协议列表
// @Tags 协议
// @Accept json
// @Produce json
// @Param category query string false "协议类别过滤"
// @Param chain_id query int false "链ID过滤"
// @Param is_active query bool false "是否只返回活跃协议" default(true)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} model.APIResponse{data=model.ProtocolListResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /protocols [get]
func (c *ProtocolController) GetProtocols(ctx *gin.Context) {
	// 解析查询参数
	query := model.ProtocolQuery{
		Category: ctx.Query("category"),
		IsActive: true,
		Page:     1,
		PageSize: 20,
	}
	
	// 解析链ID
	if chainIDStr := ctx.Query("chain_id"); chainIDStr != "" {
		if chainID, err := strconv.Atoi(chainIDStr); err == nil {
			query.ChainID = chainID
		}
	}
	
	// 解析是否活跃
	if isActiveStr := ctx.Query("is_active"); isActiveStr != "" {
		if isActive, err := strconv.ParseBool(isActiveStr); err == nil {
			query.IsActive = isActive
		}
	}
	
	// 解析页码
	if pageStr := ctx.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			query.Page = page
		}
	}
	
	// 解析每页数量
	if pageSizeStr := ctx.Query("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 && pageSize <= 100 {
			query.PageSize = pageSize
		}
	}
	
	// 获取协议列表
	response, err := c.protocolSvc.GetProtocols(ctx.Request.Context(), query)
	if err != nil {
		c.logger.WithError(err).Error("Failed to get protocols")
		ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeInternalError))
		return
	}
	
	ctx.JSON(http.StatusOK, model.NewAPIResponse(model.CodeSuccess, response))
}

// GetProtocol 获取协议详情
// @Summary 获取协议详情
// @Description 获取指定协议的详细信息
// @Tags 协议
// @Accept json
// @Produce json
// @Param protocol_id path string true "协议ID"
// @Success 200 {object} model.APIResponse{data=model.ProtocolResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /protocols/{protocol_id} [get]
func (c *ProtocolController) GetProtocol(ctx *gin.Context) {
	protocolID := ctx.Param("protocol_id")
	if protocolID == "" {
		ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeInvalidParameter))
		return
	}
	
	// 获取协议详情
	protocol, err := c.protocolSvc.GetProtocol(ctx.Request.Context(), protocolID)
	if err != nil {
		c.logger.WithError(err).Errorf("Failed to get protocol %s", protocolID)
		
		// 检查是否是协议不存在
		if err.Error() == "protocol not found" {
			ctx.JSON(http.StatusNotFound, model.NewErrorResponse(model.CodeProtocolNotFound))
			return
		}
		
		ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeInternalError))
		return
	}
	
	ctx.JSON(http.StatusOK, model.NewAPIResponse(model.CodeSuccess, protocol))
}

// GetProtocolTokens 获取协议代币列表
// @Summary 获取协议代币列表
// @Description 获取协议支持的代币列表
// @Tags 协议
// @Accept json
// @Produce json
// @Param protocol_id path string true "协议ID"
// @Param chain_id query int false "链ID过滤"
// @Param is_collateral query bool false "是否可作为抵押品"
// @Param is_borrowable query bool false "是否可借出"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /protocols/{protocol_id}/tokens [get]
func (c *ProtocolController) GetProtocolTokens(ctx *gin.Context) {
	protocolID := ctx.Param("protocol_id")
	if protocolID == "" {
		ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeInvalidParameter))
		return
	}
	
	// 解析查询参数
	query := model.TokenQuery{}
	
	// 解析链ID
	if chainIDStr := ctx.Query("chain_id"); chainIDStr != "" {
		if chainID, err := strconv.Atoi(chainIDStr); err == nil {
			query.ChainID = chainID
		}
	}
	
	// 解析是否可作为抵押品
	if isCollateralStr := ctx.Query("is_collateral"); isCollateralStr != "" {
		if isCollateral, err := strconv.ParseBool(isCollateralStr); err == nil {
			query.IsCollateral = isCollateral
		}
	}
	
	// 解析是否可借出
	if isBorrowableStr := ctx.Query("is_borrowable"); isBorrowableStr != "" {
		if isBorrowable, err := strconv.ParseBool(isBorrowableStr); err == nil {
			query.IsBorrowable = isBorrowable
		}
	}
	
	// 获取协议代币
	tokens, err := c.protocolSvc.GetProtocolTokens(ctx.Request.Context(), protocolID, query)
	if err != nil {
		c.logger.WithError(err).Errorf("Failed to get tokens for protocol %s", protocolID)
		
		// 检查是否是协议不存在
		if err.Error() == "protocol not found" {
			ctx.JSON(http.StatusNotFound, model.NewErrorResponse(model.CodeProtocolNotFound))
			return
		}
		
		ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeInternalError))
		return
	}
	
	// 构建响应
	response := map[string]interface{}{
		"protocol_id": protocolID,
		"tokens":      tokens,
		"total":       len(tokens),
	}
	
	ctx.JSON(http.StatusOK, model.NewAPIResponse(model.CodeSuccess, response))
}

// SyncProtocols 触发协议元数据同步
// @Summary 触发协议元数据同步
// @Description 手动触发协议元数据同步
// @Tags 管理
// @Accept json
// @Produce json
// @Param request body model.SyncProtocolRequest true "同步请求"
// @Success 200 {object} model.APIResponse{data=model.SyncResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /admin/sync/protocols [post]
func (c *ProtocolController) SyncProtocols(ctx *gin.Context) {
	var request model.SyncProtocolRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		c.logger.WithError(err).Error("Invalid sync request")
		ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeInvalidParameter))
		return
	}
	
	// 检查是否已有同步在进行中
	// 这里可以添加检查逻辑，防止重复同步
	
	// 触发同步
	response, err := c.protocolSvc.SyncProtocols(ctx.Request.Context(), request.ForceFullSync, request.ProtocolIDs)
	if err != nil {
		c.logger.WithError(err).Error("Failed to sync protocols")
		ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeSyncFailed))
		return
	}
	
	ctx.JSON(http.StatusOK, model.NewAPIResponse(model.CodeSuccess, response))
}

// GetSyncStatus 获取同步状态
// @Summary 获取同步状态
// @Description 获取同步任务状态
// @Tags 管理
// @Accept json
// @Produce json
// @Param sync_id path string true "同步任务ID"
// @Success 200 {object} model.APIResponse{data=model.SyncStatusResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /admin/sync/{sync_id} [get]
func (c *ProtocolController) GetSyncStatus(ctx *gin.Context) {
	syncID := ctx.Param("sync_id")
	if syncID == "" {
		ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeInvalidParameter))
		return
	}
	
	// 获取同步状态
	status, err := c.protocolSvc.GetSyncStatus(ctx.Request.Context(), syncID)
	if err != nil {
		c.logger.WithError(err).Errorf("Failed to get sync status %s", syncID)
		
		// 检查是否是同步记录不存在
		if err.Error() == "sync record not found" {
			ctx.JSON(http.StatusNotFound, model.NewErrorResponse(model.CodeSyncFailed))
			return
		}
		
		ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeInternalError))
		return
	}
	
	ctx.JSON(http.StatusOK, model.NewAPIResponse(model.CodeSuccess, status))
}