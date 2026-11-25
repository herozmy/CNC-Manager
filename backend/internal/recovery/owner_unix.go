//go:build unix

package recovery

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

func restoreOwner(data string, owner fs.FileInfo) error {
	stat := owner.Sys().(*syscall.Stat_t)
	var result error
	for _, name := range []string{"cnc.db", "cnc.db-wal", "cnc.db-shm", ".cnc-manager.lock"} {
		path := filepath.Join(data, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			result = errors.Join(result, err)
			continue
		}
		if !info.Mode().IsRegular() {
			result = errors.Join(result, fmt.Errorf("拒绝修改特殊文件所有者：%s", name))
			continue
		}
		current := info.Sys().(*syscall.Stat_t)
		if current.Uid != stat.Uid || current.Gid != stat.Gid {
			result = errors.Join(result, os.Chown(path, int(stat.Uid), int(stat.Gid)))
		}
	}
	return result
}
