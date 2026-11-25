// Package backup implements portable, bounded archives without replacing an open database.
package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"cnc-manager/internal/config"
	"cnc-manager/internal/migratev1"
	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/repo"
	"cnc-manager/internal/store"
)

const MaxArchiveBytes int64 = 512 << 20
const maxExpandedBytes uint64 = 2 << 30
const Format = "cnc-manager-v2/1"
const LegacyFormat = "cnc-manager-v1/1"

var blobName = regexp.MustCompile(`^data/nc/[a-f0-9]{2}/[a-f0-9]{2}/[a-f0-9]{64}\.nc$`)

// v1 stores the uploaded extension verbatim, unlike v2's fixed .nc suffix.
// Its filename length limit can also remove the extension from long names.
var legacyBlobName = regexp.MustCompile(`^data/nc/[a-f0-9]{2}/[a-f0-9]{2}/[a-f0-9]{64}(?:\.(?i:nc|tap|txt|prg|mpf|spf|h|eia|iso|cnc|gcode|min|dnc))?$`)
var recoveryName = regexp.MustCompile(`^(?:pre-import|manual|scheduled)-[0-9]{8}-[0-9]{6}-[0-9]+\.zip$`)

type Manifest struct {
	Format    string `json:"format"`
	Version   string `json:"version"`
	CreatedAt string `json:"createdAt"`
}

type Preview struct {
	Manifest
	Summary   repo.BackupSummary `json:"summary"`
	SHA256    string             `json:"sha256"`
	Migration *LegacyMigration   `json:"migration,omitempty"`
}

type LegacyMigration struct {
	SkippedSessions int `json:"skippedSessions"`
	OrphanFiles     int `json:"orphanFiles"`
	SkippedSettings int `json:"skippedSettings"`
}

type Recovery struct {
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	SizeBytes int64  `json:"sizeBytes"`
}

type Manager struct {
	Repo    *repo.Repo
	Files   *ncstore.Store
	Layout  config.Layout
	Version string
}

