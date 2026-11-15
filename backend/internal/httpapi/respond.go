package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"cnc-manager/internal/domain"
	"cnc-manager/internal/ncstore"
)

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

// fail 把领域错误映射成 HTTP 状态码；映射只在这一处发生。
func (s *Server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "没有此操作的权限")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ncstore.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, ncstore.ErrEmpty):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	default:
		s.log.Error("请求处理失败", "err", err)
		writeError(w, http.StatusInternalServerError, "服务器内部错误")
	}
}

// decodeJSON 解析请求体（上限 1MB）。不拒绝未知字段，便于前端表单多带字段。
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeJSONLimit(w, r, dst, 1<<20)
}

func decodeJSONLimit(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "请求内容超过大小上限")
			return false
		}
		writeError(w, http.StatusBadRequest, "请求内容格式错误："+err.Error())
		return false
	}
	return true
}

// pathID 取路径参数里的正整数编号。
func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, name)), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "路径参数 "+name+" 不是合法的编号")
		return 0, false
	}
	return id, true
}
