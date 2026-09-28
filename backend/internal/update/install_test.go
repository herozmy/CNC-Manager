package update

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeZip 按 name->content 造一个 zip，返回文件路径。
func makeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "pkg.zip")

	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("创建 zip 失败: %v", err)
	}
	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("写入 zip 条目失败: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("写入 zip 内容失败: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 zip 失败: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("关闭文件失败: %v", err)
	}
	return p
}

// validPackage 是一份结构正确的免安装包。
func validPackage(version string) map[string]string {
	return map[string]string{
		exeName:                 "MZ fake exe",
		versionRel:              version,
		webIndexRel:             "<!doctype html><div id=app></div>",
		"web/assets/index-a.js": "console.log(1)",
		"start.cmd":             "@echo off",
		"README.txt":            "readme",
	}
}

// TestExtractValidPackage 先确认正常包能顺利解压，后面几条测试才有意义。
func TestExtractValidPackage(t *testing.T) {
	zipPath := makeZip(t, validPackage("v0.04"))
	dest := filepath.Join(t.TempDir(), "stage")

	if err := Extract(zipPath, dest); err != nil {
		t.Fatalf("解压正常包失败: %v", err)
	}
	if !fileExists(filepath.Join(dest, exeName)) {
		t.Errorf("解压后没找到 %s", exeName)
	}
	if !fileExists(filepath.Join(dest, filepath.FromSlash(webIndexRel))) {
		t.Errorf("解压后没找到 %s", webIndexRel)
	}

	root, err := FindPackageRoot(dest)
	if err != nil || root != dest {
		t.Errorf("FindPackageRoot = %q, %v；期望就是 dest 本身", root, err)
	}
	v, err := Verify(root)
	if err != nil || v != "v0.04" {
		t.Errorf("Verify = %q, %v；期望 v0.04", v, err)
	}
}

// TestExtractBlocksZipSlip 是这一组里最关键的一条。
//
// 恶意（或者只是构造得不对）的压缩包可以用 ../ 把文件写到安装目录外面，
// 覆盖系统文件或者启动项。必须一条都出不去。
func TestExtractBlocksZipSlip(t *testing.T) {
	cases := []map[string]string{
		{"../escaped.txt": "pwned"},
		{"../../escaped.txt": "pwned"},
		{"web/../../escaped.txt": "pwned"},
		{"/absolute.txt": "pwned"},
		{"web/../../../escape/deep.txt": "pwned"},
	}

	for i, files := range cases {
		base := t.TempDir()
		dest := filepath.Join(base, "inner", "stage")
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatalf("准备目录失败: %v", err)
		}

		zipPath := makeZip(t, files)
		err := Extract(zipPath, dest)
		if err == nil {
			// 不报错也不一定有事：path.Clean 会把 .. 抹掉，文件可能落在 dest 里。
			// 真正要断言的是「没有任何东西跑到 dest 外面」。
			t.Logf("用例 %d 未报错，改为检查是否越界写出", i)
		}

		// 无论报不报错，外层目录里都不能冒出文件来
		escaped, _ := filepath.Glob(filepath.Join(base, "escaped.txt"))
		escaped2, _ := filepath.Glob(filepath.Join(base, "absolute.txt"))
		escaped3, _ := filepath.Glob(filepath.Join(base, "escape", "*"))
		if len(escaped)+len(escaped2)+len(escaped3) > 0 {
			t.Errorf("用例 %d：有文件被写到了暂存目录外面，存在 zip-slip", i)
		}
	}
}

// TestExtractRejectsSymlink 验证符号链接被拒绝。
//
// 符号链接是绕过目录限制的另一种办法：先创建指向系统目录的链接，
// 后续条目再往里写。离线安装包不需要链接，直接拒绝最省事。
func TestExtractRejectsSymlink(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pkg.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("创建 zip 失败: %v", err)
	}
	zw := zip.NewWriter(f)

	hdr := &zip.FileHeader{Name: "link"}
	hdr.SetMode(os.ModeSymlink | 0o777)
	if _, err := zw.CreateHeader(hdr); err != nil {
		t.Fatalf("写入链接条目失败: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 zip 失败: %v", err)
	}
	_ = f.Close()

	err = Extract(p, filepath.Join(t.TempDir(), "stage"))
	if err == nil {
		t.Fatalf("含有符号链接的包必须被拒绝")
	}
	if !strings.Contains(err.Error(), "符号链接") {
		t.Errorf("错误信息没说清原因：%v", err)
	}
}

func TestExtractRejectsGarbage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "not.zip")
	if err := os.WriteFile(p, []byte("this is not a zip"), 0o644); err != nil {
		t.Fatalf("准备文件失败: %v", err)
	}
	if err := Extract(p, filepath.Join(t.TempDir(), "stage")); err == nil {
		t.Errorf("不是 zip 的文件必须被拒绝")
	}
}

// TestFindPackageRootHandlesWrapperDir 验证有人自己解压再压回去的情况。
func TestFindPackageRootHandlesWrapperDir(t *testing.T) {
	// 包里多套一层文件夹
	files := map[string]string{}
	for k, v := range validPackage("v0.04") {
		files["cnccool-v0.04-windows-amd64/"+k] = v
	}
	zipPath := makeZip(t, files)
	dest := filepath.Join(t.TempDir(), "stage")
	if err := Extract(zipPath, dest); err != nil {
		t.Fatalf("解压失败: %v", err)
	}

	root, err := FindPackageRoot(dest)
	if err != nil {
		t.Fatalf("应该认得出多套一层的包: %v", err)
	}
	if root == dest {
		t.Fatalf("应当找到里面那一层目录")
	}
	if v, err := Verify(root); err != nil || v != "v0.04" {
		t.Errorf("Verify(%q) = %q, %v", root, v, err)
	}

	// 摊平之后，脚本要用的路径就在 stage 根上了
	if err := Flatten(root, dest); err != nil {
		t.Fatalf("摊平失败: %v", err)
	}
	if !fileExists(filepath.Join(dest, exeName)) || !fileExists(filepath.Join(dest, filepath.FromSlash(webIndexRel))) {
		t.Errorf("摊平之后 exe 和 web/index.html 都应当直接在 stage 根下")
	}
}

