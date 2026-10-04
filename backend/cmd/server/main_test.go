package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestBrowserAddrUsesLoopbackForAllInterfaces(t *testing.T) {
	if got := browserAddr("0.0.0.0:8080"); got != "127.0.0.1:8080" {
		t.Fatalf("browserAddr = %q", got)
	}
	if got := browserAddr("127.0.0.1:8090"); got != "127.0.0.1:8090" {
		t.Fatalf("browserAddr = %q", got)
	}
}

// TestRecoverToLogsPanic 验证崩溃会留下现场。
//
// 这是这次加日志文件的主要目的：车间里那个黑窗口一关，屏幕上说过什么就全没了，
// 「服务忽然连不上」就永远查不出原因。所以 panic 必须连堆栈一起写进日志。
func TestRecoverToLogsPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	// 内层抛出 panic，外层吞掉它——这样测试本身不会被带崩
	func() {
		defer func() { _ = recover() }()
		func() {
			defer recoverTo(logger)
			panic("模拟的内部错误")
		}()
	}()

	got := buf.String()
	if !strings.Contains(got, "panic") {
		t.Errorf("日志里没有 panic 记录：%q", got)
	}
	if !strings.Contains(got, "模拟的内部错误") {
		t.Errorf("日志里没有 panic 的内容：%q", got)
	}
	// 堆栈才是真正能定位问题的那部分，不能只有一句「出错了」
	if !strings.Contains(got, "goroutine") || !strings.Contains(got, "recoverTo") {
		t.Errorf("日志里没有堆栈：%q", got)
	}
}

// TestRecoverToDoesNothingWithoutPanic 验证正常路径下它什么都不做。
func TestRecoverToDoesNothingWithoutPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	func() {
		defer recoverTo(logger)
		// 正常返回
	}()

	if buf.Len() != 0 {
		t.Errorf("没有 panic 时不应当写日志，实际写了：%q", buf.String())
	}
}
