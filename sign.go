package wdt

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strings"
)

// sign 计算旗舰版开放平台签名。
//
// 算法：将 params（含 JSON 报文串 body，不含 sign 本身）按参数名升序排序，
// 拼接 secret + 依次拼接每个参数名和参数值 + secret，对整串取 32 位小写 MD5。
func sign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(secret)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	b.WriteString(secret)

	sum := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
