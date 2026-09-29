package errors

import (
	stderrors "errors"
	"testing"
)

func TestNotFound(t *testing.T) {
	e := NotFound("team not found")
	if e.Kind != KindNotFound {
		t.Fatalf("Kind=%v, want KindNotFound", e.Kind)
	}
	if e.Error() != "team not found" {
		t.Fatalf("Error()=%q", e.Error())
	}
	if KindOf(e) != KindNotFound {
		t.Fatal("KindOf 未识别 NotFound")
	}
}

func TestWrapUnwrap(t *testing.T) {
	base := stderrors.New("db down")
	e := Wrap(KindInternal, "query failed", base)
	if !stderrors.Is(e, base) {
		t.Fatal("Unwrap 后 errors.Is 应命中底层错误")
	}
	if !stderrors.As(e, &e) {
		t.Fatal("errors.As 应命中 *Error")
	}
}

func TestKindOfPlain(t *testing.T) {
	if KindOf(stderrors.New("x")) != KindInternal {
		t.Fatal("普通错误应归类 KindInternal")
	}
	if KindOf(nil) != KindInternal {
		t.Fatal("nil 应归类 KindInternal")
	}
}
