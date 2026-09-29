// verify 是一个端到端联调小程序：用真实凭证调用几个只读接口，
// 验证签名、公共参数与分页逻辑是否正确。
//
// 凭证通过环境变量传入，避免落库：
//
//	WDT_SID=xxx WDT_APPKEY=xxx WDT_APPSECRET='secret:salt' \
//	WDT_BASEURL='http://47.92.239.46/openapi' go run ./examples/verify
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	wdt "github.com/waywake/wdt-sdk-go"
)

func main() {
	sid := os.Getenv("WDT_SID")
	appkey := os.Getenv("WDT_APPKEY")
	appsecret := os.Getenv("WDT_APPSECRET")
	baseURL := os.Getenv("WDT_BASEURL")
	if sid == "" || appkey == "" || appsecret == "" {
		log.Fatal("请设置环境变量 WDT_SID / WDT_APPKEY / WDT_APPSECRET（可选 WDT_BASEURL）")
	}

	opts := []wdt.Option{wdt.WithTimeout(30 * time.Second)}
	if baseURL != "" {
		opts = append(opts, wdt.WithBaseURL(baseURL))
	}
	client, err := wdt.NewClient(sid, appkey, appsecret, opts...)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("环境: %s\nsid: %s appkey: %s\n\n", baseURLLabel(baseURL), sid, appkey)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. 服务器时间（无业务参数，验证签名与公共参数）
	run(ctx, "system.Core.now", func() (*wdt.Response, error) {
		return client.System.Now(ctx)
	})

	// 2. 店铺查询（分页接口）
	run(ctx, "setting.Shop.queryShop", func() (*wdt.Response, error) {
		return client.Shop.QueryShop(ctx, &wdt.ShopQueryShopParams{}, wdt.NewPager(10, 0, true))
	})

	// 3. 物流公司查询（分页接口）
	run(ctx, "setting.Logistics.queryLogistics", func() (*wdt.Response, error) {
		return client.Logistics.QueryLogistics(ctx, &wdt.LogisticsQueryLogisticsParams{}, wdt.NewPager(10, 0, false))
	})

	// 4. 仓库查询（分页接口）
	run(ctx, "setting.Warehouse.queryWarehouse", func() (*wdt.Response, error) {
		return client.Warehouse.QueryWarehouse(ctx, &wdt.WarehouseQueryWarehouseParams{}, wdt.NewPager(10, 0, false))
	})
}

func baseURLLabel(u string) string {
	if u == "" {
		return "默认（正式环境 http://wdt.wangdian.cn/openapi）"
	}
	return u
}

func run(ctx context.Context, name string, fn func() (*wdt.Response, error)) {
	start := time.Now()
	resp, err := fn()
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		var apiErr *wdt.APIError
		if errors.As(err, &apiErr) {
			fmt.Printf("✗ %s  status=%d message=%s (%s)\n", name, apiErr.Status, apiErr.Message, elapsed)
			return
		}
		fmt.Printf("✗ %s  请求失败: %v (%s)\n", name, err, elapsed)
		return
	}
	var pretty any
	if len(resp.Data) > 0 && json.Unmarshal(resp.Data, &pretty) != nil {
		pretty = nil
	}
	out, _ := json.MarshalIndent(pretty, "  ", " ")
	s := string(out)
	if len(s) > 400 {
		s = s[:400] + " ...(截断)"
	}
	fmt.Printf("✓ %s  (%s)\n  data: %s\n\n", name, elapsed, s)
}
