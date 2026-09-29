package wdt

import (
	"strings"
	"testing"
)

// 签名算法：md5(secret + 按参数名升序拼接的 key+value + secret)，32 位小写。
// 期望值由 md5sum 独立计算。
func TestSign(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]string
		secret string
		want   string
	}{
		{
			name:   "两个参数",
			params: map[string]string{"a": "1", "b": "2"},
			secret: "s",
			// md5("sa1b2s")
			want: "5ee29085af57d942f21f1c5ba3c2a90a",
		},
		{
			name:   "键升序排序",
			params: map[string]string{"c": "b2c", "b": "1"},
			secret: "testsecret",
			// md5("testsecret" + "b1" + "cb2c" + "testsecret")
			want: "4176bfbb890d63280f137053be381691",
		},
		{
			name: "完整公共参数(与 PHP SDK makeSign 一致)",
			params: map[string]string{
				"body":      "[]",
				"key":       "wdt_test",
				"method":    "system.Core.now",
				"salt":      "dd",
				"sid":       "wdtapi3",
				"timestamp": "100",
				"v":         "1.0",
			},
			secret: "aa",
			// md5("aa" + "body[]" + "keywdt_test" + "methodsystem.Core.now" + "saltdd" +
			//     "sidwdtapi3" + "timestamp100" + "v1.0" + "aa")
			want: "b5745bf1f4721a0d16e9a68cb7513032",
		},
		{
			name:   "sign 键不参与签名",
			params: map[string]string{"a": "1", "sign": "其他值也不影响结果"},
			secret: "s",
			want:   "585b98956d9738edec5cbd8443f7a228", // md5("sa1s")，与仅 {a:1} 时一致
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sign(tc.params, tc.secret)
			if got != tc.want {
				t.Errorf("sign() = %q, want %q", got, tc.want)
			}
			if got != strings.ToLower(got) {
				t.Errorf("签名应为小写 hex，得到 %q", got)
			}
		})
	}
}
