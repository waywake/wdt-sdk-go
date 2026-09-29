package wdt

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

const testAppsecret = "testsecret:testsalt"

// newTestClient 创建指向 httptest 服务器的客户端，时钟固定以便断言 timestamp。
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := NewClient("wdtapi3", "wdt_test", testAppsecret, WithBaseURL(srv.URL), WithRetryDelay(time.Millisecond))
	if err != nil {
		srv.Close()
		t.Fatalf("NewClient: %v", err)
	}
	c.now = func() time.Time { return time.Unix(1325347200+450444516, 0) }
	t.Cleanup(srv.Close)
	return c, srv
}

// verifySign 服务端按平台算法重算签名并比较。
func verifySign(t *testing.T, r *http.Request, body []byte, secret string) {
	t.Helper()
	q := r.URL.Query()
	params := map[string]string{}
	for k, v := range q {
		if k == "sign" {
			continue
		}
		params[k] = v[0]
	}
	if string(body) != "" {
		params["body"] = string(body)
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	plain := secret + strings.Join(mapKeysValues(params, keys), "") + secret
	sum := md5.Sum([]byte(plain))
	if want := hex.EncodeToString(sum[:]); q.Get("sign") != want {
		t.Errorf("签名不匹配: got %s want %s (plain=%s)", q.Get("sign"), want, plain)
	}
}

func mapKeysValues(params map[string]string, keys []string) []string {
	out := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		out = append(out, k, params[k])
	}
	return out
}

func TestClient_Call_BuildsSignedRequest(t *testing.T) {
	var gotBody []byte
	var gotQuery map[string]string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = b
		gotQuery = map[string]string{}
		for k, v := range r.URL.Query() {
			gotQuery[k] = v[0]
		}
		verifySign(t, r, b, "testsecret")
		w.Write([]byte(`{"status":0,"data":{"now":450444516}}`))
	})

	resp, err := c.Call(context.Background(), "system.Core.now")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got := gotQuery["method"]; got != "system.Core.now" {
		t.Errorf("method = %q", got)
	}
	if got := gotQuery["sid"]; got != "wdtapi3" {
		t.Errorf("sid = %q", got)
	}
	if got := gotQuery["key"]; got != "wdt_test" {
		t.Errorf("key = %q", got)
	}
	if got := gotQuery["salt"]; got != "testsalt" {
		t.Errorf("salt = %q, 应为 appsecret 冒号后的部分", got)
	}
	if got := gotQuery["v"]; got != "1.0" {
		t.Errorf("v = %q", got)
	}
	if got := gotQuery["timestamp"]; got != "450444516" {
		t.Errorf("timestamp = %q, 应为当前秒级时间戳减去 1325347200", got)
	}
	if string(gotBody) != "[]" {
		t.Errorf("无参调用 body = %q, want []", gotBody)
	}
	var data struct {
		Now int64 `json:"now"`
	}
	if err := resp.Decode(&data); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if data.Now != 450444516 {
		t.Errorf("data.now = %d", data.Now)
	}
}

func TestClient_Call_ParamsObjectAndPager(t *testing.T) {
	var gotQuery map[string]string
	var gotBody []byte
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = b
		gotQuery = map[string]string{}
		for k, v := range r.URL.Query() {
			gotQuery[k] = v[0]
		}
		verifySign(t, r, b, "testsecret")
		w.Write([]byte(`{"status":0,"data":{"total_count":1,"order":[]}}`))
	})

	_, err := c.PageCall(context.Background(), "sales.TradeQuery.queryWithDetail",
		NewPager(100, 2, true),
		map[string]any{"start_time": "2020-01-01 00:00:00", "end_time": "2020-01-01 01:00:00"})
	if err != nil {
		t.Fatalf("PageCall: %v", err)
	}
	if got := gotQuery["page_size"]; got != "100" {
		t.Errorf("page_size = %q", got)
	}
	if got := gotQuery["page_no"]; got != "2" {
		t.Errorf("page_no = %q", got)
	}
	if got := gotQuery["calc_total"]; got != "1" {
		t.Errorf("calc_total = %q", got)
	}
	// 报文必须是 JSON 数组，且与参与签名的 body 完全一致
	var arr []map[string]any
	if err := json.Unmarshal(gotBody, &arr); err != nil {
		t.Fatalf("body 不是 JSON 数组: %v (%s)", err, gotBody)
	}
	if len(arr) != 1 || arr[0]["start_time"] != "2020-01-01 00:00:00" {
		t.Errorf("body = %s", gotBody)
	}
	// 分页参数不参与 body，但参与签名（服务端 verifySign 已校验）
	if _, ok := gotQuery["body"]; ok {
		t.Errorf("body 不应出现在 URL 参数中")
	}
}

