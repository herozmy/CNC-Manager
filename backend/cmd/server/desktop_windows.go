//go:build windows

package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmCommand       = 0x0111
	wmUser          = 0x0400
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205

	nimAdd     = 0x00000000
	nimDelete  = 0x00000002
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	tpmRightButton = 0x0002

	commandOpen = 1001
	commandExit = 1003
	trayMessage = wmUser + 1
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	procRegisterClass   = user32.NewProc("RegisterClassExW")
	procCreateWindow    = user32.NewProc("CreateWindowExW")
	procDefWindowProc   = user32.NewProc("DefWindowProcW")
	procDestroyWindow   = user32.NewProc("DestroyWindow")
	procPostQuit        = user32.NewProc("PostQuitMessage")
	procPostMessage     = user32.NewProc("PostMessageW")
	procGetMessage      = user32.NewProc("GetMessageW")
	procTranslate       = user32.NewProc("TranslateMessage")
	procDispatch        = user32.NewProc("DispatchMessageW")
	procLoadIcon        = user32.NewProc("LoadIconW")
	procRegisterMessage = user32.NewProc("RegisterWindowMessageW")
	procCreateMenu      = user32.NewProc("CreatePopupMenu")
	procAppendMenu      = user32.NewProc("AppendMenuW")
	procDestroyMenu     = user32.NewProc("DestroyMenu")
	procGetCursorPos    = user32.NewProc("GetCursorPos")
	procSetForeground   = user32.NewProc("SetForegroundWindow")
	procTrackPopupMenu  = user32.NewProc("TrackPopupMenu")
	procShellNotify     = shell32.NewProc("Shell_NotifyIconW")
)

type wndClassEx struct {
	size, style            uint32
	wndProc                uintptr
	clsExtra, wndExtra     int32
	instance, icon, cursor windows.Handle
	background             windows.Handle
	menuName, className    *uint16
	iconSmall              windows.Handle
}

type message struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	point   point
	private uint32
}

type point struct{ x, y int32 }

type notifyIconData struct {
	size             uint32
	hwnd             windows.Handle
	id, flags        uint32
	callbackMessage  uint32
	icon             windows.Handle
	tip              [128]uint16
	state, stateMask uint32
	info             [256]uint16
	timeoutOrVersion uint32
	infoTitle        [64]uint16
	infoFlags        uint32
	guidItem         windows.GUID
	balloonIcon      windows.Handle
}

type windowsTray struct {
	hwnd           windows.Handle
	menu           windows.Handle
	open           func()
	exit           func()
	data           notifyIconData
	taskbarCreated uint32
}

var activeTray *windowsTray

// GUI-subsystem programs have no console on normal startup. CLI commands may
// still inherit redirected streams, or attach to the calling terminal.
func attachCLIConsole() {
	windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole").Call(0xffffffff)
	for _, stream := range []struct {
		f    **os.File
		id   uint32
		name string
	}{
		{&os.Stdin, windows.STD_INPUT_HANDLE, "stdin"},
		{&os.Stdout, windows.STD_OUTPUT_HANDLE, "stdout"},
		{&os.Stderr, windows.STD_ERROR_HANDLE, "stderr"},
	} {
		if _, err := (*stream.f).Stat(); err == nil {
			continue
		}
		if h, err := windows.GetStdHandle(stream.id); err == nil && h != 0 {
			*stream.f = os.NewFile(uintptr(h), stream.name)
		}
	}
}

func reportStartupError(err error) {
	text, _ := windows.UTF16PtrFromString(err.Error())
	title, _ := windows.UTF16PtrFromString("CNC-Manager")
	windows.MessageBox(0, text, title, windows.MB_ICONERROR)
}

func startDesktop(addr *net.TCPAddr, exit func()) (func(), error) {
	host := addr.IP.String()
	if addr.IP.IsUnspecified() {
		host = "127.0.0.1"
	}
	target, _ := windows.UTF16PtrFromString("http://" + net.JoinHostPort(host, strconv.Itoa(addr.Port)) + "/")
	open := func() {
		if err := windows.ShellExecute(0, nil, target, nil, nil, windows.SW_SHOWNORMAL); err != nil {
			reportStartupError(fmt.Errorf("打开浏览器失败，请手动访问 %s: %w", windows.UTF16PtrToString(target), err))
		}
	}
	ready := make(chan error, 1)
	done := make(chan struct{})
	go runTray(open, exit, ready, done)
	if err := <-ready; err != nil {
		return nil, err
	}
	hwnd := activeTray.hwnd
	go open()
	return func() {
		procPostMessage.Call(uintptr(hwnd), wmClose, 0, 0)
		<-done
	}, nil
}

