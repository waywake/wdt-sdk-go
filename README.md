# wdt-sdk-go

旺店通旗舰版（QJB）开放平台 Go 语言 SDK。

覆盖开放平台公开的全部 **188 个接口**（订单、基础、货品、采购、库存、售后等），负责公共参数组装、`sign` 签名、JSON 报文编码、分页参数与限频自动重试。接口目录与参数定义由官方文档自动提取生成（见 [scripts/](scripts/)），与 [apidoc](https://open.wangdian.cn/qjb/open/apidoc) 一一对应。

> 如果你使用的是旺店通 ERP 经典版（`https://api.wangdian.cn/openapi2/xxx.php` 系列接口），本 SDK 不适用——两者协议不同。

## 安装

```bash
go get github.com/waywake/wdt-sdk-go
```

## 快速开始

`appsecret` 由旺店通按 `secret:salt` 格式分配，冒号前为 secret（用于签名），冒号后为 salt（作为公共参数）。

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	wdt "github.com/waywake/wdt-sdk-go"
)

func main() {
	client, err := wdt.NewClient("卖家账号sid", "appkey", "secret:salt")
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.TradeQuery.QueryWithDetail(context.Background(),
		&wdt.TradeQueryWithDetailParams{
			StartTime: "2024-01-01 00:00:00",
			EndTime:   "2024-01-01 01:00:00", // 时间跨度最大 60 分钟
		},
		wdt.NewPager(100, 0, true))
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
	fmt.Println("订单总数:", data.TotalCount)
}
```

## 客户端配置

```go
client, err := wdt.NewClient(sid, appkey, appsecret,
	wdt.WithBaseURL("http://47.92.239.46/openapi"), // 测试环境；默认正式环境 http://wdt.wangdian.cn/openapi
	wdt.WithTimeout(30*time.Second),
	wdt.WithMaxRetries(2),        // 限频/网络错误自动重试次数，0 关闭
	wdt.WithRetryDelay(500*time.Millisecond), // 重试基础间隔，指数退避
	wdt.WithHTTPClient(&http.Client{Timeout: 20 * time.Second}), // 自定义 HTTP 客户端
)
```

## 使用方式

### 封装方法（推荐）

每个接口都有对应的封装方法，参数为类型化结构体，字段注释即官方文档说明：

```go
// 分页查询库存
resp, err := client.StockSpec.Search(ctx,
	&wdt.StockSpecSearchParams{WarehouseNo: "wdt-test", SpecNos: []string{"g1-s1"}},
	wdt.NewPager(100, 0, true))

// 推送原始订单（位置参数按文档顺序传入，尾部可选参数传 nil 会被自动省略）
_, err = client.RawTrade.PushSelf(ctx, "myshop", trades, orders, nil)

// 创建采购单
_, err = client.PurchaseOrder.CreateOrder(ctx, &wdt.PurchaseOrderCreateOrderParams{...})

// 无分页的接口不传 pager
_, err = client.System.Now(ctx)
```

参数结构体的约定：

- **必填字段**为值类型，始终出现在请求报文中；
- **选填标量字段**带 `omitempty`，零值（`""`、`0`、`false`）不会发送；
- **选填对象/列表字段**为指针（`*T`、`[]*T`），nil 时不会发送。

### 分页

分页查询接口的方法签名带 `*Pager` 参数：

```go
const pageSize = 100
for page := 0; ; page++ {
	resp, err := client.StockSpec.Search(ctx, params, wdt.NewPager(pageSize, page, false))
	if err != nil {
		return err
	}
	var data struct {
		Order []json.RawMessage `json:"order"`
	}
	if err := resp.Decode(&data); err != nil {
		return err
	}
	if len(data.Order) < pageSize {
		break
	}
}
```

`CalcTotal: true` 时响应 data 中带 `total_count`，按需开启（有额外开销）。

### 通用调用

封装方法覆盖全部 188 个接口；如需直接调用，可使用通用入口，业务参数按文档以 JSON 数组发送：

```go
resp, err := client.Call(ctx, "sales.TradeQuery.queryWithDetail", map[string]any{
	"start_time": "2024-01-01 00:00:00",
	"end_time":   "2024-01-01 01:00:00",
})

