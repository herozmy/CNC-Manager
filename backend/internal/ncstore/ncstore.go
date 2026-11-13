// Package ncstore 负责 NC 程序文件的托管存储。
//
// 文件按明文 sha256 寻址，同一内容全局只存一份；落盘前用 AES-256-GCM 加密。
// 数据库只存相对路径（如 a3/f9/a3f9….nc），整个数据目录可整体拷走或挂进容器。
// 文件名属于版本而不属于内容，所以落盘路径不含原始文件名。
package ncstore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// KeySize 是 AES-256 密钥长度。
const KeySize = 32

var encryptedMagic = []byte("CNCCNC01")

// Store 是 NC 文件库。
type Store struct {
	root     string
	maxBytes int64
	aead     cipher.AEAD
}

// Saved 描述一次保存的结果。
type Saved struct {
	SHA256    string
	SizeBytes int64
	RelPath   string
	Encoding  string // utf-8 或 gbk
	IsNew     bool   // false 表示库里已有同样内容，本次是秒传

	// Unrepresentable 是目标编码存不下、已被替换的字符，只有 SaveText 会填。
	// 非空时必须提示用户，否则程序里的字符会被静默改写。
	Unrepresentable []string
}

// New 创建（必要时初始化）文件库。key 必须是 KeySize 字节。
func New(root string, maxBytes int64, key []byte) (*Store, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("NC 文件密钥长度必须为 %d 字节", KeySize)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析 NC 库目录失败: %w", err)
	}
	// tmp 与文件库必须同盘，否则 os.Rename 会退化成复制加删除。
	for _, d := range []string{abs, filepath.Join(abs, "tmp")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("创建 NC 库目录失败: %w", err)
		}
	}
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return &Store{root: abs, maxBytes: maxBytes, aead: aead}, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("初始化 NC 文件加密失败: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("初始化 NC 文件加密失败: %w", err)
	}
	return aead, nil
}

// IsEncrypted 判断文件内容是否带有加密文件头。
func IsEncrypted(raw []byte) bool { return bytes.HasPrefix(raw, encryptedMagic) }

// DecryptBytes 用指定密钥解密一份加密文件内容，供旧库迁移使用。
func DecryptBytes(key, raw []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return decryptWith(aead, raw)
}

// LoadOrCreateKey 读取密钥文件；不存在时生成随机密钥并以 0600 权限保存。
// 密钥丢失意味着全部 NC 文件无法解密，备份时必须与数据一并保留。
func LoadOrCreateKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != KeySize {
			return nil, fmt.Errorf("密钥文件 %s 长度应为 %d 字节，实际 %d", path, KeySize, len(key))
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取密钥文件失败: %w", err)
	}
	key = make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建密钥目录失败: %w", err)
	}
	// O_EXCL 防止两个进程同时首次启动时互相覆盖对方的密钥。
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return LoadOrCreateKey(path)
		}
		return nil, fmt.Errorf("创建密钥文件失败: %w", err)
	}
	if _, err := f.Write(key); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("写入密钥文件失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("关闭密钥文件失败: %w", err)
	}
	return key, nil
}

// Root 返回文件库根目录。
func (s *Store) Root() string { return s.root }

// Save 把 r 的内容加密写入文件库；同样内容已存在时直接复用。
func (s *Store) Save(r io.Reader) (*Saved, error) {
	plain, err := io.ReadAll(io.LimitReader(r, s.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取 NC 文件失败: %w", err)
	}
	size := int64(len(plain))
	if size > s.maxBytes {
		return nil, fmt.Errorf("%w: 文件超过 %d MB 上限", ErrTooLarge, s.maxBytes/1024/1024)
	}
	if size == 0 {
		return nil, fmt.Errorf("%w: 文件内容为空", ErrEmpty)
	}

	h := sha256.Sum256(plain)
	sum := hex.EncodeToString(h[:])
	rel := filepath.Join(sum[0:2], sum[2:4], sum+".nc")
	abs := filepath.Join(s.root, rel)
	out := &Saved{
		SHA256:    sum,
		SizeBytes: size,
		RelPath:   filepath.ToSlash(rel),
		Encoding:  DetectEncoding(plain),
	}

	if _, err := os.Stat(abs); err == nil {
		return out, nil
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("生成随机数失败: %w", err)
	}
	var buf bytes.Buffer
	buf.Write(encryptedMagic)
	buf.Write(nonce)
	buf.Write(s.aead.Seal(nil, nonce, plain, nil))

	tmp, err := os.CreateTemp(filepath.Join(s.root, "tmp"), "upload-*.part")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath) // rename 成功后此处无文件，忽略错误
	}()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		return nil, fmt.Errorf("写入 NC 文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return nil, fmt.Errorf("刷盘失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	if err := os.Rename(tmpPath, abs); err != nil {
		return nil, fmt.Errorf("移入文件库失败: %w", err)
	}
	out.IsNew = true
	return out, nil
}

// Open 打开文件库中的文件（已解密），可用于 http.ServeContent。
func (s *Store) Open(relPath string) (*readSeekCloser, error) {
	raw, err := s.ReadAll(relPath)
	if err != nil {
		return nil, err
	}
	return &readSeekCloser{Reader: bytes.NewReader(raw)}, nil
}

type readSeekCloser struct{ *bytes.Reader }

func (r *readSeekCloser) Close() error { return nil }

func (s *Store) decrypt(raw []byte) ([]byte, error) { return decryptWith(s.aead, raw) }

func decryptWith(aead cipher.AEAD, raw []byte) ([]byte, error) {
	offset := len(encryptedMagic)
	if !bytes.HasPrefix(raw, encryptedMagic) {
		return nil, fmt.Errorf("NC 文件格式无效：缺少加密文件头")
	}
	if len(raw) < offset+aead.NonceSize()+aead.Overhead() {
		return nil, fmt.Errorf("NC 加密文件已损坏")
	}
	nonce := raw[offset : offset+aead.NonceSize()]
	plain, err := aead.Open(nil, nonce, raw[offset+aead.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("NC 文件解密失败，密钥错误或文件已被篡改: %w", err)
	}
	return plain, nil
}

// Abs 把相对路径解析成绝对路径，并拦截路径穿越。
func (s *Store) Abs(relPath string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(relPath))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法的文件路径: %s", relPath)
	}
	return filepath.Join(s.root, clean), nil
}

// SanitizeName 只保留文件名本身并限制扩展名，用于保存到版本上的文件名。
// 防止 "..\..\x.nc" 这类路径穿越；不在白名单里的扩展名会补成 .nc。
func SanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimLeft(name, ".")
	if name == "" {
		return "program.nc"
	}
	if !allowedExt[strings.ToLower(filepath.Ext(name))] {
		name += ".nc"
	}
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

var allowedExt = map[string]bool{
	".nc": true, ".tap": true, ".txt": true, ".prg": true, ".mpf": true,
	".spf": true, ".h": true, ".eia": true, ".iso": true, ".cnc": true,
	".gcode": true, ".min": true, ".dnc": true,
}
