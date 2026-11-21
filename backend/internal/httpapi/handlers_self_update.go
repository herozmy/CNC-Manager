package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"regexp"
	"time"
)

var releaseTag = regexp.MustCompile(`^v(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)

type updateState struct {
	Phase   string `json:"phase"`
	Target  string `json:"target,omitempty"`
	Message string `json:"message,omitempty"`
}

// The container launcher owns the socket and restarts only the application process.
func (s *Server) handleSelfUpdate(w http.ResponseWriter, r *http.Request) {
	command := "status"
	if r.Method == http.MethodPost {
		var in struct {
			Target string `json:"target"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		if !releaseTag.MatchString(in.Target) {
			writeError(w, http.StatusBadRequest, "目标版本号无效")
			return
		}
		command = in.Target
	}
	path := os.Getenv("CNC_UPDATE_SOCKET")
	if path == "" {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, updateState{Phase: "disabled"})
		} else {
			writeError(w, http.StatusServiceUnavailable, "当前运行方式未启用容器内更新")
		}
		return
	}
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "无法连接容器内启动器")
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	var state updateState
	if _, err = conn.Write([]byte(command + "\n")); err == nil {
		err = json.NewDecoder(conn).Decode(&state)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "更新服务响应异常，请重新确认状态")
		return
	}
	if state.Phase == "rejected" {
		writeError(w, http.StatusConflict, state.Message)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, state)
}
