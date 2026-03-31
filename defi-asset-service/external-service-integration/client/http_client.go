package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
)

// HTTPClientConfig HTTP客户端配置
type HTTPClientConfig struct {
	BaseURL              string        `json:"base_url" yaml:"base_url"`
	Timeout              time.Duration `json:"timeout" yaml:"timeout"`
	MaxIdleConns         int           `json:"max_idle_conns" yaml:"max_idle_conns"`
	MaxIdleConnsPerHost  int           `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	MaxConnsPerHost      int           `json:"max_conns_per_host" yaml:"max_conns_per_host"`
	IdleConnTimeout      time.Duration `json:"idle_conn_timeout" yaml:"idle_conn_timeout"`
	TLSHandshakeTimeout  time.Duration `json:"tls_handshake_timeout" yaml:"tls_handshake_timeout"`
	ExpectContinueTimeout time.Duration `json:"expect_continue_timeout" yaml:"expect_continue_timeout"`
	DisableKeepAlives    bool          `json:"disable_keep_alives" yaml:"disable_keep_alives"`
	DisableCompression   bool          `json:"disable_compression" yaml:"disable_compression"`
	InsecureSkipVerify   bool          `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
}

// DefaultHTTPClientConfig 默认HTTP客户端配置
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:              30 * time.Second,
		MaxIdleConns:         100,
		MaxIdleConnsPerHost:  10,
		MaxConnsPerHost:      50,
		IdleConnTimeout:      90 * time.Second,
		TLSHandshakeTimeout:  10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:    false,
		DisableCompression:   false,
		InsecureSkipVerify:   false,
	}
}

// HTTPClient HTTP客户端封装
type HTTPClient struct {
	client  *http.Client
	baseURL *url.URL
	config  HTTPClientConfig
}

// NewHTTPClient 创建新的HTTP客户端
func NewHTTPClient(baseURL string, config HTTPClientConfig) (*HTTPClient, error) {
	// 解析基础URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	// 创建Transport
	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
		TLSHandshakeTimeout: config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		DisableKeepAlives:   config.DisableKeepAlives,
		DisableCompression:  config.DisableCompression,
	}

	// 配置TLS
	if config.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// 创建HTTP客户端
	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	return &HTTPClient{
		client:  client,
		baseURL: parsedURL,
		config:  config,
	}, nil
}

// RequestOptions 请求选项
type RequestOptions struct {
	Method      string
	Path        string
	QueryParams map[string]string
	Headers     map[string]string
	Body        interface{}
	Context     context.Context
}

// Response 响应封装
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// Do 执行HTTP请求
func (c *HTTPClient) Do(opts RequestOptions) (*Response, error) {
	// 创建请求URL
	requestURL, err := c.buildURL(opts.Path, opts.QueryParams)
	if err != nil {
		return nil, fmt.Errorf("failed to build request URL: %w", err)
	}

	// 创建请求体
	var bodyReader io.Reader
	if opts.Body != nil {
		bodyBytes, err := json.Marshal(opts.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(
		getContext(opts.Context),
		opts.Method,
		requestURL.String(),
		bodyReader,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// 设置请求头
	c.setHeaders(req, opts.Headers)
	if opts.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 执行请求
	log.Debug().
		Str("method", opts.Method).
		Str("url", requestURL.String()).
		Msg("sending HTTP request")

	startTime := time.Now()
	resp, err := c.client.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		log.Error().
			Err(err).
			Str("method", opts.Method).
			Str("url", requestURL.String()).
			Dur("elapsed", elapsed).
			Msg("HTTP request failed")
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().
			Err(err).
			Str("method", opts.Method).
			Str("url", requestURL.String()).
			Int("status", resp.StatusCode).
			Dur("elapsed", elapsed).
			Msg("failed to read response body")
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debug().
		Str("method", opts.Method).
		Str("url", requestURL.String()).
		Int("status", resp.StatusCode).
		Dur("elapsed", elapsed).
		Int("body_size", len(body)).
		Msg("HTTP request completed")

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}, nil
}

// Get 执行GET请求
func (c *HTTPClient) Get(path string, queryParams map[string]string, headers map[string]string) (*Response, error) {
	return c.Do(RequestOptions{
		Method:      http.MethodGet,
		Path:        path,
		QueryParams: queryParams,
		Headers:     headers,
	})
}

// Post 执行POST请求
func (c *HTTPClient) Post(path string, body interface{}, headers map[string]string) (*Response, error) {
	return c.Do(RequestOptions{
		Method:  http.MethodPost,
		Path:    path,
		Body:    body,
		Headers: headers,
	})
}

// Put 执行PUT请求
func (c *HTTPClient) Put(path string, body interface{}, headers map[string]string) (*Response, error) {
	return c.Do(RequestOptions{
		Method:  http.MethodPut,
		Path:    path,
		Body:    body,
		Headers: headers,
	})
}

// Delete 执行DELETE请求
func (c *HTTPClient) Delete(path string, headers map[string]string) (*Response, error) {
	return c.Do(RequestOptions{
		Method:  http.MethodDelete,
		Path:    path,
		Headers: headers,
	})
}

// buildURL 构建完整的请求URL
func (c *HTTPClient) buildURL(path string, queryParams map[string]string) (*url.URL, error) {
	// 解析路径
	pathURL, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}

	// 合并基础URL和路径
	fullURL := c.baseURL.ResolveReference(pathURL)

	// 添加查询参数
	if len(queryParams) > 0 {
		query := fullURL.Query()
		for key, value := range queryParams {
			query.Set(key, value)
		}
		fullURL.RawQuery = query.Encode()
	}

	return fullURL, nil
}

// setHeaders 设置请求头
func (c *HTTPClient) setHeaders(req *http.Request, headers map[string]string) {
	// 设置默认请求头
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "DeFi-Asset-Service/1.0")

	// 设置自定义请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// getContext 获取上下文
func getContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// ParseJSONResponse 解析JSON响应
func ParseJSONResponse[T any](resp *Response) (*T, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(resp.Body))
	}

	var result T
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return &result, nil
}

// HealthCheck 健康检查
func (c *HTTPClient) HealthCheck() error {
	_, err := c.Get("/health", nil, nil)
	return err
}

// Close 关闭HTTP客户端
func (c *HTTPClient) Close() {
	// 如果Transport实现了CloseIdleConnections方法，则调用它
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}