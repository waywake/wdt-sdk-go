package wdt

// Ptr 返回 v 的指针。
//
// 生成的参数结构体中，选填的对象/列表字段为指针类型，可直接用 &T{...} 赋值；
// Ptr 主要用于辅助构造这类指针，例如：
//
//	params.DetailList = []*wdt.XxxDetail{...}
//	paramsSomeObject = wdt.Ptr(wdt.XxxObject{...})
func Ptr[T any](v T) *T {
	return &v
}
