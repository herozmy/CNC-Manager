//go:build windows

package update

import (
	"os/exec"
	"syscall"
)

// SpawnDetached 起一个和当前进程完全脱钩的替换脚本。
//
// 必须脱钩：服务马上就要退出，脚本要是还留在同一个控制台里，
// 控制台一关它就被一起带走了，更新做到一半断在那儿，那是最糟的结果。
func SpawnDetached(scriptPath string) error {
	cmd := exec.Command("cmd.exe", "/c", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// DETACHED_PROCESS：不要控制台，也就不受父进程控制台关闭的影响
		// CREATE_NEW_PROCESS_GROUP：不跟着父进程吃 Ctrl+C
		CreationFlags: 0x00000008 | 0x00000200,
	}
	return cmd.Start()
}
