package ncstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func newEncryptedStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestSaveEncryptsAtRestAndReadsPlaintext(t *testing.T) {
	store := newEncryptedStore(t)
	plain := []byte("O1234\nG00 X0 Z0\nM30\n")
	saved, err := store.Save(bytes.NewReader(plain), "O1234.nc")
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := store.Abs(saved.RelPath)
	onDisk, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(onDisk, plain) || !bytes.HasPrefix(onDisk, encryptedMagic) {
		t.Fatalf("磁盘文件没有正确加密: %q", onDisk)
	}
	got, err := store.ReadAll(saved.RelPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("解密内容不一致: %q", got)
	}
}

func TestOpenReturnsDecryptedContentForDownload(t *testing.T) {
	store := newEncryptedStore(t)
	plain := []byte("O5678\nM30\n")
	saved, err := store.Save(bytes.NewReader(plain), "O5678.nc")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.Open(saved.RelPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	got := make([]byte, len(plain))
	if _, err := reader.Read(got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("下载内容未正确解密: %q", got)
	}
}

func TestEncryptedFileWorksAfterDataBackupMovesToAnotherMachine(t *testing.T) {
	source := newEncryptedStore(t)
	plain := []byte("O7788\nG01 X10 Z-5\nM30\n")
	saved, err := source.Save(bytes.NewReader(plain), "O7788.nc")
	if err != nil {
		t.Fatal(err)
	}

	sourcePath, _ := source.Abs(saved.RelPath)
	backupRoot := t.TempDir()
	target, err := New(backupRoot, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	targetPath, _ := target.Abs(saved.RelPath)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	encrypted, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, encrypted, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := target.ReadAll(saved.RelPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("换机恢复后的 NC 内容不一致: %q", got)
	}
}

func TestReadAllKeepsLegacyPlaintextCompatible(t *testing.T) {
	store := newEncryptedStore(t)
	rel := filepath.Join("aa", "bb", "legacy.nc")
	abs, _ := store.Abs(rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	plain := []byte("LEGACY PROGRAM")
	if err := os.WriteFile(abs, plain, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadAll(filepath.ToSlash(rel))
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("无法读取历史明文内容: %q, %v", got, err)
	}
}

func TestUploadingLegacyContentReplacesPlaintextWithCiphertext(t *testing.T) {
	store := newEncryptedStore(t)
	plain := []byte("LEGACY DUPLICATE")
	hash := sha256.Sum256(plain)
	sum := hex.EncodeToString(hash[:])
	rel := filepath.Join(sum[0:2], sum[2:4], sum+".nc")
	abs, _ := store.Abs(rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, plain, 0o644); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Save(bytes.NewReader(plain), "legacy.nc")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.FromSlash(saved.RelPath) != rel {
		t.Fatalf("去重路径变化: %s", saved.RelPath)
	}
	onDisk, _ := os.ReadFile(abs)
	if bytes.Contains(onDisk, plain) || !bytes.HasPrefix(onDisk, encryptedMagic) {
		t.Fatal("重新上传后历史明文没有替换为密文")
	}
}

func TestCiphertextTamperingIsDetected(t *testing.T) {
	store := newEncryptedStore(t)
	saved, err := store.Save(bytes.NewBufferString("O9999"), "O9999.nc")
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := store.Abs(saved.RelPath)
	raw, _ := os.ReadFile(abs)
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(abs, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAll(saved.RelPath); err == nil {
		t.Fatal("被篡改的密文应当无法通过认证")
	}
}