// 带分页参数
resp, err = client.PageCall(ctx, "wms.StockSpec.search", wdt.NewPager(100, 0, false), params)
```

### 响应与错误处理

```go
resp, err := client.Goods.Push(ctx, goodsInfo, specList)
if err != nil {
	var apiErr *wdt.APIError
	if errors.As(err, &apiErr) {
		log.Printf("平台错误 status=%d: %s", apiErr.Status, apiErr.Message)
	}
	return err
}
// resp.Status == 0，业务数据在 resp.Data（json.RawMessage）
```

- 平台返回 `status != 0` 时返回 `*APIError`；
- 限频/限并发错误（`status=100`）自动按指数退避重试，可用 `wdt.IsRateLimited(err)` 识别；
- 网络错误与 5xx 同样自动重试，`context` 取消立即中止。

## 接口目录（86 个服务，188 个接口）

方法名与官方服务名对应，例如 `client.TradeQuery.QueryWithDetail` ↔ `sales.TradeQuery.queryWithDetail`。完整参数定义见各服务源码注释与[官方文档](https://open.wangdian.cn/qjb/open/apidoc)。

| 类别 | 服务 |
|---|---|
| 订单（trade.go） | `TradeQuery` `RawTrade` `LogisticsSync` `StockSync` `TradeEdit` `TradeImport` `SalesPayment` `SalesJitRefund` `StockoutSales` `InvoiceOrder` `RawPayment` `AlipayAccountCheck` `FinancePayment` `CustomAttr` |
| 基础（basic.go） | `Logistics` `PurchaseProvider` `Warehouse` `Shop` `Employee` `VirtualWarehouse` `OperationReason` `Address` `Composite` `System` |
| 货品（goods.go） | `Goods` `Suite` `ApiGoods` `GoodsClass` `GoodsBrand` `Category` `Barcode` `Bom` `Process` `SettleProcess` |
| 采购（purchase.go） | `ProviderGoods` `PurchaseOrder` `PurchaseReturn` `PurchaseApply` `StockInPreOrder` `StockinPurchase` `StockoutPurchaseReturn` `SettlePurchase` `SettlePurchaseReturn` |
| 库存（stock.go） | `StockSpec` `StockPd` `MoveOrder` `StockinOther` `StockoutOther` `StockoutOtherQuery` `StockinTransfer` `StockoutTransfer` `StockTransfer` `StockTransferEdit` `StockShelve` `StockotherIn` `StockotherInQuery` `StockotherOut` `StockotherOutQuery` `StockoutProcess` `StockinProcess` `OuterOut` `OuterIn` `OuterTransfer` `DefectChange` `SalesPick` `PositionCapacity` `Pack` `StockinJitRefund` `StockoutJitOrder` `GoodsSN` `GoodsBatch` `GoodsPack` `StockinBase` `StockoutBase` `ErrorCorrection` `StockCost` `SettleTransfer` `SettleLogistics` `SettleOtherIn` `StockoutCollect` `StockinCollect` |
| 售后（refund.go） | `StockinRefund` `PreStockin` `SmartRefund` `RawRefund` `Refund` |

## 开发

```bash
go build ./...   # 编译
go test ./...    # 单元测试（签名、请求构造、重试、分页等）
go vet ./...
```

SDK 的服务层代码由脚本从官方文档生成，文档有更新时可重新生成：

```bash
python scripts/apidoc_scrape.py   # 抓取官方文档 → scripts/data/apis.json
python scripts/gen_sdk.py         # 生成 services.go 与各分类文件
gofmt -w *.go
```

核心文件（[client.go](client.go)、[sign.go](sign.go) 等）为手写，不依赖生成器。

### 联调验证

[examples/verify](examples/verify) 用真实凭证调用几个只读接口做端到端验证（凭证走环境变量，不落库）：

```bash
WDT_SID=xxx WDT_APPKEY=xxx WDT_APPSECRET='secret:salt' \
WDT_BASEURL='http://47.92.239.46/openapi' go run ./examples/verify
```

## 许可

[MIT](LICENSE)
