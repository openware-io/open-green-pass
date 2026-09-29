// Package httpx 提供 HTTP JSON 编解码与统一错误响应。
package httpx

import (
	"encoding/json"
	"net/http"

	httperr "github.com/openware-io/open-green-pass/internal/platform/errors"
)

// WriteJSON 输出 JSON 响应。
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// WriteErr 把类型化错误映射为 HTTP 错误响应（见 httperr.Map）。
func WriteErr(w http.ResponseWriter, err error) {
	status, body := httperr.Map(err)
	WriteJSON(w, status, body)
}

// DecodeJSON 解析请求体 JSON；失败返回 *DecodeError。
func DecodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return &DecodeError{Err: err}
	}
	return nil
}

// DecodeError 请求体解析失败。
type DecodeError struct{ Err error }

func (e *DecodeError) Error() string { return "invalid json body: " + e.Err.Error() }

// Unwrap 支持 errors.Is。
func (e *DecodeError) Unwrap() error { return e.Err }
