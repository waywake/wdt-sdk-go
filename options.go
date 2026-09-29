package wdt

import (
	"net/http"
	"time"
)

// Option 配置 Client 的可选参数。
type Option func(*Client)

// WithBaseURL 覆盖 API 地址。
//
// 平台默认地址：
//   - 正式环境：http://wdt.wangdian.cn/openapi
//   - 测试环境：http://47.92.239.46/openapi
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

// WithHTTPClient 使用自定义 *http.Client（可控制代理、连接池、TLS 等）。
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout 设置单次请求超时（含连接与读取），默认 30s。
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithMaxRetries 设置限频/网络错误的自动重试次数，默认 2，设为 0 关闭重试。
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

// WithRetryDelay 设置重试的基础等待时长，默认 500ms，按指数退避（500ms、1s、2s…）。
func WithRetryDelay(d time.Duration) Option {
	return func(c *Client) { c.retryDelay = d }
}
