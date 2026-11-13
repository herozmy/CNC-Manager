// Package httpapi 提供路由、参数校验与 HTTP 响应。
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// New 返回根路由。version 来自构建时注入的 VERSION。
func New(version string) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/meta", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
	})
	return r
}
