package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"cnc-manager/internal/backup"
	"cnc-manager/internal/domain"
)

// Lock before authentication: requests queued behind an import must revalidate
// their session against the restored accounts. Other API activity cannot race
// a snapshot or observe a mixture of databases and NC files.
func (s *Server) guardMaintenance(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Role/account changes and initial ownership assignment must finish before
		// subsequent business requests authenticate. In-flight file operations finish
		// before a revocation takes effect, never midway through an authorized write.
		adminWrite := strings.HasPrefix(r.URL.Path, "/api/admin/") && r.Method != http.MethodGet && r.Method != http.MethodHead
		exclusive := adminWrite || (r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/backup/recoveries/")) || r.URL.Path == "/api/backup/create" || r.URL.Path == "/api/backup/export" || r.URL.Path == "/api/backup/import"
		if exclusive {
			s.maintenance.Lock()
			defer s.maintenance.Unlock()
		} else {
			s.maintenance.RLock()
			defer s.maintenance.RUnlock()
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireBackupAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := domain.UserFromContext(r.Context())
		if !user.IsAdmin() {
			writeError(w, 403, "仅管理员可以备份或导入整站数据")
			return
		}
		if s.backup == nil {
			writeError(w, 503, "未配置备份目录")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleBackupExport(w http.ResponseWriter, r *http.Request) {
	path, err := s.backup.Retain(r.Context(), "manual")
	if err != nil {
		s.log.Error("备份失败", "err", err)
		writeError(w, 500, "备份失败，请检查服务器空间及数据完整性；超过 512 MB 请使用数据目录迁移")
		return
	}
	s.serveBackup(w, r, path, filepath.Base(path))
}

func (s *Server) serveBackup(w http.ResponseWriter, r *http.Request, path, name string) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		writeError(w, 404, "恢复副本不存在")
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		s.fail(w, err)
		return
	}
	if !s.recordBackupEvent(w, r, "download", "started", "服务端开始返回备份文件 "+name) {
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, name, info.ModTime(), f)
}

func (s *Server) readBackup(w http.ResponseWriter, r *http.Request) *backup.Stage {
	r.Body = http.MaxBytesReader(w, r.Body, backup.MaxArchiveBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, 400, "无法读取备份，请选择不超过 512 MB 的 ZIP 文件")
		return nil
	}
	defer r.MultipartForm.RemoveAll()
	f, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "请选择备份文件")
		return nil
	}
	defer f.Close()
	stage, err := s.backup.Inspect(r.Context(), f, header.Size)
	if err != nil {
		writeError(w, 400, "备份检查失败："+err.Error())
		return nil
	}
	return stage
}

func (s *Server) handleBackupInspect(w http.ResponseWriter, r *http.Request) {
	stage := s.readBackup(w, r)
	if stage == nil {
		return
	}
	defer stage.Close()
	writeJSON(w, 200, stage.Preview)
}

func (s *Server) handleBackupImport(w http.ResponseWriter, r *http.Request) {
	stage := s.readBackup(w, r)
	if stage == nil {
		return
	}
	defer stage.Close()
	if r.FormValue("confirmation") != "覆盖导入" || r.FormValue("sha256") != stage.SHA256 {
		writeError(w, 400, "请先检查备份并确认覆盖导入；文件变化后需重新检查")
		return
	}
	name, err := s.backup.Apply(r.Context(), stage)
	if err != nil {
		s.log.Error("导入备份失败", "err", err)
		writeError(w, 500, "导入未完成，原数据已保留。请检查服务器空间和日志后重试")
		return
	}
	http.SetCookie(w, s.sessionCookie(r, "", -1))
	writeJSON(w, 200, map[string]string{"recoveryName": name})
}

func (s *Server) handleBackupRecoveries(w http.ResponseWriter, r *http.Request) {
	items, err := s.backup.Recoveries()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, items)
}

func (s *Server) handleBackupRecoveryDownload(w http.ResponseWriter, r *http.Request) {
	path, err := s.backup.RecoveryPath(chi.URLParam(r, "name"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	s.serveBackup(w, r, path, filepath.Base(path))
}

func (s *Server) handleBackupRecoveryDelete(w http.ResponseWriter, r *http.Request) {
	path, err := s.backup.RecoveryPath(chi.URLParam(r, "name"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		writeError(w, 404, "备份不存在或已删除")
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	if !info.Mode().IsRegular() {
		writeError(w, 400, "只能删除备份文件")
		return
	}
	if !s.recordBackupEvent(w, r, "delete", "started", "开始删除备份 "+filepath.Base(path)) {
		return
	}
	if err := os.Remove(path); err != nil {
		s.fail(w, err)
		return
	}
	if !s.recordBackupEvent(w, r, "delete", "success", "已删除备份 "+filepath.Base(path)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBackupCreate(w http.ResponseWriter, r *http.Request) {
	path, err := s.backup.Retain(r.Context(), "manual")
	if err != nil {
		s.log.Error("备份失败", "err", err)
		writeError(w, 500, "备份失败，请检查服务器空间及数据完整性；超过 512 MB 请使用数据目录迁移")
		return
	}
	if !s.recordBackupEvent(w, r, "create", "success", "已创建完整备份 "+filepath.Base(path)) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": filepath.Base(path)})
}

// File operations report their actual phase; they cannot share a SQL transaction.
func (s *Server) recordBackupEvent(w http.ResponseWriter, r *http.Request, action, result, detail string) bool {
	u, _ := domain.UserFromContext(r.Context())
	err := s.repo.RecordEvent(r.Context(), domain.AuditEntry{Action: action, EntityType: "backup", Result: result, Detail: detail, Actor: u.Username, ActorID: &u.ID, ActorRoleName: u.Role})
	if err != nil {
		s.fail(w, err)
		return false
	}
	return true
}
