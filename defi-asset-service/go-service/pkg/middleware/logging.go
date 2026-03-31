package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"defi-asset-service/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// LoggingMiddleware 日志中间件
type LoggingMiddleware struct {
	logger *logrus.Logger
}

// NewLoggingMiddleware 创建日志中间件
func NewLoggingMiddleware(logger *logrus.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

// RequestLogging 请求日志中间件
func (m *LoggingMiddleware) RequestLogging() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 生成请求ID
		requestID := uuid.New().String()
		ctx.Set("request_id", requestID)
		
		// 设置请求ID到Header
		ctx.Header("X-Request-ID", requestID)
		
		// 记录请求开始时间
		startTime := time.Now()
		
		// 记录请求信息
		requestInfo := m.getRequestInfo(ctx)
		
		// 读取请求体（用于日志记录）
		var requestBody []byte
		if ctx.Request.Body != nil {
			requestBody, _ = io.ReadAll(ctx.Request.Body)
			ctx.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}
		
		// 创建自定义的ResponseWriter来捕获响应
		blw := &bodyLogWriter{
			ResponseWriter: ctx.Writer,
			body:           bytes.NewBufferString(""),
		}
		ctx.Writer = blw
		
		// 处理请求
		ctx.Next()
		
		// 计算处理时间
		duration := time.Since(startTime)
		
		// 记录响应信息
		responseInfo := m.getResponseInfo(ctx, blw, duration)
		
		// 记录完整的请求日志
		logEntry := m.logger.WithFields(logrus.Fields{
			"request_id":      requestID,
			"method":          requestInfo.Method,
			"path":            requestInfo.Path,
			"query":           requestInfo.Query,
			"client_ip":       requestInfo.ClientIP,
			"user_agent":      requestInfo.UserAgent,
			"user_id":         requestInfo.UserID,
			"user_address":    requestInfo.UserAddress,
			"request_body":    string(requestBody),
			"response_status": responseInfo.Status,
			"response_time":   responseInfo.DurationMs,
			"response_size":   responseInfo.Size,
			"error_code":      responseInfo.ErrorCode,
			"error_message":   responseInfo.ErrorMessage,
		})
		
		// 根据状态码选择日志级别
		if responseInfo.Status >= 500 {
			logEntry.Error("Request completed with server error")
		} else if responseInfo.Status >= 400 {
			logEntry.Warn("Request completed with client error")
		} else {
			logEntry.Info("Request completed successfully")
		}
		
		// 异步存储到数据库（如果配置了）
		go m.storeRequestLog(ctx, requestInfo, responseInfo, requestBody, blw.body.Bytes())
	}
}

// getRequestInfo 获取请求信息
func (m *LoggingMiddleware) getRequestInfo(ctx *gin.Context) *RequestInfo {
	info := &RequestInfo{
		RequestID:    ctx.GetString("request_id"),
		Method:       ctx.Request.Method,
		Path:         ctx.Request.URL.Path,
		Query:        ctx.Request.URL.RawQuery,
		ClientIP:     ctx.ClientIP(),
		UserAgent:    ctx.Request.UserAgent(),
		Timestamp:    time.Now(),
	}
	
	// 获取用户信息（如果已认证）
	if userID, exists := ctx.Get("user_id"); exists {
		if id, ok := userID.(uint64); ok {
			info.UserID = id
		}
	}
	
	if userAddress, exists := ctx.Get("user_address"); exists {
		if address, ok := userAddress.(string); ok {
			info.UserAddress = address
		}
	}
	
	return info
}

// getResponseInfo 获取响应信息
func (m *LoggingMiddleware) getResponseInfo(ctx *gin.Context, blw *bodyLogWriter, duration time.Duration) *ResponseInfo {
	info := &ResponseInfo{
		Status:      ctx.Writer.Status(),
		DurationMs:  duration.Milliseconds(),
		Size:        ctx.Writer.Size(),
		Timestamp:   time.Now(),
	}
	
	// 尝试从响应体中提取错误信息
	if info.Status >= 400 {
		var errorResp model.ErrorResponse
		if err := json.Unmarshal(blw.body.Bytes(), &errorResp); err == nil {
			info.ErrorCode = errorResp.Code
			info.ErrorMessage = errorResp.Message
		}
	}
	
	return info
}

// storeRequestLog 存储请求日志到数据库
func (m *LoggingMiddleware) storeRequestLog(ctx *gin.Context, requestInfo *RequestInfo, responseInfo *ResponseInfo, requestBody, responseBody []byte) {
	// 这里应该实现将日志存储到数据库的逻辑
	// 例如存储到MySQL的api_request_logs表
	
	// 注意：在实际项目中，应该使用异步队列来处理日志存储
	// 避免阻塞请求处理
	
	// 示例代码：
	/*
	logEntry := model.APIRequestLog{
		RequestID:      requestInfo.RequestID,
		APIPath:        requestInfo.Path,
		Method:         requestInfo.Method,
		UserID:         requestInfo.UserID,
		UserAddress:    requestInfo.UserAddress,
		IPAddress:      requestInfo.ClientIP,
		UserAgent:      requestInfo.UserAgent,
		RequestParams:  parseQueryParams(requestInfo.Query),
		RequestBody:    parseRequestBody(requestBody),
		ResponseStatus: responseInfo.Status,
		ResponseTimeMs: int(responseInfo.DurationMs),
		ErrorCode:      responseInfo.ErrorCode,
		ErrorMessage:   responseInfo.ErrorMessage,
		CreatedAt:      time.Now(),
	}
	
	// 存储到数据库
	if err := db.Create(&logEntry).Error; err != nil {
		m.logger.WithError(err).Error("Failed to store request log")
	}
	*/
}

// parseQueryParams 解析查询参数
func parseQueryParams(query string) map[string]interface{} {
	// 这里应该实现查询参数的解析逻辑
	return nil
}

// parseRequestBody 解析请求体
func parseRequestBody(body []byte) map[string]interface{} {
	if len(body) == 0 {
		return nil
	}
	
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}
	
	return data
}

// RequestInfo 请求信息
type RequestInfo struct {
	RequestID    string    `json:"request_id"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Query        string    `json:"query"`
	ClientIP     string    `json:"client_ip"`
	UserAgent    string    `json:"user_agent"`
	UserID       uint64    `json:"user_id,omitempty"`
	UserAddress  string    `json:"user_address,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// ResponseInfo 响应信息
type ResponseInfo struct {
	Status       int       `json:"status"`
	DurationMs   int64     `json:"duration_ms"`
	Size         int       `json:"size"`
	ErrorCode    int       `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// bodyLogWriter 自定义ResponseWriter用于捕获响应体
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write 重写Write方法
func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// WriteString 重写WriteString方法
func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}