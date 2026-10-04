package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"cnccool/internal/domain"
	"cnccool/internal/repo"
)

const (
	authCookieName  = "cnccool_session"
	sessionLifetime = 12 * time.Hour
)

type credentialsInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	required, err := s.repo.AuthSetupRequired(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setupRequired": required})
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	var input credentialsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := s.repo.CreateFirstUser(r.Context(), input.Username, input.DisplayName, input.Password)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.startSession(w, r, user)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input credentialsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	key := loginAttemptKey(r, input.Username)
	if wait := s.loginWait(key); wait > 0 {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "登录失败次数过多，请一分钟后重试")
		return
	}
	user, err := s.repo.Authenticate(r.Context(), input.Username, input.Password)
	if err != nil {
		if errors.Is(err, repo.ErrInvalidCredentials) {
			s.recordLoginFailure(key)
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		s.fail(w, err)
		return
	}
	s.clearLoginFailures(key)
	s.startSession(w, r, user)
}

func loginAttemptKey(r *http.Request, username string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host + "|" + strings.ToLower(strings.TrimSpace(username))
}

func (s *Server) loginWait(key string) time.Duration {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	attempt := s.loginAttempts[key]
	if attempt.blockedUntil.After(time.Now()) {
		return time.Until(attempt.blockedUntil)
	}
	if !attempt.blockedUntil.IsZero() {
		delete(s.loginAttempts, key)
	}
	return 0
}

func (s *Server) recordLoginFailure(key string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	attempt := s.loginAttempts[key]
	attempt.failures++
	if attempt.failures >= 5 {
		attempt.blockedUntil = time.Now().Add(time.Minute)
	}
	s.loginAttempts[key] = attempt
}

func (s *Server) clearLoginFailures(key string) {
	s.loginMu.Lock()
	delete(s.loginAttempts, key)
	s.loginMu.Unlock()
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user domain.User) {
	token, expires, err := s.repo.CreateSession(r.Context(), user.ID, sessionLifetime)
	if err != nil {
		s.fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: authCookieName, Value: token, Path: "/api", HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
		Expires: expires, MaxAge: int(sessionLifetime.Seconds()),
	})
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := domain.UserFromContext(r.Context())
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(authCookieName); err == nil {
		_ = s.repo.DeleteSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: authCookieName, Value: "", Path: "/api", HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}
