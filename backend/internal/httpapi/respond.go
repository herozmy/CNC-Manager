package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"cnccool/internal/domain"
)

// errorBody 是统一的错误响应体。前端只需要读 error 字段并弹提示。
type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	// 响应头已经发出，这里出错也无法再改状态码，交给访问日志即可
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

// fail 把领域错误映射成合适的 HTTP 状态码。
//
// 领域层不认识 HTTP，HTTP 层不认识 SQL——整个错误映射只在这一个地方发生，
// 以后加错误类型只需改这一个 switch。
func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		s.log.Error("请求处理失败", "err", err)
		writeError(w, http.StatusInternalServerError, "服务器内部错误："+err.Error())
	}
}

// decodeJSON 解析请求体。
//
// 刻意不开启 DisallowUnknownFields：前端表单往往会多带 id、createdAt 这类字段，
// 严格拒绝会让联调变得非常痛苦，得不偿失。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "请求内容格式错误："+err.Error())
		return false
	}
	return true
}

// pathID 取路径参数里的编号。
func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := strings.TrimSpace(chi.URLParam(r, name))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "路径参数 "+name+" 不是合法的编号")
		return 0, false
	}
	return id, true
}

// queryInt 取整数查询参数，缺失或非法时返回默认值。
func queryInt(r *http.Request, name string, def int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// queryInt64 取 64 位整数查询参数，缺失或非法时返回 0。
func queryInt64(r *http.Request, name string) int64 {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