func TestFindPackageRootRejectsBadPackages(t *testing.T) {
	t.Run("没有 exe", func(t *testing.T) {
		zipPath := makeZip(t, map[string]string{"web/index.html": "x"})
		dest := filepath.Join(t.TempDir(), "stage")
		_ = Extract(zipPath, dest)
		if _, err := FindPackageRoot(dest); err == nil {
			t.Errorf("没有 exe 的包必须被拒绝")
		}
	})

	t.Run("有多个 exe", func(t *testing.T) {
		zipPath := makeZip(t, map[string]string{
			"a/" + exeName: "x",
			"b/" + exeName: "x",
		})
		dest := filepath.Join(t.TempDir(), "stage")
		_ = Extract(zipPath, dest)
		if _, err := FindPackageRoot(dest); err == nil {
			t.Errorf("有多个 exe 时无法确定装哪一个，必须被拒绝")
		}
	})
}

func TestVerifyRejectsIncompletePackages(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "缺少 web/index.html",
			files: map[string]string{exeName: "x", versionRel: "v0.04"},
			want:  "web/index.html",
		},
		{
			name:  "缺少 VERSION",
			files: map[string]string{exeName: "x", webIndexRel: "x"},
			want:  "VERSION",
		},
		{
			name:  "VERSION 内容看不懂",
			files: map[string]string{exeName: "x", webIndexRel: "x", versionRel: "latest"},
			want:  "版本号",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range c.files {
				p := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatalf("准备目录失败: %v", err)
				}
				if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
					t.Fatalf("准备文件失败: %v", err)
				}
			}
			_, err := Verify(dir)
			if err == nil {
				t.Fatalf("不完整的包必须被拒绝")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("错误信息里应当提到 %q，实际：%v", c.want, err)
			}
		})
	}
}

// TestWriteApplyScript 验证替换脚本的内容。
func TestWriteApplyScript(t *testing.T) {
	root := t.TempDir()
	l := Layout{Root: root, ExePath: filepath.Join(root, exeName), CanSelfUpdate: true}

	path, err := WriteApplyScript(ApplyRequest{Layout: l, PID: 4321, Addr: "127.0.0.1:8090"})
	if err != nil {
		t.Fatalf("生成脚本失败: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读脚本失败: %v", err)
	}
	script := string(raw)

	if !strings.Contains(script, "4321") {
		t.Errorf("脚本里没有带上旧进程号，就没法精确地等它退出")
	}
	if !strings.Contains(script, "127.0.0.1:8090") {
		t.Errorf("脚本里没有把监听地址带过去，重启后会换端口")
	}
	if !strings.Contains(script, "web.old") {
		t.Errorf("脚本里没有回滚用的备份名，换到一半失败就退不回去了")
	}
	if !strings.Contains(script, "CNC_NO_BROWSER=1") {
		t.Errorf("重启时应当抑制自动开浏览器，否则用户会多出一个标签页")
	}

	// 行尾必须是 CRLF，否则 cmd.exe 解释批处理会出现难以预料的行为
	if strings.Contains(strings.ReplaceAll(script, "\r\n", ""), "\n") {
		t.Errorf("脚本里存在只有 LF 的行尾")
	}

	// 必须是纯 ASCII：cmd.exe 按控制台代码页解释 .cmd，
	// 写中文的话换一台代码页不同的机器就可能变成乱码甚至语法错误
	for i, r := range script {
		if r > 127 {
			t.Fatalf("脚本第 %d 个字符不是 ASCII（%q），换台机器就可能乱码", i, r)
		}
	}
}

func TestLayoutPaths(t *testing.T) {
	l := Layout{Root: filepath.Join("D:", "cnccool")}
	if got := l.UpdateDir(); got != filepath.Join("D:", "cnccool", updateDirName) {
		t.Errorf("UpdateDir = %q", got)
	}
	if got := l.StageDir(); got != filepath.Join("D:", "cnccool", updateDirName, "stage") {
		t.Errorf("StageDir = %q", got)
	}
}

// TestCleanStageKeepsLog 验证清理暂存时不会把日志一起删掉。
//
// 更新失败时那个日志是唯一的线索，删了就没法查了。
func TestCleanStageKeepsLog(t *testing.T) {
	root := t.TempDir()
	l := Layout{Root: root}

	if err := os.MkdirAll(l.StageDir(), 0o755); err != nil {
		t.Fatalf("准备暂存目录失败: %v", err)
	}
	logPath := filepath.Join(l.UpdateDir(), "apply-update.log")
	if err := os.WriteFile(logPath, []byte("previous run"), 0o644); err != nil {
		t.Fatalf("准备日志失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(l.UpdateDir(), applyScriptName), []byte("x"), 0o644); err != nil {
		t.Fatalf("准备脚本失败: %v", err)
	}

	l.CleanStage()

	if _, err := os.Stat(l.StageDir()); !os.IsNotExist(err) {
		t.Errorf("暂存目录应当被清掉")
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("日志不该被删掉：%v", err)
	}
	if _, err := os.Stat(filepath.Join(l.UpdateDir(), applyScriptName)); !os.IsNotExist(err) {
		t.Errorf("上一轮的脚本应当被清掉")
	}
}