// All Manager operations must be called under the HTTP maintenance lock.
func (m *Manager) Create(ctx context.Context, target string) error {
	work, err := os.MkdirTemp(m.Layout.Root, ".backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	dbPath := filepath.Join(work, "cnc.db")
	if err := m.Repo.BackupSnapshot(ctx, dbPath); err != nil {
		return err
	}
	_, blobs, err := m.Repo.CheckBackupDB(ctx, dbPath)
	if err != nil {
		return err
	}
	key, err := os.ReadFile(m.Layout.KeyPath)
	if err != nil {
		return err
	}
	if len(key) != ncstore.KeySize {
		return fmt.Errorf("加密密钥无效")
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(target)
		}
	}()
	zw := zip.NewWriter(out)
	manifest, _ := json.Marshal(Manifest{Format, m.Version, time.Now().UTC().Format(time.RFC3339)})
	for _, item := range []struct {
		name string
		data []byte
	}{{"manifest.json", manifest}, {"data/nc.key", key}} {
		w, err := zw.Create(item.name)
		if err != nil {
			return err
		}
		if _, err := w.Write(item.data); err != nil {
			return err
		}
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		return err
	}
	total := uint64(info.Size()) + uint64(len(manifest)+len(key))
	if info.Size() > MaxArchiveBytes || len(blobs) > 100000 {
		return fmt.Errorf("数据超过网页备份上限，请使用数据目录迁移")
	}
	if err := addFile(zw, "data/cnc.db", dbPath); err != nil {
		return err
	}
	for _, blob := range blobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := validBlob(blob, m.Files.MaxBytes()); err != nil {
			return err
		}
		raw, err := os.ReadFile(filepath.Join(m.Layout.NCDir, filepath.FromSlash(blob.RelPath)))
		if err != nil {
			return fmt.Errorf("NC 文件缺失: %w", err)
		}
		if _, err := verifyBlob(key, raw, blob); err != nil {
			return err
		}
		total += uint64(len(raw))
		if total > maxExpandedBytes {
			return fmt.Errorf("数据超过网页备份上限，请使用数据目录迁移")
		}
		w, err := zw.Create("data/nc/" + blob.RelPath)
		if err != nil {
			return err
		}
		if _, err := w.Write(raw); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	info, err = out.Stat()
	if err != nil {
		return err
	}
	if info.Size() > MaxArchiveBytes {
		return fmt.Errorf("备份超过 512 MB，请使用数据目录迁移")
	}
	if err := out.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func addFile(zw *zip.Writer, name, path string) error {
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

type Stage struct {
	Preview
	root  string
	key   []byte
	blobs []repo.BackupBlob
}

func (s *Stage) Close() { os.RemoveAll(s.root) }

// Inspect extracts only a strict allowlist into a private temporary directory.
// Size limits apply to actual bytes as well as ZIP metadata, preventing ZIP bombs.
func (m *Manager) Inspect(ctx context.Context, source io.ReaderAt, size int64) (_ *Stage, err error) {
	if size < 1 || size > MaxArchiveBytes {
		return nil, fmt.Errorf("请选择不超过 512 MB 的备份 ZIP 文件")
	}
	zr, err := zip.NewReader(source, size)
	if err != nil {
		return nil, fmt.Errorf("文件不是有效的备份 ZIP")
	}
	if len(zr.File) > 100003 {
		return nil, fmt.Errorf("备份中的文件数量过多")
	}
	root, err := os.MkdirTemp(m.Layout.Root, ".import-")
	if err != nil {
		return nil, err
	}
	s := &Stage{root: root}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	legacy := false
	for _, f := range zr.File {
		if f.Name == "data/cnccool.db" {
			legacy = true
		}
	}
	if legacy && len(zr.File) > 100002 {
		return nil, fmt.Errorf("备份中的 NC 文件数量过多")
	}
	dbName := "data/cnc.db"
	extractRoot := root
	if legacy {
		dbName = "data/cnccool.db"
		extractRoot = filepath.Join(root, "v1")
	}
	seen := map[string]bool{}
	allowedBlob := blobName
	if legacy {
		allowedBlob = legacyBlobName
	}
	var total uint64
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := f.Name
		if !f.Mode().IsRegular() {
			return nil, fmt.Errorf("备份包含非普通文件: %q", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("备份包含重复文件: %q", name)
		}
		if name != "manifest.json" && name != dbName && (legacy || name != "data/nc.key") && !allowedBlob.MatchString(name) {
			return nil, fmt.Errorf("备份包含不支持的路径: %q", name)
		}
		seen[name] = true
		limit := uint64(m.Files.MaxBytes() + 64)
		switch name {
		case "manifest.json":
			limit = 4096
		case dbName:
			limit = uint64(MaxArchiveBytes)
		case "data/nc.key":
			limit = ncstore.KeySize
		}
		if f.UncompressedSize64 > limit || f.UncompressedSize64 > maxExpandedBytes-total {
			return nil, fmt.Errorf("备份解压后超过允许大小")
		}
		total += f.UncompressedSize64
		path := filepath.Join(extractRoot, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		if err := extractFile(f, path, int64(f.UncompressedSize64)); err != nil {
			return nil, fmt.Errorf("备份文件损坏: %w", err)
		}
	}
	required := []string{"manifest.json", dbName}
	if !legacy {
		required = append(required, "data/nc.key")
	}
	for _, name := range required {
		if !seen[name] {
			return nil, fmt.Errorf("备份缺少 %s", name)
		}
	}
	raw, err := os.ReadFile(filepath.Join(extractRoot, "manifest.json"))
	if err != nil {
		return nil, err
	}
	if legacy {
		if err := m.convertLegacy(ctx, s, raw); err != nil {
			return nil, err
		}
	} else if json.Unmarshal(raw, &s.Manifest) != nil || s.Format != Format || len(s.Version) > 100 {
		return nil, fmt.Errorf("仅支持 CNC-Manager v1 或 v2 导出的整站备份")
	}
	if _, err := time.Parse(time.RFC3339, s.CreatedAt); err != nil {
		return nil, fmt.Errorf("备份时间无效")
	}
	s.key, err = os.ReadFile(filepath.Join(root, "data/nc.key"))
	if err != nil || len(s.key) != ncstore.KeySize {
		return nil, fmt.Errorf("备份加密密钥无效")
	}
	s.Summary, s.blobs, err = m.Repo.CheckBackupDB(ctx, filepath.Join(root, "data/cnc.db"))
	if err != nil {
		return nil, err
	}
	if !legacy && len(seen) != len(s.blobs)+3 {
		return nil, fmt.Errorf("NC 文件数量与数据库不匹配")
	}
	for _, blob := range s.blobs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validBlob(blob, m.Files.MaxBytes()); err != nil {
			return nil, err
		}
		if !legacy && !seen["data/nc/"+blob.RelPath] {
			return nil, fmt.Errorf("备份缺少 NC 文件")
		}
		raw, err := os.ReadFile(filepath.Join(root, "data/nc", filepath.FromSlash(blob.RelPath)))
		if err != nil {
			return nil, err
		}
		if _, err := verifyBlob(s.key, raw, blob); err != nil {
			return nil, err
		}
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(source, 0, size)); err != nil {
		return nil, err
	}
	s.SHA256 = hex.EncodeToString(h.Sum(nil))
	return s, nil
}

// Convert only in the private staging directory. Apply uses the same validated
// v2 snapshot/recovery transaction for both formats; inspection never writes live data.
func (m *Manager) convertLegacy(ctx context.Context, s *Stage, raw []byte) error {
	var manifest struct {
		Format    int    `json:"format"`
		Version   string `json:"version"`
		CreatedAt string `json:"createdAt"`
	}
	if json.Unmarshal(raw, &manifest) != nil || manifest.Format != 1 || len(manifest.Version) > 100 {
		return fmt.Errorf("v1 备份格式无效，请选择 v1 程序导出的整站 ZIP")
	}
	if _, err := time.Parse(time.RFC3339, manifest.CreatedAt); err != nil {
		return fmt.Errorf("备份时间无效")
	}
	s.Manifest = Manifest{LegacyFormat, manifest.Version, manifest.CreatedAt}
	layout := config.NewLayout(filepath.Join(s.root, "data"))
	key, err := ncstore.LoadOrCreateKey(layout.KeyPath)
	if err != nil {
		return err
	}
	files, err := ncstore.New(layout.NCDir, m.Files.MaxBytes(), key)
	if err != nil {
		return err
	}
	db, err := store.Open(layout.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}
	old := filepath.Join(s.root, "v1", "data")
	report, err := migratev1.Run(ctx, filepath.Join(old, "cnccool.db"), filepath.Join(old, "nc"), db, files)
	if err != nil {
		return fmt.Errorf("v1 数据转换失败: %w", err)
	}
	s.Migration = &LegacyMigration{report.SkippedSession, report.OrphanFiles, report.SkippedSettings}
	return nil
}

func extractFile(f *zip.File, target string, size int64) error {
	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(in, size+1))
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("解压大小不匹配")
	}
	return out.Close()
}

