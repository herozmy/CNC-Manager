package config

import (
	"fmt"
	"strconv"
	"time"
)

// Config 是服务运行配置，全部来自环境变量。
type Config struct {
	Addr            string        // CNC_ADDR，默认 127.0.0.1:8080；容器内用 0.0.0.0:8080
	DataDir         string        // CNC_DATA_DIR，默认 ./data
	MaxUploadBytes  int64         // CNC_MAX_UPLOAD_MB，默认 64
	SessionLifetime time.Duration // CNC_SESSION_HOURS，默认 168（7 天）
	Layout          Layout
}

// Load 读取配置。getenv 通常传 os.Getenv，测试里传假函数。
func Load(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}
	c := Config{Addr: get("CNC_ADDR", "127.0.0.1:8080"), DataDir: get("CNC_DATA_DIR", "data")}

	mb, err := strconv.Atoi(get("CNC_MAX_UPLOAD_MB", "64"))
	if err != nil || mb < 1 || mb > 1024 {
		return Config{}, fmt.Errorf("CNC_MAX_UPLOAD_MB 必须是 1~1024 的整数")
	}
	c.MaxUploadBytes = int64(mb) << 20

	hours, err := strconv.Atoi(get("CNC_SESSION_HOURS", "168"))
	if err != nil || hours < 1 || hours > 24*365 {
		return Config{}, fmt.Errorf("CNC_SESSION_HOURS 必须是 1~8760 的整数")
	}
	c.SessionLifetime = time.Duration(hours) * time.Hour
	c.Layout = NewLayout(c.DataDir)
	return c, nil
}
