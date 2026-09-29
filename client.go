package wdt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 平台默认 API 地址与协议常量。
const (
	// DefaultBaseURL 是正式环境地址；测试环境为 http://47.92.239.46/openapi。
	DefaultBaseURL = "http://wdt.wangdian.cn/openapi"

	// apiVersion 是公共参数 v 的当前版本。
	apiVersion = "1.0"

	// timestampEpoch 为 2012-01-01 00:00:00（北京时间）。
	// 公共参数 timestamp = 当前秒级时间戳 - timestampEpoch，
	// 与服务器时间相差 120s 内视为合法。
	timestampEpoch = 1325347200
)

const (
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 2
	defaultRetryDelay = 500 * time.Millisecond
)

// Client 是旺店通旗舰版开放平台的 API 客户端。并发安全。
//
// 各业务接口通过内嵌的 Services 分组暴露，例如
// c.TradeQuery.QueryWithDetail、c.Goods.Push、c.WMS.StockSpec 等。
type Client struct {
	sid    string
	key    string
	secret string
	salt   string

	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	maxRetries int
	retryDelay time.Duration

	// now 供测试注入时钟。
	now func() time.Time

	// 服务分组，字段见 services.go。
	Services
}

// NewClient 创建客户端。
//
// appsecret 由平台按 "secret:salt" 格式分配，冒号前为 secret 用于签名，
// 冒号后为 salt 作为公共请求参数。
func NewClient(sid, appkey, appsecret string, opts ...Option) (*Client, error) {
	secret, salt, ok := strings.Cut(appsecret, ":")
	if !ok || secret == "" || salt == "" {
		return nil, fmt.Errorf("wdt: appsecret 格式不正确，应为 secret:salt 形式（如 testsecret:testsalt）")
	}
	if sid == "" || appkey == "" {
		return nil, fmt.Errorf("wdt: sid 与 appkey 不能为空")
	}

	c := &Client{
		sid:        sid,
		key:        appkey,
		secret:     secret,
		salt:       salt,
		baseURL:    DefaultBaseURL,
		timeout:    defaultTimeout,
		maxRetries: defaultMaxRetries,
		retryDelay: defaultRetryDelay,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(c)
	}
	if _, err := url.Parse(c.baseURL); err != nil {
		return nil, fmt.Errorf("wdt: baseURL 不合法: %w", err)
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: c.timeout}
	}
	newServices(c)
	return c, nil
}

// Call 调用任意接口（通用入口）。
//
// method 为接口服务名（如 sales.TradeQuery.queryWithDetail），
// args 为业务参数，按平台约定以 JSON 数组形式作为请求体发送：
//
//	// 查询类接口：单个参数对象
//	client.Call(ctx, "sales.TradeQuery.queryWithDetail", map[string]any{
//		"start_time": "2024-01-01 00:00:00",
//		"end_time":   "2024-01-01 01:00:00",
//	})
//
//	// 多参数接口：按文档顺序依次传入
//	client.Call(ctx, "sales.RawTrade.pushSelf", shopNo, rawTradeList)
//
// 尾部为 nil 的参数会被自动去掉，与文档示例保持一致。
func (c *Client) Call(ctx context.Context, method string, args ...any) (*Response, error) {
	if args == nil {
		args = []any{}
	}
	return c.do(ctx, method, nil, args)
}

// PageCall 调用分页查询接口，在 URL 上附加 page_size/page_no/calc_total 公共分页参数。
func (c *Client) PageCall(ctx context.Context, method string, pager *Pager, args ...any) (*Response, error) {
	return c.do(ctx, method, pager, args)
}

func (c *Client) do(ctx context.Context, method string, pager *Pager, args []any) (*Response, error) {
	if method == "" {
		return nil, fmt.Errorf("wdt: method 不能为空")
	}
	if args == nil {
		args = []any{}
	}
	body, err := jsonEncode(trimTrailingNils(args))
	if err != nil {
		return nil, fmt.Errorf("wdt: 编码请求参数失败: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepContext(ctx, c.retryDelay<<(attempt-1)); err != nil {
				return nil, err
			}
		}
		resp, err := c.attempt(ctx, method, pager, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retryable(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) attempt(ctx context.Context, method string, pager *Pager, body []byte) (*Response, error) {
	params := map[string]string{
		"sid":       c.sid,
		"key":       c.key,
		"salt":      c.salt,
		"method":    method,
		"timestamp": strconv.FormatInt(c.now().Unix()-timestampEpoch, 10),
		"v":         apiVersion,
	}
	if pager != nil {
		params["page_size"] = strconv.Itoa(pager.PageSize)
		params["page_no"] = strconv.Itoa(pager.PageNo)
		if pager.CalcTotal {
			params["calc_total"] = "1"
		} else {
			params["calc_total"] = "0"
		}
	}
	// JSON 报文以 body 参数参与签名，但不放入 URL。
	params["body"] = string(body)
	params["sign"] = sign(params, c.secret)
	delete(params, "body")

	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"?"+query.Encode(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("wdt: 构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &transportError{method: method, err: err}
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &transportError{method: method, err: err}
	}
	if httpResp.StatusCode >= 400 {
		return nil, &transportError{
			method: method,
			err:    fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, snippet(raw)),
			status: httpResp.StatusCode,
		}
	}

	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("wdt: %s 响应不是有效 JSON: %s", method, snippet(raw))
	}
	if resp.Status != 0 {
		return nil, &APIError{Method: method, Status: resp.Status, Message: resp.Message}
	}
	return &resp, nil
}

// transportError 表示网络层或 HTTP 层错误（可重试）。
type transportError struct {
	method string
	err    error
	status int
}

func (e *transportError) Error() string {
	return fmt.Sprintf("wdt: %s 请求失败: %v", e.method, e.err)
}

func (e *transportError) Unwrap() error { return e.err }

func retryable(err error) bool {
	if IsRateLimited(err) {
		return true
	}
	var te *transportError
	if errors.As(err, &te) {
		return te.status == 0 || te.status >= 500
	}
	return false
}

// trimTrailingNils 去掉尾部 nil 参数，与文档示例中省略可选参数的形态一致。
func trimTrailingNils(args []any) []any {
	i := len(args)
	for i > 0 && args[i-1] == nil {
		i--
	}
	return args[:i]
}

// jsonEncode 与平台 PHP 端一致：非 ASCII 字符不转义，HTML 字符不转义。
func jsonEncode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func snippet(b []byte) string {
	s := string(b)
	if len(s) > 256 {
		s = s[:256] + "..."
	}
	return s
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
