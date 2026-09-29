// Package applog 提供一个按大小滚动的日志文件写入器。
//
// 为什么需要它：这个程序是给车间用的，用户双击 start.cmd 之后就是一个黑窗口。
// 窗口一关、或者服务自己没了，屏幕上说过什么就全没了——
// 「服务忽然连不上」这种事就永远是悬案。所以日志必须落到文件里。
//
// 只做最简单的那种滚动：超过上限就把当前文件改名成 .1（覆盖上一代的 .1），
// 再开一个空的。不压缩、不保留多代——现场要的是「最近一次发生了什么」，一代足够。
package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DefaultMaxBytes 是单个日志文件的大小上限。
//
// 纯文本、每行一个请求，4MB 能装下几万行，够回溯很久了；
// 再大也只是占地方。
const DefaultMaxBytes = 4 << 20

// RotatingFile 是实现了 io.Writer 的滚动日志文件。
//
// 可以安全地被多个 goroutine 同时写（HTTP 服务是多协程的）。
type RotatingFile struct {
	path     string
	maxBytes int64

	mu   sync.Mutex
	file *os.File
	size int64
}

// Open 打开（或创建）日志文件。父目录不存在会自动建。
func Open(path string, maxBytes int64) (*RotatingFile, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}

	var size int64
	if info, err := f.Stat(); err == nil {
		size = info.Size()
	}

	return &RotatingFile{path: path, maxBytes: maxBytes, file: f, size: size}, nil
}

// Path 返回当前日志文件的路径。
func (r *RotatingFile) Path() string { return r.path }

// Write 写入一行（或一段）日志，必要时先滚动。
//
// 滚动失败不返回错误：写日志是辅助功能，不能因为改名失败就把日志整个丢掉，
// 更不能把服务带崩。这时候继续往原文件写就是了。
func (r *RotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		return 0, os.ErrClosed
	}
	if r.size > 0 && r.size+int64(len(p)) > r.maxBytes {
		_ = r.rotateLocked()
	}

	n, err := r.file.Write(p)
	r.size += int64(n)
	return n, err
}

// Close 关闭文件。重复调用是安全的。
func (r *RotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}

// rotateLocked 把当前文件挪成 .1，再开一个新的。调用方必须已经持锁。
func (r *RotatingFile) rotateLocked() error {
	if err := r.file.Close(); err != nil {
		// 关不掉也继续：下面会重新打开，最坏情况是原文件被继续追加
		r.file = nil
	}

	rotated := r.path + ".1"
	// 先删掉上一代，否则 Windows 上 Rename 会因为目标已存在而失败
	_ = os.Remove(rotated)
	if err := os.Rename(r.path, rotated); err != nil {
		// 改名失败（多半是文件被杀毒软件占着）：重新打开原文件继续追加，
		// 总比把日志丢了强
		f, openErr := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if openErr != nil {
			return openErr
		}
		r.file = f
		return err
	}

	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	r.file = f
	r.size = 0
	return nil
}
