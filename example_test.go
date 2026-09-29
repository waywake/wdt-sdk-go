package wdt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// 创建客户端并查询订单（示例，不会实际执行）。
func ExampleClient() {
	client, err := NewClient(
		"你的卖家账号sid",   // sid
		"你的appkey",    // appkey，在开放平台"自助对接"申请
		"secret:salt", // appsecret，平台按 secret:salt 格式分配
		WithBaseURL("http://wdt.wangdian.cn/openapi"),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.TradeQuery.QueryWithDetail(context.Background(),
		&TradeQueryWithDetailParams{
			StartTime: "2024-01-01 00:00:00",
			EndTime:   "2024-01-01 01:00:00",
			ShopNo:    "testshop",
		},
		NewPager(100, 0, true))
	if err != nil {
		log.Fatal(err)
	}

	var data struct {
		TotalCount int             `json:"total_count"`
		Order      json.RawMessage `json:"order"`
	}
	if err := resp.Decode(&data); err != nil {
		log.Fatal(err)
	}
	fmt.Println(data.TotalCount)
}

// 分页遍历：业务方自行控制翻页，calc_total 仅在需要总数时开启。
func ExampleClient_pageLoop() {
	client, _ := NewClient("sid", "appkey", "secret:salt")

	const pageSize = 100
	for page := 0; ; page++ {
		resp, err := client.StockSpec.Search(context.Background(),
			&StockSpecSearchParams{WarehouseNo: "wdt-test"}, NewPager(pageSize, page, false))
		if err != nil {
			log.Fatal(err)
		}
		var data struct {
			Order []json.RawMessage `json:"order"`
		}
		if err := resp.Decode(&data); err != nil {
			log.Fatal(err)
		}
		if len(data.Order) < pageSize {
			break
		}
	}
}

// 通用调用：使用 Call 直呼任意接口，业务参数以 JSON 数组发送。
func ExampleClient_call() {
	client, _ := NewClient("sid", "appkey", "secret:salt")

	resp, err := client.Call(context.Background(), "system.Core.now")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.Status)
}

// 推送原始订单：位置参数按文档顺序传入（shop_no、原始单列表、原始子单列表），
// 可选的尾部参数可省略。必填字段为值类型且始终发送；选填标量字段带 omitempty（零值不发送），
// 选填对象/列表字段为指针。
func ExampleClient_pushTrade() {
	client, _ := NewClient("sid", "appkey", "secret:salt")

	trade := &RawTradePushSelfRawTrade{
		Tid:             "T20240101001",
		ProcessStatus:   10,
		TradeStatus:     30,
		RefundStatus:    0,
		PayStatus:       2,
		OrderCount:      1,
		GoodsCount:      1,
		PayMethod:       1,
		TradeTime:       "2024-01-01 12:00:00",
		EndTime:         "2024-01-01 12:00:00",
		BuyerNick:       "买家昵称",
		ReceiverName:    "张三",
		ReceiverArea:    "河南省周口市川汇区",
		ReceiverAddress: "某某街道1号",
		ReceiverMobile:  "13800000000",
		PostAmount:      10,
		Discount:        0,
		Receivable:      10,
		DeliveryTerm:    1,
		IsAutoWms:       false,
		WarehouseNo:     "wdt-test",
		Remark:          "备注",
	}
	order := &RawTradePushSelfRawTradeOrder{
		Tid:          "T20240101001",
		Oid:          "O1",
		Status:       30,
		RefundStatus: 0,
		GoodsId:      "1",
		SpecId:       "1",
		GoodsNo:      "g1",
		SpecNo:       "g1-s1",
		GoodsName:    "商品",
		Num:          1,
		Price:        10,
	}

	// 原始单与原始子单分开传入，通过 tid 关联；尾部可选参数（优惠列表）传 nil 会被自动省略
	_, err := client.RawTrade.PushSelf(context.Background(),
		"myshop", []*RawTradePushSelfRawTrade{trade}, []*RawTradePushSelfRawTradeOrder{order}, nil)
	if err != nil {
		log.Fatal(err)
	}
}

// 自定义 HTTP 客户端（代理、连接池等）。
func Example_withHTTPClient() {
	hc := &http.Client{Timeout: 20 * time.Second}
	client, err := NewClient("sid", "appkey", "secret:salt",
		WithHTTPClient(hc),
		WithMaxRetries(0))
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}
