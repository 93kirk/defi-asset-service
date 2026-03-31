package api

import (
	"net/http"
	"strconv"
	"strings"

	"defi-asset-service/internal/model"
	"defi-asset-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// UserController 用户控制器
type UserController struct {
	serviceA service.ServiceAService
	serviceB service.ServiceBService
	logger   *logrus.Logger
}

// NewUserController 创建用户控制器
func NewUserController(
	serviceA service.ServiceAService,
	serviceB service.ServiceBService,
	logger *logrus.Logger,
) *UserController {
	return &UserController{
		serviceA: serviceA,
		serviceB: serviceB,
		logger:   logger,
	}
}

// RegisterRoutes 注册路由
func (c *UserController) RegisterRoutes(router *gin.RouterGroup) {
	userGroup := router.Group("/users")
	{
		userGroup.GET("/:address/summary", c.GetUserSummary)
		userGroup.GET("/:address/assets", c.GetUserAssets)
		userGroup.GET("/:address/positions", c.GetUserPositions)
		userGroup.POST("/batch/assets", c.BatchGetUserAssets)
	}
}

// GetUserSummary 获取用户资产总览
// @Summary 获取用户资产总览
// @Description 获取用户在DeFi协议中的总资产情况
// @Tags 用户
// @Accept json
// @Produce json
// @Param address path string true "用户钱包地址"
// @Param chain_id query int false "链ID" default(1)
// @Param include_assets query bool false "是否包含资产详情" default(false)
// @Param include_positions query bool false "是否包含仓位详情" default(false)
// @Success 200 {object} model.APIResponse{data=model.UserSummaryResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users/{address}/summary [get]
func (c *UserController) GetUserSummary(ctx *gin.Context) {
	address := ctx.Param("address")
	if !isValidAddress(address) {
		ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeInvalidAddress))
		return
	}
	
	// 解析查询参数
	chainID, _ := strconv.Atoi(ctx.DefaultQuery("chain_id", "1"))
	includeAssets, _ := strconv.ParseBool(ctx.DefaultQuery("include_assets", "false"))
	includePositions, _ := strconv.ParseBool(ctx.DefaultQuery("include_positions", "false"))
	
	// 获取资产数据
	var assets []model.AssetResponse
	if includeAssets {
		assetsResp, err := c.serviceA.GetUserAssets(ctx.Request.Context(), address, chainID, "", "")
		if err != nil {
			c.logger.WithError(err).Error("Failed to get user assets")
			ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeServiceAError))
			return
		}
		assets = assetsResp
	}
	
	// 获取仓位数据
	var positions []model.PositionResponse
	if includePositions {
		positionsResp, err := c.serviceB.GetUserPositions(ctx.Request.Context(), address, chainID, "", "", false)
		if err != nil {
			c.logger.WithError(err).Error("Failed to get user positions")
			ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeServiceBError))
			return
		}
		positions = positionsResp
	}
	
	// 计算汇总信息
	totalAssetValue := calculateTotalAssetValue(assets)
	totalPositionValue := calculateTotalPositionValue(positions)
	totalValue := totalAssetValue + totalPositionValue
	
	// 构建响应
	response := model.UserSummaryResponse{
		User: model.UserSummary{
			Address:              address,
			ChainID:              chainID,
			TotalValueUSD:        totalValue,
			TotalAssetValueUSD:   totalAssetValue,
			TotalPositionValueUSD: totalPositionValue,
			ProtocolCount:        countUniqueProtocols(positions),
			PositionCount:        len(positions),
			LastUpdatedAt:        getCurrentTimestamp(),
		},
	}
	
	if includeAssets {
		response.Assets = assets
	}
	
	if includePositions {
		response.Positions = positions
	}
	
	ctx.JSON(http.StatusOK, model.NewAPIResponse(model.CodeSuccess, response))
}