func TestClient_Call_TrimsTrailingNils(t *testing.T) {
	var gotBody []byte
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"status":0}`))
	})
	_, err := c.Call(context.Background(), "sales.RawTrade.pushSelf", "test", []any{map[string]any{"tid": "T1"}}, nil, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if string(gotBody) != `["test",[{"tid":"T1"}]]` {
		t.Errorf("body = %s, 尾部 nil 参数应被去掉", gotBody)
	}
}

func TestClient_Call_APIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":200,"message":"店铺编号不存在或已停用"}`))
	})
	_, err := c.Call(context.Background(), "sales.TradeQuery.queryWithDetail", map[string]any{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("期望 *APIError，得到 %v", err)
	}
	if apiErr.Status != 200 || apiErr.Message != "店铺编号不存在或已停用" {
		t.Errorf("APIError = %+v", apiErr)
	}
	if apiErr.Method != "sales.TradeQuery.queryWithDetail" {
		t.Errorf("APIError.Method = %q", apiErr.Method)
	}
	if IsRateLimited(err) {
		t.Errorf("普通业务错误不应被判定为限频")
	}
}

func TestClient_Call_RateLimitRetried(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Write([]byte(`{"status":100,"message":"超过每分钟最大调用频率限制，请稍后重试"}`))
			return
		}
		w.Write([]byte(`{"status":0,"data":1}`))
	})
	resp, err := c.Call(context.Background(), "wms.StockSpec.search", map[string]any{})
	if err != nil {
		t.Fatalf("限频后应重试成功: %v", err)
	}
	if calls != 2 {
		t.Errorf("调用次数 = %d, want 2", calls)
	}
	if string(resp.Data) != "1" {
		t.Errorf("data = %s", resp.Data)
	}
}

func TestClient_Call_RateLimitRetriesExhausted(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"status":100,"message":"超过每分钟最大并发次数限制，请稍后重试"}`))
	})
	c.maxRetries = 3
	_, err := c.Call(context.Background(), "wms.StockSpec.search")
	if err == nil {
		t.Fatal("超过最大重试次数后应返回错误")
	}
	if calls != 4 { // 首次 + 3 次重试
		t.Errorf("调用次数 = %d, want 4", calls)
	}
	if !IsRateLimited(err) {
		t.Errorf("最终错误应为限频错误: %v", err)
	}
}

func TestClient_Call_ServerErrorRetried(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"status":0}`))
	})
	if _, err := c.Call(context.Background(), "system.Core.now"); err != nil {
		t.Fatalf("5xx 后应重试成功: %v", err)
	}
	if calls != 2 {
		t.Errorf("调用次数 = %d, want 2", calls)
	}
}

func TestClient_Call_NonJSONResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>gateway error</html>`))
	})
	_, err := c.Call(context.Background(), "system.Core.now")
	if err == nil || !strings.Contains(err.Error(), "不是有效 JSON") {
		t.Fatalf("期望非 JSON 响应错误，得到 %v", err)
	}
}

func TestClient_Call_ContextCanceled(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":0}`))
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Call(ctx, "system.Core.now"); !errors.Is(err, context.Canceled) {
		t.Fatalf("期望 context.Canceled，得到 %v", err)
	}
}

func TestNewClient(t *testing.T) {
	if _, err := NewClient("sid", "key", "no-colon-secret"); err == nil {
		t.Error("appsecret 缺少冒号应报错")
	}
	if _, err := NewClient("", "key", "a:b"); err == nil {
		t.Error("sid 为空应报错")
	}
	c, err := NewClient("sid", "key", "a:b", WithBaseURL("http://127.0.0.1:1/openapi"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.baseURL != "http://127.0.0.1:1/openapi" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
	if c.Services.TradeQuery == nil || c.Services.Goods == nil || c.Services.System == nil {
		t.Error("服务未初始化")
	}
}

func TestPager(t *testing.T) {
	p := NewPager(50, 0, false)
	if p.PageSize != 50 || p.PageNo != 0 || p.CalcTotal {
		t.Errorf("Pager = %+v", p)
	}
}
