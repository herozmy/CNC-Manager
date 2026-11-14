// Package datalock excludes concurrent servers and offline maintenance on one data directory.
package datalock

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Acquire keeps a persistent lock file: unlinking it would let new processes lock a different inode.
func Acquire(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("数据正在被服务或维护程序使用，请先停止相关进程: %w", err)
	}
	return f, nil
}
