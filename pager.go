package wdt

// Pager 描述分页参数。分页查询接口通过 PageCall 传入。
//
// 平台约定 page_no 从 0 开始；calc_total 为 true 时响应 data 中
// 会包含满足查询条件的总条数（total_count），但会带来额外开销。
type Pager struct {
	// PageSize 分页大小，单量较大的卖家建议 200 以下。
	PageSize int
	// PageNo 页号，从 0 开始。
	PageNo int
	// CalcTotal 是否计算查询结果的总条数。
	CalcTotal bool
}

// NewPager 创建分页参数。
func NewPager(pageSize, pageNo int, calcTotal bool) *Pager {
	return &Pager{PageSize: pageSize, PageNo: pageNo, CalcTotal: calcTotal}
}
