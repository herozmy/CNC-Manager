//go:build windows

package tray

import (
	_ "embed"
	"fmt"
	"runtime"
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

	commandOpen    = 1001
	commandLogs    = 1002
	commandExit    = 1003
	trayMessage    = wmUser + 1
	idiApplication = 32512
)

var (
	user32                     = windows.NewLazySystemDLL("user32.dll")
	shell32                    = windows.NewLazySystemDLL("shell32.dll")
	procRegisterClass          = user32.NewProc("RegisterClassExW")
	procCreateWindow           = user32.NewProc("CreateWindowExW")
	procDefWindowProc          = user32.NewProc("DefWindowProcW")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procPostQuit               = user32.NewProc("PostQuitMessage")
	procPostMessage            = user32.NewProc("PostMessageW")
	procGetMessage             = user32.NewProc("GetMessageW")
	procTranslate              = user32.NewProc("TranslateMessage")
	procDispatch               = user32.NewProc("DispatchMessageW")
	procLoadIcon               = user32.NewProc("LoadIconW")
	procCreateIconFromResource = user32.NewProc("CreateIconFromResourceEx")
	procCreateMenu             = user32.NewProc("CreatePopupMenu")
	procAppendMenu             = user32.NewProc("AppendMenuW")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procGetCursorPos           = user32.NewProc("GetCursorPos")
	procSetForeground          = user32.NewProc("SetForegroundWindow")
	procTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	procShellNotify            = shell32.NewProc("Shell_NotifyIconW")
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
	hwnd    windows.Handle
	menu    windows.Handle
	actions *guardedActions
}

var activeTray *windowsTray

//go:embed cnccool.ico
var embeddedIcon []byte

func startPlatform(actions *guardedActions) (func(), error) {
	ready := make(chan error, 1)
	done := make(chan struct{})
	go runTray(actions, ready, done)
	if err := <-ready; err != nil {
		return nil, err
	}
	return func() {
		if activeTray != nil && activeTray.hwnd != 0 {
			procPostMessage.Call(uintptr(activeTray.hwnd), wmClose, 0, 0)
		}
		<-done
	}, nil
}

func runTray(actions *guardedActions, ready chan<- error, done chan<- struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(done)

	className, _ := windows.UTF16PtrFromString("CNCCoolTrayWindow")
	title, _ := windows.UTF16PtrFromString("CNC Manager")
	wndProc := syscall.NewCallback(trayWindowProc)
	class := wndClassEx{size: uint32(unsafe.Sizeof(wndClassEx{})), wndProc: wndProc, className: className}
	if result, _, callErr := procRegisterClass.Call(uintptr(unsafe.Pointer(&class))); result == 0 {
		ready <- fmt.Errorf("register tray window: %w", callErr)
		return
	}
	hwnd, _, callErr := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 0, 0, 0, 0, 0, 0)
	if hwnd == 0 {
		ready <- fmt.Errorf("create tray window: %w", callErr)
		return
	}
	menu, _, _ := procCreateMenu.Call()
	appendMenu(menu, mfString, commandOpen, "打开 CNC 管理系统")
	appendMenu(menu, mfString, commandLogs, "打开日志目录")
	procAppendMenu.Call(menu, mfSeparator, 0, 0)
	appendMenu(menu, mfString, commandExit, "退出")

	activeTray = &windowsTray{hwnd: windows.Handle(hwnd), menu: windows.Handle(menu), actions: actions}
	icon := uintptr(0)
	if frame, frameErr := icoFrame(embeddedIcon, 32); frameErr == nil && len(frame) > 0 {
		icon, _, _ = procCreateIconFromResource.Call(
			uintptr(unsafe.Pointer(&frame[0])), uintptr(len(frame)), 1, 0x00030000, 32, 32, 0)
	}
	if icon == 0 {
		icon, _, _ = procLoadIcon.Call(0, idiApplication)
	}
	data := notifyIconData{size: uint32(unsafe.Sizeof(notifyIconData{})), hwnd: windows.Handle(hwnd), id: 1,
		flags: nifMessage | nifIcon | nifTip, callbackMessage: trayMessage, icon: windows.Handle(icon)}
	copy(data.tip[:], syscall.StringToUTF16("CNC 加工程序管理系统"))
	if result, _, callErr := procShellNotify.Call(nimAdd, uintptr(unsafe.Pointer(&data))); result == 0 {
		procDestroyMenu.Call(menu)
		procDestroyWindow.Call(hwnd)
		activeTray = nil
		ready <- fmt.Errorf("add tray icon: %w", callErr)
		return
	}
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
	activeTray = nil
}

func appendMenu(menu uintptr, flags, id uintptr, label string) {
	text, _ := windows.UTF16PtrFromString(label)
	procAppendMenu.Call(menu, flags, id, uintptr(unsafe.Pointer(text)))
}

func trayWindowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	tray := activeTray
	if tray != nil {
		switch msg {
		case trayMessage:
			switch uint32(lParam) {
			case wmLButtonDblClk:
				tray.actions.openApp()
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
				tray.actions.openApp()
			case commandLogs:
				tray.actions.openLogs()
			case commandExit:
				tray.actions.exit()
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
