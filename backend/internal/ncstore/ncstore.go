// Package ncstore 负责 NC 程序文件的托管存储。
//
// 核心设计：文件按内容 sha256 寻址，同一份程序无论被多少条记录引用都只存一份。
// 三个好处：
//   1. 同一程序在多个工序复用时不占多份空间；
//   2. 天然"秒传"——内容已存在就直接建立引用，不重复落盘；
//   3. 文件内容自带校验，能发现"程序被人偷偷改过"这种要命的情况。
//
// 另一个关键点：数据库里存的是相对路径（如 a3/f9/a3f9….nc），不是绝对路径。
// 这样整个数据目录可以被整体拷走、备份、挂进容器，路径永远不会失效。
package ncstore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store 是 NC 文件库。
type Store struct {
	root     string
	maxBytes int64
}

// Saved 描述一次保存的结果。
type Saved struct {
	SHA256       string
	SizeBytes    int64
	OriginalName string
	RelPath      string
	IsNew        bool // false 表示库里已有同样内容的文件，本次是秒传

	// Encoding 只有 SaveText 会填（调用方明确知道目标编码）。
	// 从上传流保存时留空，由读取方用完整内容检测。
	Encoding string

	// Unrepresentable 是目标编码存不下、已被替换掉的字符（正常情况为空）。
	// 只有 SaveText 会填。非空时必须提示用户，否则程序里的字符被静默改写。
	Unrepresentable []string
}

// New 创建（必要时初始化）文件库。
func New(root string, maxBytes int64) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析 NC 库目录失败: %w", err)
	}
	// tmp 目录和文件库必须在同一个盘上，否则 os.Rename 会退化成"复制+删除"
	for _, d := range []string{abs, filepath.Join(abs, "tmp")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("创建 NC 库目录失败: %w", err)
		}
	}
	return &Store{root: abs, maxBytes: maxBytes}, nil
}

// Root 返回文件库根目录。
func (s *Store) Root() string { return s.root }

// Save 把 r 的内容写入文件库。
//
// 边写临时文件边算哈希，写完再按哈希决定落位，所以：
// 同一个文件重复上传不会产生第二份拷贝，不同内容也不会互相覆盖。
func (s *Store) Save(r io.Reader, originalName string) (*Saved, error) {
	tmp, err := os.CreateTemp(filepath.Join(s.root, "tmp"), "upload-*.part")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	// 无论成功失败都清理临时文件，避免磁盘里堆积垃圾
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	// 多读 1 字节即可判断是否超限，不用先把整个文件落盘再检查
	h := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, s.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("写入 NC 文件失败: %w", err)
	}
	if written > s.maxBytes {
		return nil, fmt.Errorf("%w: 文件超过 %d MB 上限", ErrTooLarge, s.maxBytes/1024/1024)
	}
	if written == 0 {
		return nil, fmt.Errorf("%w: 文件内容为空", ErrEmpty)
	}
	if err := tmp.Sync(); err != nil {
		return nil, fmt.Errorf("刷盘失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("关闭临时文件失败: %w", err)
	}

	sum := hex.EncodeToString(h.Sum(nil))
	base := sanitizeName(originalName)
	rel := filepath.Join(sum[0:2], sum[2:4], sum+filepath.Ext(base))
	abs := filepath.Join(s.root, rel)

	out := &Saved{
		SHA256:       sum,
		SizeBytes:    written,
		OriginalName: base,
		RelPath:      filepath.ToSlash(rel),
		// 编码留空：这里是流式写入，拿不到完整内容无法可靠判断
		// （GBK 的多字节字符可能正好被缓冲区切断）。
		// 真正的编码在读取时用完整内容检测，见 DetectEncoding / ReadAll。
	}

	// 同样内容已在库里：直接复用，临时文件由 defer 清掉
	if _, err := os.Stat(abs); err == nil {
		return out, nil
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	out.IsNew = true
	if err := os.Rename(tmpPath, abs); err != nil {
		return nil, fmt.Errorf("移入文件库失败: %w", err)
	}
	return out, nil
}

// Open 打开文件库中的文件。
func (s *Store) Open(relPath string) (*os.File, error) {
	abs, err := s.Abs(relPath)
	if err != nil {
		return nil, err
	}
	return os.Open(abs)
}

// Abs 把相对路径解析成绝对路径，并拦截路径穿越。
func (s *Store) Abs(relPath string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(relPath))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法的文件路径: %s", relPath)
	}
	return filepath.Join(s.root, clean), nil
}

// allowedExt 是允许的数控程序扩展名。
// 不在白名单里的扩展名会被补成 .nc，避免有人上传 exe 之类的文件进程序库。
var allowedExt = map[string]bool{
	".nc": true, ".tap": true, ".txt": true, ".prg": true, ".mpf": true,
	".spf": true, ".h": true, ".eia": true, ".iso": true, ".cnc": true,
	".gcode": true, ".min": true, ".dnc": true,
}

// sanitizeName 只保留文件名本身，去掉任何目录成分。
// 这是防止 "..\..\windows\system32\x.nc" 这类路径穿越的第一道防线。
func sanitizeName(name string) string {
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
