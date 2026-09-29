// Package middleware 提供 HTTP 接入层中间件：请求追踪、租户注入、操作人注入、访问日志。
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/openware-io/open-green-pass/pkg/protocol"
)

// RequestID 从 x-gp-request-id 读取；缺省生成随机 hex，注入 context 并写回响应头。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get(protocol.HeaderRequestID)
		if rid == "" {
			rid = newRequestID()
		}
		w.Header().Set(protocol.HeaderRequestID, rid)
		next.ServeHTTP(w, r)
	})
}

func newRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
