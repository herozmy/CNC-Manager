package httpapi

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"cnccool/internal/store"
	"cnccool/internal/update"
	"github.com/go-chi/chi/v5"
)

const maxBackupBytes = 2 << 30

type backupManifest struct {
	Format    int    `json:"format"`
	CreatedAt string `json:"createdAt"`
	Version   string `json:"version"`
}

type backupItem struct {
	ID        int    `json:"id"`
	CreatedAt string `json:"createdAt"`
	Version   string `json:"version"`
	SizeBytes int64  `json:"sizeBytes"`
}

func (s *Server) backupsDir() string { return filepath.Join(s.cfg.DataDir, "backups") }

func (s *Server) handleDatabaseBackup(w http.ResponseWriter, r *http.Request) {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	tempDir, err := os.MkdirTemp("", "cnccool-backup-*")
	if err != nil {
		s.fail(w, err)
		return
	}
	defer os.RemoveAll(tempDir)
	dbPath := filepath.Join(tempDir, "cnccool.db")
	if _, err := s.repo.DB().ExecContext(r.Context(), `VACUUM INTO ?`, dbPath); err != nil {
		s.fail(w, fmt.Errorf("生成数据库备份失败: %w", err))
		return
	}
	if err := os.MkdirAll(s.backupsDir(), 0o755); err != nil {
		s.fail(w, err)
		return
	}
	id, err := nextBackupID(s.backupsDir())
	if err != nil {
		s.fail(w, err)
		return
	}
	archivePath := filepath.Join(s.backupsDir(), fmt.Sprintf("backup-%06d.zip", id))
	if err := createBackupArchive(archivePath, dbPath, s.cfg.NCDir, s.version); err != nil {
		s.fail(w, err)
		return
	}
	file, err := os.Open(archivePath)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer file.Close()
	info, _ := file.Stat()
	filename := fmt.Sprintf("cnccool-backup-%d-%s.zip", id, time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

func nextBackupID(dir string) (int, error) {
	items, err := listBackups(dir)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 1, nil
	}
	return items[0].ID + 1, nil
}

func listBackups(dir string) ([]backupItem, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []backupItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]backupItem, 0)
	for _, entry := range entries {
		var id int
		if entry.IsDir() || func() bool { _, e := fmt.Sscanf(entry.Name(), "backup-%06d.zip", &id); return e != nil }() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		zr, err := zip.OpenReader(path)
		if err != nil {
			continue
		}
		var manifest backupManifest
		for _, f := range zr.File {
			if f.Name == "manifest.json" {
				rc, e := f.Open()
				if e == nil {
					raw, _ := io.ReadAll(rc)
					_ = rc.Close()
					_ = json.Unmarshal(raw, &manifest)
				}
				break
			}
		}
		_ = zr.Close()
		info, _ := entry.Info()
		items = append(items, backupItem{id, manifest.CreatedAt, manifest.Version, info.Size()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	return items, nil
}

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	items, err := listBackups(s.backupsDir())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, items)
}

func backupID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		return 0, fmt.Errorf("备份编号无效")
	}
	return id, nil
}
func (s *Server) storedBackupPath(id int) string {
	return filepath.Join(s.backupsDir(), fmt.Sprintf("backup-%06d.zip", id))
}

