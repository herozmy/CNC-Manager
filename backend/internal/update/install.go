package update

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	// exeName 是包内可执行文件的名字，也是「这确实是个免安装包」的主要依据。
	exeName = "cnccool-server.exe"
	// webIndexRel 和 versionRel 是包里另外两个必需文件。
	webIndexRel = "web/index.html"
	versionRel  = "VERSION"

	// updateDirName 是安装目录下放暂存内容和替换脚本的地方。
	updateDirName = ".update"

	// maxExtractBytes 限制解压后的总大小。
	// 上传的是一个压缩包，不设上限的话一个几十 KB 的炸弹就能把磁盘写满。
	maxExtractBytes = 512 << 20 // 512 MB

	// applyScriptName 是负责换文件的批处理。
	applyScriptName = "apply-update.cmd"
)

// Layout 描述当前程序是以什么形态装着的。
type Layout struct {
	Root    string // exe 所在目录，也就是免安装包的根
	ExePath string
	// CanSelfUpdate 表示这个形态能不能自己更新自己。
	// 开发模式下 exe 在 backend\ 而前端在 dist\，这个值是 false，
	// 界面会把安装入口藏起来，不会让用户点了才发现不支持。
	CanSelfUpdate bool
}

// DetectLayout 找出可执行文件所在的目录，并判断这个形态能不能自更新。
func DetectLayout() Layout {
	exe, err := os.Executable()
	if err != nil {
		return Layout{}
	}
	// 走软链/快捷方式时取真实路径，否则会更新到错的目录
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	root := filepath.Dir(exe)
	l := Layout{Root: root, ExePath: exe}
	if fileExists(filepath.Join(root, filepath.FromSlash(webIndexRel))) {
		l.CanSelfUpdate = true
	}
	return l
}

// UpdateDir 返回暂存目录的路径。
func (l Layout) UpdateDir() string { return filepath.Join(l.Root, updateDirName) }

// StageDir 返回解压新版本的位置。
func (l Layout) StageDir() string { return filepath.Join(l.UpdateDir(), "stage") }

// CleanStage 清掉上一次留下的暂存内容，在启动时调用。
//
// 只动 stage 目录，不碰 apply-update.log —— 那个日志是更新失败时唯一的线索，
// 留着给人看。
func (l Layout) CleanStage() {
	if l.Root == "" {
		return
	}
	_ = os.RemoveAll(l.StageDir())
	_ = os.Remove(filepath.Join(l.UpdateDir(), applyScriptName))
}

// CleanLeftovers 清掉上一次离线安装留下的暂存内容。服务启动时调用一次。
//
// 替换脚本出于安全考虑不删自己（正在运行的 .cmd 也删不掉），
// 所以清理放在下一次启动做。
func CleanLeftovers() { DetectLayout().CleanStage() }

// Extract 把上传上来的离线包解压到 destDir。
//
// 这里挡两类东西：写出到目录外面的路径（zip-slip），以及解压后体积失控。
func Extract(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return errors.New("这不是一个有效的 zip 文件")
	}
	defer func() { _ = zr.Close() }()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("创建暂存目录失败: %w", err)
	}

	var total int64
	for _, f := range zr.File {
		// 包里的路径可能用反斜杠（有些压缩工具会这么写）
		name := strings.ReplaceAll(f.Name, "\\", "/")

		target, err := safeJoin(destDir, name)
		if err != nil {
			return err
		}

		// 目录项
		if strings.HasSuffix(name, "/") || f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
			continue
		}

		// 符号链接一律拒绝：它可以让解压写到 destDir 外面的任何地方，
		// 而离线安装包本来就不需要链接。
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("压缩包里含有符号链接，已拒绝解压：%s", name)
		}

		if f.UncompressedSize64 > uint64(maxExtractBytes) {
			return fmt.Errorf("压缩包解压后过大（超过 %d MB），已拒绝", maxExtractBytes>>20)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		written, err := extractFile(f, target, maxExtractBytes-total)
		if err != nil {
			return err
		}
		total += written
	}
	return nil
}

// extractFile 写出单个文件，并保证不超过 remaining 字节。
//
// 不能只信压缩包头里写的大小——那是可以造假的，所以边写边数。
func extractFile(f *zip.File, target string, remaining int64) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("读取压缩包内容失败: %w", err)
	}
	defer func() { _ = rc.Close() }()

	out, err := os.Create(target)
	if err != nil {
		return 0, fmt.Errorf("写入文件失败: %w", err)
	}
	defer func() { _ = out.Close() }()

	if remaining <= 0 {
		return 0, fmt.Errorf("压缩包解压后过大（超过 %d MB），已拒绝", maxExtractBytes>>20)
	}

	written, err := io.Copy(out, io.LimitReader(rc, remaining+1))
	if err != nil {
		return written, fmt.Errorf("解压文件失败: %w", err)
	}
	if written > remaining {
		return written, fmt.Errorf("压缩包解压后过大（超过 %d MB），已拒绝", maxExtractBytes>>20)
	}
	return written, nil
}

// safeJoin 把压缩包里的条目名拼到 destDir 下，越界就报错。
//
// 这就是所谓的 zip-slip：条目名可以写成 ../../windows/system32/x.dll，
// 照抄着解压就写到安装目录外面去了。所有条目都必须过这一关。
func safeJoin(destDir, name string) (string, error) {
	// path.Clean 会把 "/../x" 规整成 "/x"，再砍掉前导斜杠，
	// 结果里就不可能残留 ".."，拼接后不会跑到 destDir 外面。
	clean := strings.TrimPrefix(path.Clean("/"+name), "/")
	if clean == "" || clean == "." {
		return destDir, nil
	}

	target := filepath.Join(destDir, filepath.FromSlash(clean))
	// 双保险：拼完之后再确认一次确实还在 destDir 里面
	if target != destDir && !strings.HasPrefix(target, destDir+string(filepath.Separator)) {
		return "", fmt.Errorf("压缩包里的路径越界，已拒绝：%s", name)
	}
	return target, nil
}

// FindPackageRoot 找到包里 cnccool-server.exe 所在的目录。
//
// 大多数人拿到的是我们发布的那个 zip，exe 就在根目录；但也有人会先解压、
// 再自己压回去，于是多套了一层文件夹。这里把这种情况也认下来，
// 免得用户对着「包格式不对」干瞪眼。
func FindPackageRoot(dir string) (string, error) {
	if fileExists(filepath.Join(dir, exeName)) {
		return dir, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("读取解压结果失败: %w", err)
	}

	var found []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if fileExists(filepath.Join(sub, exeName)) {
			found = append(found, sub)
		}
	}

	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("压缩包里找不到 %s，请确认下载的是 Windows 免安装包", exeName)
	default:
		return "", fmt.Errorf("压缩包里有多个 %s，无法确定该装哪一个", exeName)
	}
}

// Verify 确认解压出来的目录确实是一个完整的免安装包，返回包里的版本号。
func Verify(root string) (string, error) {
	for _, rel := range []string{exeName, webIndexRel} {
		if !fileExists(filepath.Join(root, filepath.FromSlash(rel))) {
			// 提示里用包内的正斜杠写法，用户在压缩包里看到的就是这个
			return "", fmt.Errorf("压缩包里缺少 %s，不是完整的免安装包", rel)
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, versionRel))
	if err != nil {
		return "", fmt.Errorf("压缩包里缺少 VERSION 文件，无法确认版本号")
	}
	v := strings.TrimSpace(string(raw))
	if _, ok := ParseVersion(v); !ok {
		return "", fmt.Errorf("压缩包里的版本号看不懂：%q", v)
	}
	return v, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