func validBlob(b repo.BackupBlob, max int64) error {
	if len(b.SHA256) != 64 || !blobName.MatchString("data/nc/"+b.RelPath) || b.RelPath != b.SHA256[:2]+"/"+b.SHA256[2:4]+"/"+b.SHA256+".nc" || b.SizeBytes < 1 || b.SizeBytes > max {
		return fmt.Errorf("NC 文件路径、大小无效或超过当前服务器的单文件上限")
	}
	return nil
}

func verifyBlob(key, raw []byte, b repo.BackupBlob) ([]byte, error) {
	plain, err := ncstore.DecryptBytes(key, raw)
	if err != nil {
		return nil, fmt.Errorf("NC 文件解密失败，请检查备份是否完整")
	}
	sum := sha256.Sum256(plain)
	if int64(len(plain)) != b.SizeBytes || hex.EncodeToString(sum[:]) != b.SHA256 {
		return nil, fmt.Errorf("NC 文件校验失败，备份可能已损坏")
	}
	return plain, nil
}

// Retain publishes only complete archives; temporary files never enter the list.
func (m *Manager) Retain(ctx context.Context, prefix string) (string, error) {
	dir := filepath.Join(m.Layout.Root, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	work, err := os.MkdirTemp(dir, ".pending-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	name := prefix + "-" + time.Now().Format("20060102-150405") + "-" + strings.TrimPrefix(filepath.Base(work), ".pending-") + ".zip"
	temporary := filepath.Join(work, "archive.zip")
	if err := m.Create(ctx, temporary); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	target := filepath.Join(dir, name)
	if err := os.Rename(temporary, target); err != nil {
		return "", err
	}
	return target, nil
}

// Apply creates a downloadable recovery copy first. NC contents are re-encrypted
// with the destination key and installed before the database transaction. Existing
// files are never replaced, so rollback (or a crash) leaves the old data readable.
func (m *Manager) Apply(ctx context.Context, s *Stage) (string, error) {
	path, err := m.Retain(ctx, "pre-import")
	if err != nil {
		return "", fmt.Errorf("无法备份当前数据，已取消导入: %w", err)
	}
	for _, blob := range s.blobs {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		raw, err := os.ReadFile(filepath.Join(s.root, "data/nc", filepath.FromSlash(blob.RelPath)))
		if err != nil {
			return "", err
		}
		plain, err := verifyBlob(s.key, raw, blob)
		if err != nil {
			return "", err
		}
		// Refuse to reuse a damaged destination file with the same content address.
		if existing, err := m.Files.ReadAll(blob.RelPath); err == nil {
			if !bytes.Equal(existing, plain) {
				return "", fmt.Errorf("服务器现有 NC 文件损坏，已取消导入")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if _, err := m.Files.Save(bytes.NewReader(plain)); err != nil {
			return "", err
		}
	}
	if err := m.Repo.RestoreSnapshot(ctx, filepath.Join(s.root, "data/cnc.db")); err != nil {
		return "", fmt.Errorf("导入失败，原数据已保留: %w", err)
	}
	return filepath.Base(path), nil
}

func (m *Manager) Recoveries() ([]Recovery, error) {
	items := []Recovery{}
	entries, err := os.ReadDir(filepath.Join(m.Layout.Root, "backups"))
	if os.IsNotExist(err) {
		return items, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !recoveryName.MatchString(entry.Name()) || !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		items = append(items, Recovery{entry.Name(), info.ModTime().UTC().Format(time.RFC3339), info.Size()})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].Name > items[j].Name
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	return items, nil
}

// PruneScheduled always keeps the archive just published, even when timestamps tie.
func (m *Manager) PruneScheduled(newest string, keep int) error {
	if keep < 1 {
		return fmt.Errorf("计划备份至少保留一份")
	}
	items, err := m.Recoveries()
	if err != nil {
		return err
	}
	remaining := keep - 1
	for _, item := range items {
		if item.Name == newest || !strings.HasPrefix(item.Name, "scheduled-") {
			continue
		}
		if remaining > 0 {
			remaining--
			continue
		}
		if err := os.Remove(filepath.Join(m.Layout.Root, "backups", item.Name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (m *Manager) RecoveryPath(name string) (string, error) {
	if !recoveryName.MatchString(name) || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("恢复副本名称无效")
	}
	return filepath.Join(m.Layout.Root, "backups", name), nil
}
