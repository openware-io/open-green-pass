// Package paging 提供列表分页参数钳制与 DB LIMIT/OFFSET 计算，以及泛型分页结果。
package paging

// DefaultPageSize 默认每页条数。
const DefaultPageSize = 20

// MaxPageSize 每页上限，防止大页拖垮 DB。
const MaxPageSize = 200

// Request 分页请求。
type Request struct {
	Page     int
	PageSize int
}

// New 构造并钳制分页参数（page>=1, 1<=size<=MaxPageSize）。
func New(page, size int) Request {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = DefaultPageSize
	}
	if size > MaxPageSize {
		size = MaxPageSize
	}
	return Request{Page: page, PageSize: size}
}

// Offset 返回 DB OFFSET。
func (r Request) Offset() int { return (r.Page - 1) * r.PageSize }

// Limit 返回 DB LIMIT。
func (r Request) Limit() int { return r.PageSize }

// Result 泛型分页结果。
type Result[T any] struct {
	Total int64 // 总记录数（不含分页）
	Items []T
}

// NewResult 构造分页结果。
func NewResult[T any](total int64, items []T) Result[T] {
	if items == nil {
		items = []T{}
	}
	return Result[T]{Total: total, Items: items}
}
