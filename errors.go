package wdt

import (
	"errors"
	"fmt"
	"strings"
)

// APIError 表示平台返回的业务错误（响应中 status != 0）。
type APIError struct {
	// Method 是本次调用的接口名称，例如 sales.TradeQuery.queryWithDetail。
	Method string
	// Status 是平台返回的状态码，0 表示成功，非 0 表示失败。
	Status int
	// Message 是平台返回的错误信息。
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("wdt: %s 失败 status=%d: %s", e.Method, e.Status, e.Message)
}

// IsRateLimited 判断 err 是否为平台限频/限并发错误。
// 此类错误（status=100）由客户端自动重试，业务方一般无需关心。
func IsRateLimited(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return isRateLimitMessage(apiErr.Message)
	}
	return false
}

// isRateLimitMessage 匹配平台限频错误文案。
func isRateLimitMessage(msg string) bool {
	return strings.Contains(msg, "超过每分钟最大调用频率限制") ||
		strings.Contains(msg, "超过每分钟最大并发次数限制")
}
