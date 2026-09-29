// Package httperr 把 pkg/errors 的类型化错误映射为 HTTP 状态码与统一错误响应体。
package httperr

import (
	"net/http"

	gperr "github.com/openware-io/open-green-pass/pkg/errors"
)

// Body 统一错误响应结构。
type Body struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// Status 返回错误对应的 HTTP 状态码。
func Status(err error) int {
	switch gperr.KindOf(err) {
	case gperr.KindNotFound:
		return http.StatusNotFound
	case gperr.KindConflict:
		return http.StatusConflict
	case gperr.KindValidation:
		return http.StatusBadRequest
	case gperr.KindUnauthorized:
		return http.StatusUnauthorized
	case gperr.KindForbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// Map 返回 (状态码, 响应体)。内部错误隐藏底层细节，避免信息泄漏。
func Map(err error) (int, Body) {
	status := Status(err)
	msg := err.Error()
	if status == http.StatusInternalServerError {
		msg = "internal error"
	}
	return status, Body{Code: codeOf(status), Msg: msg}
}

func codeOf(status int) string {
	switch status {
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusBadRequest:
		return "VALIDATION_ERROR"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	default:
		return "INTERNAL"
	}
}
