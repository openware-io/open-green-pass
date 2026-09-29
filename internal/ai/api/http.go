// Package api AI 域 HTTP 接入层。
// 业务路由由 P1 注册到 gateway mux；此处为层占位。
package api

import "net/http"

// Register 注册 AI 域路由（P1 实现）。
func Register(mux *http.ServeMux) {
	_ = mux
}