// GetUserAssets 获取用户实时资产
// @Summary 获取用户实时资产
// @Description 获取用户有balance概念的协议资产（实时查询）
// @Tags 用户
// @Accept json
// @Produce json
// @Param address path string true "用户钱包地址"
// @Param chain_id query int false "链ID" default(1)
// @Param protocol_id query string false "协议ID过滤"
// @Param token_address query string false "代币地址过滤"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users/{address}/assets [get]
func (c *UserController) GetUserAssets(ctx *gin.Context) {
	address := ctx.Param("address")
	if !isValidAddress(address) {
		ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeInvalidAddress))
		return
	}
	
	// 解析查询参数
	chainID, _ := strconv.Atoi(ctx.DefaultQuery("chain_id", "1"))
	protocolID := ctx.Query("protocol_id")
	tokenAddress := ctx.Query("token_address")
	
	// 获取资产数据
	assets, err := c.serviceA.GetUserAssets(ctx.Request.Context(), address, chainID, protocolID, tokenAddress)
	if err != nil {
		c.logger.WithError(err).Error("Failed to get user assets")
		ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeServiceAError))
		return
	}
	
	// 构建响应
	response := map[string]interface{}{
		"address":        address,
		"chain_id":       chainID,
		"total_value_usd": calculateTotalAssetValue(assets),
		"assets":         assets,
		"queried_at":     getCurrentTimestamp(),
	}
	
	ctx.JSON(http.StatusOK, model.NewAPIResponse(model.CodeSuccess, response))
}

// GetUserPositions 获取用户协议仓位
// @Summary 获取用户协议仓位
// @Description 获取用户无balance概念的协议仓位数据（带缓存）
// @Tags 用户
// @Accept json
// @Produce json
// @Param address path string true "用户钱包地址"
// @Param chain_id query int false "链ID" default(1)
// @Param protocol_id query string false "协议ID过滤"
// @Param position_type query string false "仓位类型过滤"
// @Param refresh query bool false "强制刷新缓存" default(false)
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users/{address}/positions [get]
func (c *UserController) GetUserPositions(ctx *gin.Context) {
	address := ctx.Param("address")
	if !isValidAddress(address) {
		ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeInvalidAddress))
		return
	}
	
	// 解析查询参数
	chainID, _ := strconv.Atoi(ctx.DefaultQuery("chain_id", "1"))
	protocolID := ctx.Query("protocol_id")
	positionType := ctx.Query("position_type")
	refresh, _ := strconv.ParseBool(ctx.DefaultQuery("refresh", "false"))
	
	// 获取仓位数据
	positions, err := c.serviceB.GetUserPositions(ctx.Request.Context(), address, chainID, protocolID, positionType, refresh)
	if err != nil {
		c.logger.WithError(err).Error("Failed to get user positions")
		ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeServiceBError))
		return
	}
	
	// 检查是否从缓存获取
	cached := !refresh
	cacheExpiresAt := ""
	if cached {
		// 计算缓存过期时间（假设缓存10分钟）
		cacheExpiresAt = getFutureTimestamp(10 * 60)
	}
	
	// 构建响应
	response := map[string]interface{}{
		"address":          address,
		"chain_id":         chainID,
		"total_value_usd":  calculateTotalPositionValue(positions),
		"positions":        positions,
		"cached":           cached,
		"cache_expires_at": cacheExpiresAt,
		"last_updated_at":  getCurrentTimestamp(),
	}
	
	ctx.JSON(http.StatusOK, model.NewAPIResponse(model.CodeSuccess, response))
}

