package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"cnccool/internal/domain"
)

// cors 处理跨域。
//
// 开发时前端跑在 Vite dev server（5173），API 在 8080，属于跨域；
// 生产部署时两者同源（都由 nginx 提供），这段配置自然就不会生效。
func (s *Server) cors(next http.Handler) http.Handler {
	allowed := make(map[string]bool, len(s.cfg.CORSOrigins))
	for _, o := range s.cfg.CORSOrigins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(authCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		user, err := s.repo.UserBySession(r.Context(), cookie.Value)
		if err != nil {
			http.SetCookie(w, &http.Cookie{Name: authCookieName, Value: "", Path: "/api", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
			writeError(w, http.StatusUnauthorized, "登录已失效，请重新登录")
			return
		}
		next.ServeHTTP(w, r.WithContext(domain.WithUser(r.Context(), user)))
	})
}

func (s *Server) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		parsed, err := url.Parse(origin)
		if err == nil && strings.EqualFold(parsed.Host, r.Host) {
			next.ServeHTTP(w, r)
			return
		}
		for _, allowed := range s.cfg.CORSOrigins {
			if origin == strings.TrimRight(allowed, "/") {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeError(w, http.StatusForbidden, "请求来源不受信任")
	})
}

// recoverer 兜住 panic，避免单个请求出错把整个服务带崩。
// 现场软件最忌讳"点一下就整个程序没了"，这一层是底线保障。
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("请求处理发生 panic", "panic", rec, "method", r.Method, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusWriter 记录真实写出的状态码，供访问日志使用。
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.status = http.StatusOK
		w.wrote = true
	}
	return w.ResponseWriter.Write(b)
}

// accessLog 记录每个请求的方法、路径、状态码和耗时。
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"ms", time.Since(start).Milliseconds())
	})
}

// trimTrailingSlash 在进入路由前去掉多余的尾部斜杠。
//
// 写成包装器而不是 chi 中间件：chi 的 Use 中间件是在路由匹配之后才执行的，
// 在那个位置改 URL 已经来不及了。
func trimTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 1 && strings.HasSuffix(r.URL.Path, "/") {
			r.URL.Path = strings.TrimRight(r.URL.Path, "/")
		}
		next.ServeHTTP(w, r)
	})
}
