// Package errors 定义领域/应用层的类型化错误（供 gateway HTTP 映射与 domain 传播）。
package errors

// Kind 错误类别，用于上层（gateway）映射为 HTTP 语义。
type Kind uint8

const (
	// KindInternal 未分类的内部错误。
	KindInternal Kind = iota
	// KindNotFound 资源不存在。
	KindNotFound
	// KindConflict 状态冲突（如版本回退/重复提交）。
	KindConflict
	// KindValidation 输入校验失败。
	KindValidation
	// KindUnauthorized 未认证。
	KindUnauthorized
	// KindForbidden 无权限（RLS/角色）。
	KindForbidden
)

// Error 携带 Kind 的可比较错误。
type Error struct {
	Kind Kind
	Msg  string
	Err  error
}

// New 构造 Kind 错误。
func New(kind Kind, msg string) *Error { return &Error{Kind: kind, Msg: msg} }

// Wrap 构造带底层错误的 Kind 错误。
func Wrap(kind Kind, msg string, err error) *Error { return &Error{Kind: kind, Msg: msg, Err: err} }

// NotFound 便捷构造。
func NotFound(msg string) *Error { return New(KindNotFound, msg) }

// Conflict 便捷构造。
func Conflict(msg string) *Error { return New(KindConflict, msg) }

// Validation 便捷构造。
func Validation(msg string) *Error { return New(KindValidation, msg) }

// Unauthorized 便捷构造。
func Unauthorized(msg string) *Error { return New(KindUnauthorized, msg) }

// Forbidden 便捷构造。
func Forbidden(msg string) *Error { return New(KindForbidden, msg) }

// Error 实现 error 接口。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

// Unwrap 支持 errors.Is/As。
func (e *Error) Unwrap() error { return e.Err }

// KindOf 提取错误 Kind；非类型化错误返回 KindInternal。
func KindOf(err error) Kind {
	if err == nil {
		return KindInternal
	}
	if e, ok := err.(*Error); ok {
		return e.Kind
	}
	return KindInternal
}
