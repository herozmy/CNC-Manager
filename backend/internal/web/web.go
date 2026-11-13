// Package web 内嵌前端构建产物，并提供 SPA 回退。
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler 返回内嵌前端的处理器。
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return handler(sub)
}

func handler(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(files, name); err != nil {
			// 带扩展名的资源缺失返回 404；其余路径交给前端路由。
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			r.URL.Path = "/"
		}
		if name == "index.html" || path.Ext(name) == "" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}
