package main

import (
	"log"
	"net/http"
	"os"

	"cnc-manager/internal/httpapi"
	"cnc-manager/internal/web"
)

// version 通过 -ldflags "-X main.version=..." 注入，来源为根目录 VERSION。
var version = "dev"

func main() {
	addr := os.Getenv("CNC_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("CNC-Manager %s 监听 %s", version, addr)
	log.Fatal(http.ListenAndServe(addr, httpapi.New(version, web.Handler())))
}