// BatchGetUserAssets 批量查询用户资产
// @Summary 批量查询用户资产
// @Description 批量查询多个用户的资产情况
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body model.BatchAssetRequest true "批量查询请求"
// @Success 200 {object} model.APIResponse{data=model.BatchAssetResponse}
// @Failure 400 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /users/batch/assets [post]
func (c *UserController) BatchGetUserAssets(ctx *gin.Context) {
	var request model.BatchAssetRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		c.logger.WithError(err).Error("Invalid batch request")
		ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeInvalidParameter))
		return
	}
	
	// 验证地址数量
	if len(request.Addresses) > 50 {
		ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeBatchLimitExceeded))
		return
	}
	
	// 验证地址格式
	for _, address := range request.Addresses {
		if !isValidAddress(address) {
			ctx.JSON(http.StatusBadRequest, model.NewErrorResponse(model.CodeInvalidAddress))
			return
		}
	}
	
	// 批量获取资产数据
	var results []model.UserSummaryResponse
	
	// 获取资产数据（如果请求包含）
	var assetsMap map[string][]model.AssetResponse
	if request.IncludeAssets {
		assets, err := c.serviceA.BatchGetUserAssets(ctx.Request.Context(), request.Addresses, request.ChainID)
		if err != nil {
			c.logger.WithError(err).Error("Failed to batch get user assets")
			ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeServiceAError))
			return
		}
		assetsMap = assets
	}
	
	// 获取仓位数据（如果请求包含）
	var positionsMap map[string][]model.PositionResponse
	if request.IncludePositions {
		positions, err := c.serviceB.BatchGetUserPositions(ctx.Request.Context(), request.Addresses, request.ChainID)
		if err != nil {
			c.logger.WithError(err).Error("Failed to batch get user positions")
			ctx.JSON(http.StatusInternalServerError, model.NewErrorResponse(model.CodeServiceBError))
			return
		}
		positionsMap = positions
	}
	
	// 构建结果
	for _, address := range request.Addresses {
		var assets []model.AssetResponse
		var positions []model.PositionResponse
		
		if request.IncludeAssets {
			assets = assetsMap[address]
		}
		
		if request.IncludePositions {
			positions = positionsMap[address]
		}
		
		totalAssetValue := calculateTotalAssetValue(assets)
		totalPositionValue := calculateTotalPositionValue(positions)
		totalValue := totalAssetValue + totalPositionValue
		
		result := model.UserSummaryResponse{
			User: model.UserSummary{
				Address:              address,
				ChainID:              request.ChainID,
				TotalValueUSD:        totalValue,
				TotalAssetValueUSD:   totalAssetValue,
				TotalPositionValueUSD: totalPositionValue,
				ProtocolCount:        countUniqueProtocols(positions),
				PositionCount:        len(positions),
				LastUpdatedAt:        getCurrentTimestamp(),
			},
		}
		
		if request.IncludeAssets {
			result.Assets = assets
		}
		
		if request.IncludePositions {
			result.Positions = positions
		}
		
		results = append(results, result)
	}
	
	// 构建响应
	response := model.BatchAssetResponse{
		Results:   results,
		QueriedAt: getCurrentTimestamp(),
	}
	
	ctx.JSON(http.StatusOK, model.NewAPIResponse(model.CodeSuccess, response))
}

// 辅助函数

// isValidAddress 验证地址格式
func isValidAddress(address string) bool {
	if len(address) != 42 {
		return false
	}
	
	if !strings.HasPrefix(address, "0x") {
		return false
	}
	
	// 简单的格式验证，实际项目中应该更严格
	for _, ch := range address[2:] {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	
	return true
}

// calculateTotalAssetValue 计算总资产价值
func calculateTotalAssetValue(assets []model.AssetResponse) float64 {
	var total float64
	for _, asset := range assets {
		total += asset.ValueUSD
	}
	return total
}

// calculateTotalPositionValue 计算总仓位价值
func calculateTotalPositionValue(positions []model.PositionResponse) float64 {
	var total float64
	for _, position := range positions {
		total += position.ValueUSD
	}
	return total
}

// countUniqueProtocols 计算唯一协议数量
func countUniqueProtocols(positions []model.PositionResponse) int {
	protocols := make(map[string]bool)
	for _, position := range positions {
		protocols[position.ProtocolID] = true
	}
	return len(protocols)
}

// getCurrentTimestamp 获取当前时间戳字符串
func getCurrentTimestamp() string {
	return time.Now().Format(time.RFC3339)
}

// getFutureTimestamp 获取未来时间戳字符串
func getFutureTimestamp(seconds int) string {
	return time.Now().Add(time.Duration(seconds) * time.Second).Format(time.RFC3339)
}

// 导入必要的包
import (
	"time"
)