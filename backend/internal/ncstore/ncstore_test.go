package ncstore

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testKey(b byte) []byte { return bytes.Repeat([]byte{b}, KeySize) }

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir(), 1<<20, testKey(1))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSaveEncryptsAtRestAndReadsPlaintext(t *testing.T) {
	s := newStore(t)
	plain := []byte("O1234\nG00 X0 Z0\nM30\n")
	saved, err := s.Save(bytes.NewReader(plain))
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := s.Abs(saved.RelPath)
	onDisk, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(onDisk, plain) || !bytes.HasPrefix(onDisk, encryptedMagic) {
		t.Fatalf("磁盘文件没有正确加密: %q", onDisk)
	}
	got, err := s.ReadAll(saved.RelPath)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("解密内容不一致: %q err=%v", got, err)
	}
	if !strings.HasSuffix(saved.RelPath, ".nc") || saved.Encoding != EncodingUTF8 || !saved.IsNew {
		t.Fatalf("保存结果异常: %+v", saved)
	}
}

func TestSaveDeduplicatesByContent(t *testing.T) {
	s := newStore(t)
	first, err := s.Save(strings.NewReader("O1\nM30\n"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Save(strings.NewReader("O1\nM30\n"))
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 != second.SHA256 || first.RelPath != second.RelPath {
		t.Fatal("相同内容应得到相同哈希与路径")
	}
	if second.IsNew {
		t.Fatal("第二次应为秒传")
	}
}

func TestSaveRejectsEmptyAndOversized(t *testing.T) {
	s, err := New(t.TempDir(), 8, testKey(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(strings.NewReader("")); !errors.Is(err, ErrEmpty) {
		t.Fatalf("空文件应返回 ErrEmpty，得到 %v", err)
	}
	if _, err := s.Save(strings.NewReader("123456789")); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("超限应返回 ErrTooLarge，得到 %v", err)
	}
	if _, err := s.Save(strings.NewReader("12345678")); err != nil {
		t.Fatalf("刚好等于上限应允许: %v", err)
	}
}

func TestWrongKeyCannotDecrypt(t *testing.T) {
	dir := t.TempDir()
	a, _ := New(dir, 1<<20, testKey(1))
	saved, err := a.Save(strings.NewReader("O1\nM30\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := New(dir, 1<<20, testKey(2))
	if _, err := b.ReadAll(saved.RelPath); err == nil {
		t.Fatal("错误密钥不应能解密")
	}
}

func TestTamperedFileFailsAuthentication(t *testing.T) {
	s := newStore(t)
	saved, _ := s.Save(strings.NewReader("O1\nM30\n"))
	abs, _ := s.Abs(saved.RelPath)
	raw, _ := os.ReadFile(abs)
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(abs, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadAll(saved.RelPath); err == nil {
		t.Fatal("被篡改的文件应解密失败")
	}
}

func TestPlaintextFileOnDiskIsRejected(t *testing.T) {
	s := newStore(t)
	saved, _ := s.Save(strings.NewReader("O1\nM30\n"))
	abs, _ := s.Abs(saved.RelPath)
	if err := os.WriteFile(abs, []byte("O1\nM30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadAll(saved.RelPath); err == nil {
		t.Fatal("缺少文件头的内容不应被当作有效文件")
	}
}

func TestAbsBlocksPathTraversal(t *testing.T) {
	s := newStore(t)
	for _, p := range []string{"../x", "a/../../x", "..", "/etc/passwd"} {
		if _, err := s.Abs(p); err == nil {
			t.Fatalf("路径 %q 应被拒绝", p)
		}
	}
	if _, err := s.Abs("ab/cd/x.nc"); err != nil {
		t.Fatalf("正常相对路径被拒绝: %v", err)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		`..\..\win\x.nc`: "x.nc",
		"":               "program.nc",
		".hidden.nc":     "hidden.nc",
		"evil.exe":       "evil.exe.nc",
		"O1234.MPF":      "O1234.MPF",
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q，期望 %q", in, got, want)
		}
	}
	if got := SanitizeName(strings.Repeat("a", 300) + ".nc"); len(got) > 200 {
		t.Errorf("文件名未截断: %d", len(got))
	}
}

func TestNewRejectsBadKeyLength(t *testing.T) {
	if _, err := New(t.TempDir(), 1, []byte("short")); err == nil {
		t.Fatal("短密钥应被拒绝")
	}
}

func TestLoadOrCreateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "nc.key")
	k1, err := LoadOrCreateKey(path)
	if err != nil || len(k1) != KeySize {
		t.Fatalf("首次生成失败: %v len=%d", err, len(k1))
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("密钥文件权限应为 0600: %v %v", info, err)
	}
	k2, err := LoadOrCreateKey(path)
	if err != nil || !bytes.Equal(k1, k2) {
		t.Fatal("再次加载应得到同一密钥")
	}
	if err := os.WriteFile(path, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateKey(path); err == nil {
		t.Fatal("长度错误的密钥文件应报错")
	}
}

func TestSaveTextGBKRoundTripAndWarnsUnrepresentable(t *testing.T) {
	s := newStore(t)
	saved, err := s.SaveText("(精车外圆 Ø60)\nM30\n", EncodingGBK)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Encoding != EncodingGBK {
		t.Fatalf("编码 = %q", saved.Encoding)
	}
	if len(saved.Unrepresentable) != 1 || saved.Unrepresentable[0] != "Ø" {
		t.Fatalf("应警告 Ø 无法用 GBK 表示: %v", saved.Unrepresentable)
	}
	raw, _ := s.ReadAll(saved.RelPath)
	text, err := DecodeToUTF8(raw, EncodingGBK)
	if err != nil || !strings.Contains(text, "精车外圆") {
		t.Fatalf("GBK 往返失败: %q err=%v", text, err)
	}
}

func TestDetectEncoding(t *testing.T) {
	if DetectEncoding([]byte("O1 G00")) != EncodingUTF8 {
		t.Error("纯 ASCII 应判为 UTF-8")
	}
	if DetectEncoding([]byte{0xbe, 0xab, 0xb3, 0xb5}) != EncodingGBK {
		t.Error("GBK 字节应判为 gbk")
	}
}

func TestSaveDetectsGBKUpload(t *testing.T) {
	s := newStore(t)
	saved, err := s.Save(bytes.NewReader([]byte{'(', 0xbe, 0xab, 0xb3, 0xb5, ')', '\n'}))
	if err != nil || saved.Encoding != EncodingGBK {
		t.Fatalf("上传 GBK 文件应检测为 gbk: %+v err=%v", saved, err)
	}
}

func TestDecryptBytes(t *testing.T) {
	s := newStore(t)
	saved, _ := s.Save(strings.NewReader("O1\nM30\n"))
	abs, _ := s.Abs(saved.RelPath)
	raw, _ := os.ReadFile(abs)
	if !IsEncrypted(raw) || IsEncrypted([]byte("O1\nM30\n")) {
		t.Fatal("IsEncrypted 判断错误")
	}
	plain, err := DecryptBytes(testKey(1), raw)
	if err != nil || string(plain) != "O1\nM30\n" {
		t.Fatalf("解密结果 %q err=%v", plain, err)
	}
	if _, err := DecryptBytes(testKey(9), raw); err == nil {
		t.Fatal("错误密钥应失败")
	}
	if _, err := DecryptBytes([]byte("x"), raw); err == nil {
		t.Fatal("错误长度密钥应失败")
	}
}
