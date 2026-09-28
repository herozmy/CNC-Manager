package httpapi

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"cnccool/internal/update"
)

// maxPackageBytes 限制上传的离线包大小。
//
// 免安装包现在 6 MB 上下，给到 200 MB 足够宽松，也挡住了明显的滥用
// （压缩包是会被完整解压到磁盘上的）。
const maxPackageBytes = 200 << 20

// handleUpdateCheck 问仓库上有没有新版本。
//
// 约定：查不到不算错误。车间没网是常态，返回 200 加一个 error 字段，
// 界面安静地什么都不显示就好，不该弹一个红叉吓人。
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	// fresh=1 表示用户手工点的「检查更新」，这时丢掉缓存重新问一次，
	// 否则用户点了半天看到的还是半小时前的结论。
	fresh := r.URL.Query().Get("fresh") == "1"
	writeJSON(w, http.StatusOK, s.update.Check(r.Context(), s.version, fresh))
}

// handleUpdateInstall 装上用户上传的离线包。
//
// 这个接口会把 exe 换掉，等价于在本机执行任意代码，所以：
//   - 只接受来自本机的请求（见下面的 isLoopback）；
//   - 只在免安装版布局下开放，开发模式下直接拒绝。
//
// 流程：校验 -> 解压到暂存目录 -> 生成替换脚本 -> 起脱钩的脚本 -> 自己退出。
// 真正换文件的是那个脚本，因为 Windows 上运行中的 exe 锁着，服务换不了自己。
func (s *Server) handleUpdateInstall(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r.RemoteAddr) {
		writeError(w, http.StatusForbidden,
			"离线安装只能在本机操作。服务一旦监听 0.0.0.0，这个接口就等于把改程序的能力交给了整个局域网。")
		return
	}
	if !s.layout.CanSelfUpdate {
		writeError(w, http.StatusBadRequest,
			"当前不是免安装版布局（可执行文件旁边没有 web 目录），无法自更新。"+
				"开发模式下请用 scripts\\build-release.cmd 重新打包。")
		return
	}
	if s.restart == nil {
		writeError(w, http.StatusInternalServerError, "服务没有配置重启动作，无法完成安装")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPackageBytes)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "读取上传内容失败（可能超过了大小限制）："+err.Error())
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "没有收到离线包文件")
		return
	}
	defer func() { _ = file.Close() }()

	tmp, err := os.CreateTemp("", "cnccool-update-*.zip")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "创建临时文件失败："+err.Error())
		return
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		writeError(w, http.StatusBadRequest, "保存上传内容失败："+err.Error())
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "保存上传内容失败："+err.Error())
		return
	}

	stage := s.layout.StageDir()
	if err := os.RemoveAll(stage); err != nil {
		writeError(w, http.StatusInternalServerError, "清理暂存目录失败："+err.Error())
		return
	}
	if err := update.Extract(tmpPath, stage); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	root, err := update.FindPackageRoot(stage)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	newVersion, err := update.Verify(root)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 比当前版本还旧的包一律不装。装上去只会让人以为更新成功了，
	// 而其实功能反而变少了；要退回旧版本请手工替换整个文件夹。
	if update.CompareVersion(newVersion, s.version) < 0 {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("包里的版本是 %s，比当前运行的 %s 还旧，已拒绝安装", newVersion, s.version))
		return
	}

	// 包可能多套了一层文件夹，替换脚本按固定路径找东西，这里先摊平
	if err := update.Flatten(root, stage); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	script, err := update.WriteApplyScript(update.ApplyRequest{
		Layout: s.layout,
		PID:    os.Getpid(),
		Addr:   s.cfg.Addr,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := update.SpawnDetached(script); err != nil {
		writeError(w, http.StatusInternalServerError, "启动替换脚本失败："+err.Error())
		return
	}

	s.log.Info("开始安装离线包",
		"from", s.version, "to", newVersion, "stage", stage, "script", script)

	// 先把响应发出去，再退出，否则前端只会看到一个连接被断开。
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    newVersion,
		"previous":   s.version,
		"restarting": true,
	})
	s.restart()
}

// isLoopback 判断请求是不是从本机发出来的。
//
// 只看 RemoteAddr，不看 X-Forwarded-For —— 那个头是客户端随便写的，
// 拿它做安全判断等于没判断。
func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
