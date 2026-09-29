package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAndRead(t *testing.T) {
	p := filepath.Join(t.TempDir(), "logs", "server.log")
	f, err := Open(p, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write([]byte("第一行\n")); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if _, err := f.Write([]byte("第二行\n")); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读回失败: %v", err)
	}
	if got := string(raw); got != "第一行\n第二行\n" {
		t.Errorf("内容不对：%q", got)
	}
}

// TestAppendAcrossReopen 验证重开时接在原文件后面而不是清空。
//
// 这个很重要：服务重启一次就把上次的线索抹掉，那日志等于没用。
func TestAppendAcrossReopen(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.log")

	f1, err := Open(p, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	_, _ = f1.Write([]byte("上一次运行留下的\n"))
	_ = f1.Close()

	f2, err := Open(p, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("重开失败: %v", err)
	}
	_, _ = f2.Write([]byte("这一次的\n"))
	_ = f2.Close()

	raw, _ := os.ReadFile(p)
	got := string(raw)
	if !strings.Contains(got, "上一次运行留下的") || !strings.Contains(got, "这一次的") {
		t.Errorf("重开之后应当接在后面，实际：%q", got)
	}
}

// TestRotatesWhenFull 验证写满之后会滚动，而且旧内容还留着。
func TestRotatesWhenFull(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.log")
	// 上限设小一点，方便触发滚动
	f, err := Open(p, 64)
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	first := strings.Repeat("a", 40) + "\n"
	second := strings.Repeat("b", 40) + "\n"
	if _, err := f.Write([]byte(first)); err != nil {
		t.Fatalf("第一次写入失败: %v", err)
	}
	if _, err := f.Write([]byte(second)); err != nil {
		t.Fatalf("第二次写入失败: %v", err)
	}

	cur, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读当前日志失败: %v", err)
	}
	if !strings.Contains(string(cur), "bbbb") {
		t.Errorf("当前日志里应当有第二次写的内容，实际：%q", string(cur))
	}
	if strings.Contains(string(cur), "aaaa") {
		t.Errorf("第二次写入应当触发滚动，第一次的内容不该还在当前文件里")
	}

	old, err := os.ReadFile(p + ".1")
	if err != nil {
		t.Fatalf("滚动出来的 .1 不存在: %v", err)
	}
	if !strings.Contains(string(old), "aaaa") {
		t.Errorf("上一代日志里应当是第一次写的内容，实际：%q", string(old))
	}
}

// TestKeepsOnlyOneGeneration 验证只留一代，不会越滚越多。
func TestKeepsOnlyOneGeneration(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "server.log")
	f, err := Open(p, 32)
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	for i := 0; i < 10; i++ {
		if _, err := f.Write([]byte(strings.Repeat("x", 20) + "\n")); err != nil {
			t.Fatalf("第 %d 次写入失败: %v", i, err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("列目录失败: %v", err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("应当只有 server.log 和 server.log.1 两个文件，实际 %d 个：%v", len(entries), names)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.log")
	f, err := Open(p, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Errorf("第一次关闭出错: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Errorf("重复关闭不该出错: %v", err)
	}
	if _, err := f.Write([]byte("x")); err == nil {
		t.Errorf("关闭之后写入应当报错")
	}
}

func TestOpenCreatesParentDirectory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a", "b", "c", "server.log")
	f, err := Open(p, DefaultMaxBytes)
	if err != nil {
		t.Fatalf("应当自动建目录，却失败: %v", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := os.Stat(p); err != nil {
		t.Errorf("日志文件没建出来: %v", err)
	}
}
