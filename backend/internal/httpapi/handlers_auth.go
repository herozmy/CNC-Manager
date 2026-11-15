package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"cnc-manager/internal/domain"
)

type credentialsInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	required, err := s.repo.SetupRequired(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setupRequired": required})
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	var in credentialsInput
	if !decodeJSON(w, r, &in) {
		return
	}
	user, err := s.repo.CreateFirstUser(r.Context(), in.Username, in.DisplayName, in.Password)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.startSession(w, r, user)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in credentialsInput
	if !decodeJSON(w, r, &in) {
		return
	}
	key := loginAttemptKey(r, in.Username)
	if s.loginBlocked(key) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "登录失败次数过多，请一分钟后重试")
		return
	}
	user, token, err := s.repo.Login(r.Context(), in.Username, in.Password, s.sessionLifetime)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			s.recordLoginFailure(key)
			if logErr := s.repo.RecordEvent(r.Context(), domain.AuditEntry{Action: "login", EntityType: "app_user", Result: "failed", Detail: "用户名或密码错误"}); logErr != nil {
				s.log.Error("登录失败日志写入失败", "err", logErr)
			}
		}
		s.fail(w, err)
		return
	}
	s.clearLoginFailures(key)
	http.SetCookie(w, s.sessionCookie(r, token, int(s.sessionLifetime.Seconds())))
	writeJSON(w, http.StatusOK, user)
}

func loginAttemptKey(r *http.Request, username string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host + "|" + strings.ToLower(strings.TrimSpace(username))
}

// 同一 IP + 用户名连续失败 5 次封锁一分钟。状态在内存里，重启清零即可接受。
func (s *Server) loginBlocked(key string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	a := s.loginAttempts[key]
	if a.blockedUntil.After(time.Now()) {
		return true
	}
	if !a.blockedUntil.IsZero() {
		delete(s.loginAttempts, key)
	}
	return false
}

func (s *Server) recordLoginFailure(key string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	a := s.loginAttempts[key]
	a.failures++
	if a.failures >= 5 {
		a.blockedUntil = time.Now().Add(time.Minute)
	}
	s.loginAttempts[key] = a
}

func (s *Server) clearLoginFailures(key string) {
	s.loginMu.Lock()
	delete(s.loginAttempts, key)
	s.loginMu.Unlock()
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user domain.User) {
	token, _, err := s.repo.CreateSession(r.Context(), user.ID, s.sessionLifetime)
	if err != nil {
		s.fail(w, err)
		return
	}
	http.SetCookie(w, s.sessionCookie(r, token, int(s.sessionLifetime.Seconds())))
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := domain.UserFromContext(r.Context())
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DisplayName *string `json:"displayName"`
	}
	if !decodeJSONLimit(w, r, &in, 8<<10) {
		return
	}
	if in.DisplayName == nil {
		writeError(w, http.StatusBadRequest, "请输入显示名称")
		return
	}
	user, _ := domain.UserFromContext(r.Context())
	updated, err := s.repo.UpdateDisplayName(r.Context(), user.ID, *in.DisplayName)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSONLimit(w, r, &in, 8<<10) {
		return
	}
	user, _ := domain.UserFromContext(r.Context())
	key := "change-password|" + loginAttemptKey(r, user.Username)
	if s.loginBlocked(key) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "原密码错误次数过多，请一分钟后重试")
		return
	}
	if err := s.repo.ChangePassword(r.Context(), user.ID, in.CurrentPassword, in.NewPassword); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			s.recordLoginFailure(key)
			// An incorrect current password does not invalidate a valid login.
			writeError(w, http.StatusBadRequest, "原密码不正确")
		} else {
			s.fail(w, err)
		}
		return
	}
	s.clearLoginFailures(key)
	http.SetCookie(w, s.sessionCookie(r, "", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(authCookieName); err == nil {
		_ = s.repo.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, s.sessionCookie(r, "", -1))
	w.WriteHeader(http.StatusNoContent)
}
