package datalock

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// A zero share mode excludes other opens and deletion until Close or process exit.
func Acquire(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return nil, fmt.Errorf("数据正在被服务或维护程序使用，请先停止相关进程: %w", err)
	}
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(h)
		return nil, errors.New("锁文件不能是重解析点")
	}
	return os.NewFile(uintptr(h), path), nil
}