func runTray(open, exit func(), ready chan<- error, done chan<- struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)

	className, _ := windows.UTF16PtrFromString("CNCManagerV2TrayWindow")
	title, _ := windows.UTF16PtrFromString("CNC Manager")
	wndProc := syscall.NewCallback(trayWindowProc)
	var module windows.Handle
	if err := windows.GetModuleHandleEx(windows.GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT, nil, &module); err != nil {
		ready <- err
		return
	}
	class := wndClassEx{size: uint32(unsafe.Sizeof(wndClassEx{})), instance: module, wndProc: wndProc, className: className}
	if result, _, callErr := procRegisterClass.Call(uintptr(unsafe.Pointer(&class))); result == 0 {
		ready <- fmt.Errorf("register tray window: %w", callErr)
		return
	}
	hwnd, _, callErr := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 0, 0, 0, 0, uintptr(module), 0)
	if hwnd == 0 {
		ready <- fmt.Errorf("create tray window: %w", callErr)
		return
	}
	menu, _, callErr := procCreateMenu.Call()
	if menu == 0 {
		procDestroyWindow.Call(hwnd)
		ready <- fmt.Errorf("create tray menu: %w", callErr)
		return
	}
	appendMenu(menu, mfString, commandOpen, "打开 CNC 管理系统")

	procAppendMenu.Call(menu, mfSeparator, 0, 0)
	appendMenu(menu, mfString, commandExit, "退出")

	// Resource 1 is the executable's icon group.
	icon, _, callErr := procLoadIcon.Call(uintptr(module), 1)
	if icon == 0 {
		procDestroyMenu.Call(menu)
		procDestroyWindow.Call(hwnd)
		ready <- fmt.Errorf("load application icon: %w", callErr)
		return
	}
	data := notifyIconData{size: uint32(unsafe.Sizeof(notifyIconData{})), hwnd: windows.Handle(hwnd), id: 1,
		flags: nifMessage | nifIcon | nifTip, callbackMessage: trayMessage, icon: windows.Handle(icon)}
	copy(data.tip[:], syscall.StringToUTF16("CNC 加工程序管理系统"))
	if result, _, callErr := procShellNotify.Call(nimAdd, uintptr(unsafe.Pointer(&data))); result == 0 {
		procDestroyMenu.Call(menu)
		procDestroyWindow.Call(hwnd)

		ready <- fmt.Errorf("add tray icon: %w", callErr)
		return
	}
	taskbarName, _ := windows.UTF16PtrFromString("TaskbarCreated")
	taskbarMessage, _, _ := procRegisterMessage.Call(uintptr(unsafe.Pointer(taskbarName)))
	activeTray = &windowsTray{hwnd: windows.Handle(hwnd), menu: windows.Handle(menu), open: open, exit: exit, data: data, taskbarCreated: uint32(taskbarMessage)}
	ready <- nil

	var msg message
	for {
		result, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		procTranslate.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatch.Call(uintptr(unsafe.Pointer(&msg)))
	}
	procShellNotify.Call(nimDelete, uintptr(unsafe.Pointer(&data)))
	procDestroyMenu.Call(menu)
	exit()
}

func appendMenu(menu uintptr, flags, id uintptr, label string) {
	text, _ := windows.UTF16PtrFromString(label)
	procAppendMenu.Call(menu, flags, id, uintptr(unsafe.Pointer(text)))
}

func trayWindowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	tray := activeTray
	if tray != nil {
		if tray.taskbarCreated != 0 && msg == tray.taskbarCreated {
			procShellNotify.Call(nimAdd, uintptr(unsafe.Pointer(&tray.data)))
			return 0
		}
		switch msg {
		case trayMessage:
			switch uint32(lParam) {
			case wmLButtonDblClk:
				tray.open()
			case wmRButtonUp:
				var cursor point
				procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
				procSetForeground.Call(hwnd)
				procTrackPopupMenu.Call(uintptr(tray.menu), tpmRightButton, uintptr(cursor.x), uintptr(cursor.y), 0, hwnd, 0)
			}
			return 0
		case wmCommand:
			switch uint16(wParam & 0xffff) {
			case commandOpen:
				tray.open()
			case commandExit:
				tray.exit()
			}
			return 0
		case wmDestroy:
			procPostQuit.Call(0)
			return 0
		case wmClose:
			procDestroyWindow.Call(hwnd)
			return 0
		}
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return result
}
