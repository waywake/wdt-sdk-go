// Package wdt 是旺店通旗舰版 ERP 开放平台的 Go 语言 SDK。
//
// SDK 覆盖平台公开的全部 188 个接口（订单、基础、货品、采购、库存、售后等类别），
// 负责公共参数组装、sign 签名、JSON 报文编码、分页参数以及限频重试。
//
// # 快速开始
//
//	appsecret 由平台按 "secret:salt" 格式分配，冒号前为 secret，冒号后为 salt。
//
//	client, err := wdt.NewClient("卖家账号sid", "appkey", "secret:salt")
//	if err != nil {
//		log.Fatal(err)
//	}
//	resp, err := client.TradeQuery.QueryWithDetail(ctx, &wdt.TradeQueryWithDetailParams{
//		StartTime: "2024-01-01 00:00:00",
//		EndTime:   "2024-01-01 01:00:00",
//	}, wdt.NewPager(100, 0, true))
//	if err != nil {
//		log.Fatal(err)
//	}
//	var data struct {
//		TotalCount int             `json:"total_count"`
//		Order      []json.RawMessage `json:"order"`
//	}
//	if err := resp.Decode(&data); err != nil {
//		log.Fatal(err)
//	}
//
// # 通用调用
//
// 未提供封装方法的接口可以直接使用通用调用：
//
//	resp, err := client.Call(ctx, "system.Core.now")
//
// # 环境
//
// 默认连正式环境 http://wdt.wangdian.cn/openapi，测试环境可用
// WithBaseURL("http://47.92.239.46/openapi") 切换。
//
// 文档：https://open.wangdian.cn/qjb/open/apidoc
package wdt
