package httpapi

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// mountWeb 把前端静态文件挂到根路径下。
//
// 只在配置了 CNC_WEB_DIR 时才挂。开发时不配这个变量，前端由 Vite 提供，
// 前后端保持独立（前端照样能热更新）；打包发布时配上它，一个 exe 就能同时
// 提供接口和界面，用户解压即用，不需要装 Node，也不需要额外架 nginx。
//
// 注意 /api 的路由是显式注册的，优先级高于这里的通配，所以两种模式不冲突。
func (s *Server) mountWeb(r chi.Router) {
	if s.cfg.WebDir == "" {
		return
	}

	webDir := s.cfg.WebDir
	indexPath := filepath.Join(webDir, "index.html")
	fileServer := http.FileServer(http.Dir(webDir))

	handler := func(w http.ResponseWriter, req *http.Request) {
		// path.Clean 会把 "/../x" 规整成 "/x"，前面再补一个 "/"，
		// 结果里就不可能残留 ".."，因此拼接后不会跑到 webDir 外面去。
		rel := strings.TrimPrefix(path.Clean("/"+req.URL.Path), "/")
		target := filepath.Join(webDir, filepath.FromSlash(rel))

		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			// 前端构建产物的文件名里带内容哈希，内容一变文件名就变，
			// 所以 assets 下的东西可以放心让浏览器长期缓存。
			if strings.HasPrefix(rel, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, req)
			return
		}

		// 走到这里说明磁盘上没有这个文件。
		//
		// 带扩展名的地址（/assets/x.js、/favicon.ico）是在找具体文件，
		// 直接 404 更诚实：否则浏览器会把 index.html 当成 JS 去解析，
		// 报出来的错误跟真正的原因八竿子打不着。
		// 不带扩展名的当它是前端路由地址，回 index.html 交给前端解析。
		if rel != "" && strings.Contains(path.Base(rel), ".") {
			writeError(w, http.StatusNotFound, "静态资源不存在：/"+rel)
			return
		}
		if _, err := os.Stat(indexPath); err != nil {
			writeError(w, http.StatusNotFound,
				"前端目录里没有 index.html："+indexPath+"，请确认 CNC_WEB_DIR 指向了构建产物目录")
			return
		}
		// index.html 绝不缓存，否则换了前端文件以后用户刷新看到的还是旧版本。
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, req, indexPath)
	}

	// HEAD 也要注册：chi 的 Get 只管 GET，不注册的话静态资源探测会拿到 405。
	r.Get("/*", handler)
	r.Head("/*", handler)
}

// ---------------------------------------------------------------------------
// 前端版本比对
//
// 场景：用户开着页面，我们把 web\ 目录覆盖成新版本。页面里跑的还是旧的 JS，
// 他不会知道要刷新。解决办法是让前端能问出「服务器现在打算给你哪一份前端」，
// 再和自己正在跑的那一份比一比。
//
// 指纹就用入口脚本的文件名。Vite 构建出来的产物名带内容哈希，
// 前端一改文件名就变，所以这个名字等价于「这一版前端」的身份。
// 这样做不需要在构建时往代码里塞版本号，前后端各自从 index.html 取同一个值。
// ---------------------------------------------------------------------------

var (
	reScriptTag        = regexp.MustCompile(`(?is)<script\b[^>]*>`)
	reScriptSrc        = regexp.MustCompile(`(?is)\bsrc\s*=\s*["']([^"']+)["']`)
	reScriptTypeModule = regexp.MustCompile(`(?is)\btype\s*=\s*["']module["']`)
)

// parseEntryScript 从 index.html 里取出入口脚本地址，取不到返回空串。
//
// 只看带 type="module" 的脚本：Vite 产出的入口就是它。
// 属性顺序不固定，所以先把标签整个抠出来再分别找 src 和 type。
func parseEntryScript(html []byte) string {
	for _, tag := range reScriptTag.FindAll(html, -1) {
		if !reScriptTypeModule.Match(tag) {
			continue
		}
		if m := reScriptSrc.FindSubmatch(tag); m != nil {
			return string(m[1])
		}
	}
	return ""
}

// webEntryCache 缓存从 index.html 解析出来的入口脚本地址。
//
// 免安装版的升级方式就是直接覆盖 web\ 目录，服务进程不一定跟着重启，
// 所以缓存按 index.html 的修改时间和大小失效：覆盖完下一次请求就能反映出来，
// 平时又不用每次都读盘。
type webEntryCache struct {
	mu    sync.Mutex
	valid bool
	mtime time.Time
	size  int64
	entry string
}

// webEntry 返回当前 web\index.html 引用的入口脚本地址（形如 /assets/index-abc.js）。
//
// 没配置 CNC_WEB_DIR（开发模式，前端由 Vite 提供）、或者文件里找不到时返回空串。
// 前端拿到空串就跳过比对——宁可不提示，也不能拿一个假指纹去误报。
func (s *Server) webEntry() string {
	if s.cfg.WebDir == "" {
		return ""
	}

	indexPath := filepath.Join(s.cfg.WebDir, "index.html")
	info, err := os.Stat(indexPath)
	if err != nil {
		return ""
	}

	c := &s.web
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.valid && c.size == info.Size() && c.mtime.Equal(info.ModTime()) {
		return c.entry
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return ""
	}
	c.entry = parseEntryScript(data)
	c.mtime = info.ModTime()
	c.size = info.Size()
	c.valid = true
	return c.entry
}
