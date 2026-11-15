package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"cnc-manager/internal/domain"
)

const authCookieName = "cnc_session"

func (s *Server) sessionCookie(r *http.Request, token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: authCookieName, Value: token, Path: "/api", HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode, MaxAge: maxAge,
	}
}

// requireAuth 校验会话 Cookie，并把用户放入上下文。
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		c, err := r.Cookie(authCookieName)
		if err != nil || c.Value == "" {
			s.recordUnauthenticated(r)
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		user, err := s.repo.UserBySession(r.Context(), c.Value)
		if err != nil {
			http.SetCookie(w, s.sessionCookie(r, "", -1))
			s.recordUnauthenticated(r)
			writeError(w, http.StatusUnauthorized, "登录已失效，请重新登录")
			return
		}
		next.ServeHTTP(w, r.WithContext(domain.WithUser(r.Context(), user)))
	})
}

// sameOrigin 是 CSRF 防线：写请求必须带 Origin（或 Referer）且与 Host 一致。
// 两者都缺失时拒绝——浏览器对跨站写请求一定会带 Origin，缺失只可能是非浏览器客户端，
// 这类客户端补一个头即可。
func (s *Server) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		source := r.Header.Get("Origin")
		if source == "" {
			source = r.Header.Get("Referer")
		}
		if source == "" {
			writeError(w, http.StatusForbidden, "缺少请求来源")
			return
		}
		u, err := url.Parse(source)
		if err != nil || !strings.EqualFold(u.Host, r.Host) {
			writeError(w, http.StatusForbidden, "请求来源不受信任")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoverer 兜住 panic，避免单个请求把整个服务带崩。
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

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// accessLog 只记录 API 请求，静态资源不记，避免刷屏。
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		s.log.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.status, "ms", time.Since(start).Milliseconds())
	})
}

// trimTrailingSlash 在路由匹配前去掉尾部斜杠（chi 的 Use 中间件在匹配后执行，改 URL 来不及）。
func trimTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 1 && strings.HasSuffix(r.URL.Path, "/") {
			r.URL.Path = strings.TrimRight(r.URL.Path, "/")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recordUnauthenticated(r *http.Request) {
	if err := s.repo.RecordEvent(r.Context(), domain.AuditEntry{Action: "request", EntityType: "http", Result: "failed", Detail: "未登录或会话失效"}); err != nil {
		s.log.Error("未登录访问日志写入失败", "err", err)
	}
}
