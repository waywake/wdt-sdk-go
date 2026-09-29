package wdt

import (
	"encoding/json"
	"fmt"
)

// Response 是平台响应的公共信封。
type Response struct {
	// Status 状态码，0 表示调用成功。
	Status int `json:"status"`
	// Message 错误信息，无错误时不返回。
	Message string `json:"message"`
	// Data 业务数据，原始 JSON。不同接口的结构不同，
	// 通过 Decode 解析为具体类型，或直接用 json.Unmarshal 处理。
	Data json.RawMessage `json:"data"`
}

// Decode 将业务数据 Data 反序列化到 v。
func (r *Response) Decode(v any) error {
	if len(r.Data) == 0 {
		return fmt.Errorf("wdt: 响应不含 data 字段")
	}
	if err := json.Unmarshal(r.Data, v); err != nil {
		return fmt.Errorf("wdt: 解析 data 失败: %w", err)
	}
	return nil
}
