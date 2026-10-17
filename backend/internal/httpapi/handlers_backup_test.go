package httpapi

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cnccool/internal/config"
	"cnccool/internal/domain"
	"cnccool/internal/repo"
	"cnccool/internal/store"
)

func TestDatabaseBackupRequiresAuthenticationAndReturnsConsistentDatabase(t *testing.T) {
	temp := t.TempDir()
	db, err := store.Open(filepath.Join(temp, "source.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("迁移测试数据库失败: %v", err)
	}

	repository := repo.New(db)
	server := NewServer(repository, nil, &config.Config{DataDir: temp, NCDir: filepath.Join(temp, "nc")}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	handler := server.Router()

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/backup/database", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("未登录备份状态码 = %d，期望 401", unauthorized.Code)
	}

	user, err := repository.CreateFirstUser(context.Background(), "admin", "管理员", "secure123")
	if err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	token, _, err := repository.CreateSession(context.Background(), user.ID, sessionLifetime)
	if err != nil {
		t.Fatalf("创建测试会话失败: %v", err)
	}
	if _, err := repository.CreateDrawing(domain.WithUser(context.Background(), user), domain.DrawingInput{
		DrawingNo: "BACKUP-001", Name: "迁移验证",
	}); err != nil {
		t.Fatalf("写入待备份数据失败: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/backup/database", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: token})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("备份状态码 = %d，响应 = %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/zip" {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if disposition := recorder.Header().Get("Content-Disposition"); !strings.Contains(disposition, "cnccool-backup-") || !strings.Contains(disposition, ".zip") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}

	backupPath := filepath.Join(temp, "downloaded.zip")
	if err := os.WriteFile(backupPath, recorder.Body.Bytes(), 0o600); err != nil {
		t.Fatalf("保存下载结果失败: %v", err)
	}
	zr, err := zip.OpenReader(backupPath)
	if err != nil {
		t.Fatalf("下载结果不是 ZIP: %v", err)
	}
	defer zr.Close()
	stage := filepath.Join(temp, "stage")
	for _, f := range zr.File {
		target := filepath.Join(stage, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		in, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(target)
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(out, in)
		_ = in.Close()
		_ = out.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := verifyBackupStage(stage); err != nil {
		t.Fatalf("备份包校验失败: %v", err)
	}
	backupDB, err := store.Open(filepath.Join(stage, "data", "cnccool.db"))
	if err != nil {
		t.Fatalf("下载结果不是可打开的 SQLite 数据库: %v", err)
	}
	defer func() { _ = backupDB.Close() }()
	var count int
	if err := backupDB.Get(&count, `SELECT COUNT(*) FROM drawing WHERE drawing_no = 'BACKUP-001'`); err != nil {
		t.Fatalf("读取备份数据库失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("备份数据库中的迁移验证图纸数量 = %d，期望 1", count)
	}
}

func TestVerifyBackupStageRejectsIncompletePackage(t *testing.T) {
	stage := t.TempDir()
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), []byte(`{"format":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyBackupStage(stage); err == nil || !strings.Contains(err.Error(), "cnccool.db") {
		t.Fatalf("应拒绝缺数据库的备份包，实际: %v", err)
	}
}

func TestRestoreScriptKeepsRollbackPathsAndRestarts(t *testing.T) {
	script := restoreScript(`D:\app\data\cnccool.db`, `D:\app\data\nc`, `D:\app\data\.restore\stage`, `D:\app\data\.restore\rollback.db`, `D:\app\CNC-Manager.exe`, `127.0.0.1:8080`)
	for _, expected := range []string{`set "ROLLBACK_DB=D:\app\data\.restore\rollback.db"`, `move /y "%ROLLBACK_DB%" "%DB%"`, `move "%NC%.old" "%NC%"`, `start "" "D:\app\CNC-Manager.exe"`} {
		if !strings.Contains(script, expected) {
			t.Errorf("恢复脚本缺少 %q", expected)
		}
	}
}

func TestBackupHistoryUsesIncreasingNumbers(t *testing.T) {
	dir := t.TempDir()
	if id, err := nextBackupID(dir); err != nil || id != 1 {
		t.Fatalf("首个备份编号 = %d, err=%v", id, err)
	}
	for _, id := range []int{1, 3, 2} {
		path := filepath.Join(dir, fmt.Sprintf("backup-%06d.zip", id))
		out, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(out)
		manifest, _ := json.Marshal(backupManifest{Format: 1, CreatedAt: fmt.Sprintf("2026-10-%02dT00:00:00+08:00", id), Version: "v1.2.7"})
		if err := writeZipBytes(zw, "manifest.json", manifest); err != nil {
			t.Fatal(err)
		}
		_ = zw.Close()
		_ = out.Close()
	}
	items, err := listBackups(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].ID != 3 || items[2].ID != 1 {
		t.Fatalf("备份历史顺序不正确: %#v", items)
	}
	if id, err := nextBackupID(dir); err != nil || id != 4 {
		t.Fatalf("下一个备份编号 = %d, err=%v", id, err)
	}
}