func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	id, err := backupID(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	path := s.storedBackupPath(id)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		writeError(w, 404, "备份不存在")
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	defer file.Close()
	info, _ := file.Stat()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="cnccool-backup-%d.zip"`, id))
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}

func createBackupArchive(target, dbPath, ncDir, version string) error {
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	manifest, _ := json.Marshal(backupManifest{1, time.Now().Format(time.RFC3339), version})
	if err = writeZipBytes(zw, "manifest.json", manifest); err == nil {
		err = writeZipFile(zw, "data/cnccool.db", dbPath)
	}
	if err == nil {
		err = filepath.Walk(ncDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(ncDir, path)
			if e != nil {
				return e
			}
			return writeZipFile(zw, "data/nc/"+filepath.ToSlash(rel), path)
		})
		if os.IsNotExist(err) {
			err = nil
		}
	}
	if e := zw.Close(); err == nil {
		err = e
	}
	if e := out.Close(); err == nil {
		err = e
	}
	return err
}

func writeZipBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
func writeZipFile(zw *zip.Writer, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

func (s *Server) handleDatabaseRestore(w http.ResponseWriter, r *http.Request) {
	if s.restart == nil {
		writeError(w, 500, "服务没有配置重启动作，无法恢复备份")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBackupBytes)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeError(w, 400, "读取备份包失败："+err.Error())
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "没有收到备份包文件")
		return
	}
	defer file.Close()
	root := filepath.Join(s.cfg.DataDir, ".restore")
	_ = os.RemoveAll(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		s.fail(w, err)
		return
	}
	zipPath := filepath.Join(root, "backup.zip")
	out, err := os.Create(zipPath)
	if err != nil {
		s.fail(w, err)
		return
	}
	_, copyErr := io.Copy(out, file)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		writeError(w, 400, "保存备份包失败")
		return
	}
	if err := s.validateBackupArchive(zipPath); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	s.backupMu.Lock()
	if err := os.MkdirAll(s.backupsDir(), 0o755); err != nil {
		s.backupMu.Unlock()
		s.fail(w, err)
		return
	}
	id, err := nextBackupID(s.backupsDir())
	storedPath := s.storedBackupPath(id)
	if err == nil {
		err = copyFile(zipPath, storedPath)
	}
	s.backupMu.Unlock()
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.startBackupRestore(r, storedPath); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"restarting": true})
	s.restart()
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (s *Server) validateBackupArchive(path string) error {
	root, err := os.MkdirTemp(s.cfg.DataDir, ".validate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	if err := update.Extract(path, root); err != nil {
		return err
	}
	return verifyBackupStage(root)
}

func (s *Server) handleStoredBackupRestore(w http.ResponseWriter, r *http.Request) {
	id, err := backupID(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	path := s.storedBackupPath(id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		writeError(w, 404, "备份不存在")
		return
	}
	if err := s.startBackupRestore(r, path); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"restarting": true})
	s.restart()
}

func (s *Server) startBackupRestore(r *http.Request, zipPath string) error {
	if s.restart == nil {
		return fmt.Errorf("服务没有配置重启动作，无法恢复备份")
	}
	workRoot, err := os.MkdirTemp(s.cfg.DataDir, ".restore-")
	if err != nil {
		return err
	}
	stage := filepath.Join(workRoot, "stage")
	if err := update.Extract(zipPath, stage); err != nil {
		return err
	}
	if err := verifyBackupStage(stage); err != nil {
		return err
	}
	rollbackDB := filepath.Join(workRoot, "rollback.db")
	if _, err := s.repo.DB().ExecContext(r.Context(), `VACUUM INTO ?`, rollbackDB); err != nil {
		return fmt.Errorf("创建恢复前安全快照失败: %w", err)
	}
	scriptPath := filepath.Join(workRoot, "apply-restore.cmd")
	script := restoreScript(s.cfg.DBPath, s.cfg.NCDir, stage, rollbackDB, s.layout.ExePath, s.cfg.Addr)
	if err := os.WriteFile(scriptPath, []byte(strings.ReplaceAll(script, "\n", "\r\n")), 0o644); err != nil {
		return err
	}
	return update.SpawnDetached(scriptPath)
}

func verifyBackupStage(stage string) error {
	raw, err := os.ReadFile(filepath.Join(stage, "manifest.json"))
	if err != nil {
		return fmt.Errorf("备份包缺少 manifest.json")
	}
	var manifest backupManifest
	if json.Unmarshal(raw, &manifest) != nil || manifest.Format != 1 {
		return fmt.Errorf("备份包格式不受支持")
	}
	info, err := os.Stat(filepath.Join(stage, "data", "cnccool.db"))
	if err != nil || info.IsDir() || info.Size() == 0 {
		return fmt.Errorf("备份包缺少有效的 data/cnccool.db")
	}
	db, err := store.Open(filepath.Join(stage, "data", "cnccool.db"))
	if err != nil {
		return fmt.Errorf("备份数据库无法打开: %w", err)
	}
	defer db.Close()
	var integrity string
	if err := db.Get(&integrity, `PRAGMA integrity_check`); err != nil || integrity != "ok" {
		return fmt.Errorf("备份数据库完整性检查失败")
	}
	var tableCount int
	if err := db.Get(&tableCount, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('drawing','operation','nc_program','nc_version')`); err != nil || tableCount != 4 {
		return fmt.Errorf("备份数据库不是 CNC-Manager 数据库")
	}
	return nil
}

func restoreScript(dbPath, ncDir, stage, rollbackDB, exePath, addr string) string {
	q := func(v string) string { return strings.ReplaceAll(v, "%", "%%") }
	return fmt.Sprintf("@echo off\nsetlocal\nset \"DB=%s\"\nset \"NC=%s\"\nset \"STAGE=%s\"\nset \"ROLLBACK_DB=%s\"\nset /a WAITED=0\n:retry\nmove /y \"%%STAGE%%\\data\\cnccool.db\" \"%%DB%%\" >nul 2>&1\nif not errorlevel 1 goto swap_nc\nset /a WAITED+=1\nif %%WAITED%% GEQ 120 goto done\nping -n 2 127.0.0.1 >nul 2>&1\ngoto retry\n:swap_nc\nif exist \"%%NC%%.old\" rd /s /q \"%%NC%%.old\"\nif exist \"%%NC%%\" move \"%%NC%%\" \"%%NC%%.old\" >nul 2>&1\nif exist \"%%STAGE%%\\data\\nc\" move \"%%STAGE%%\\data\\nc\" \"%%NC%%\" >nul 2>&1\nif errorlevel 1 goto restore_old\nif not exist \"%%NC%%\" mkdir \"%%NC%%\"\nif exist \"%%NC%%.old\" rd /s /q \"%%NC%%.old\"\ndel /q \"%%ROLLBACK_DB%%\" >nul 2>&1\ngoto restart\n:restore_old\nmove /y \"%%ROLLBACK_DB%%\" \"%%DB%%\" >nul 2>&1\nif exist \"%%NC%%\" rd /s /q \"%%NC%%\"\nif exist \"%%NC%%.old\" move \"%%NC%%.old\" \"%%NC%%\" >nul 2>&1\n:restart\nset \"CNC_NO_BROWSER=1\"\nset \"CNC_ADDR=%s\"\nstart \"\" \"%s\"\n:done\nendlocal\n", q(dbPath), q(ncDir), q(stage), q(rollbackDB), q(addr), q(exePath))
}
