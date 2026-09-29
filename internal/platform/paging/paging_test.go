package paging

import "testing"

func TestClamp(t *testing.T) {
	if r := New(0, 0); r.Page != 1 || r.PageSize != DefaultPageSize {
		t.Fatalf("默认值错: %+v", r)
	}
	if r := New(3, 500); r.Page != 3 || r.PageSize != MaxPageSize {
		t.Fatalf("上限错: %+v", r)
	}
	if r := New(2, 10); r.Offset() != 10 || r.Limit() != 10 {
		t.Fatalf("offset/limit 错: %+v", r)
	}
}

func TestResult(t *testing.T) {
	r := NewResult[int64](3, []int64{1, 2, 3})
	if r.Total != 3 || len(r.Items) != 3 {
		t.Fatalf("Result 错: %+v", r)
	}
	empty := NewResult[string](0, nil)
	if empty.Items == nil {
		t.Fatal("nil items 应规范化为空切片")
	}
}
