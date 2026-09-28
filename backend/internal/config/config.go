// Package config 集中管理运行期配置。
//
// 所有配置项一律来自环境变量，代码里不出现任何硬编码绝对路径。
// 这样后期容器化（Docker）时只需要改环境变量，业务代码一行都不用动。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config 是服务运行所需的全部配置。
type Config struct {
	Addr        string   // HTTP 监听地址，如 127.0.0.1:8080
	DataDir     string   // 数据根目录（数据库文件 + NC 文件库）
	DBPath      string   // SQLite 数据库文件绝对路径
	NCDir       string   // NC 文件托管目录绝对路径
	WebDir      string   // 可选：前端静态文件目录，设了就在同一端口顺带托管前端
	MaxUploadMB int64    // 单个 NC 文件上传上限（MB）
	CORSOrigins []string // 允许跨域访问的前端来源，开发时是 Vite 的 5173
	LogLevel    string   // debug / info / warn / error

	// UpdateRepo 是查新版本用的 GitHub 仓库，形如 owner/name。
	// 留空表示不检查更新——离线车间可以这么关掉，服务就完全不去连外网。
	UpdateRepo string
	// UpdateAPI 是 GitHub API 的地址，默认用官方地址。
	// 做成可配置是为了两件事：企业内网自建，以及测试时指向一个本地假服务。
	UpdateAPI string
}

// Load 从环境变量装载配置，缺省值适用于本机开发。
func Load() (*Config, error) {
	c := &Config{
		Addr:        getEnv("CNC_ADDR", "127.0.0.1:8080"),
		MaxUploadMB: getEnvInt("CNC_MAX_UPLOAD_MB", 64),
		LogLevel:    getEnv("CNC_LOG_LEVEL", "info"),

		UpdateRepo: getEnv("CNC_UPDATE_REPO", "herozmy/CNC-Manager"),
		UpdateAPI:  getEnv("CNC_UPDATE_API", ""),
	}

	// 数据目录统一解析成绝对路径，避免因为启动目录不同而"数据找不到"。
	absData, err := filepath.Abs(getEnv("CNC_DATA_DIR", "data"))
	if err != nil {
		return nil, fmt.Errorf("解析数据目录失败: %w", err)
	}
	c.DataDir = absData
	c.DBPath = getEnv("CNC_DB_PATH", filepath.Join(absData, "cnccool.db"))
	c.NCDir = getEnv("CNC_NC_DIR", filepath.Join(absData, "nc"))

	// 可选：设了就顺带在同一端口托管前端静态文件。
	//
	// 开发时不设，前端由 Vite 提供，前后端保持独立、前端可以热更新；
	// 打包发布时设上它，解压一个目录、双击 start.cmd 就能用，
	// 不需要 Node、也不需要 nginx。
	if webDir := getEnv("CNC_WEB_DIR", ""); webDir != "" {
		absWeb, err := filepath.Abs(webDir)
		if err != nil {
			return nil, fmt.Errorf("解析前端目录失败: %w", err)
		}
		c.WebDir = absWeb
	}

	for _, o := range strings.Split(getEnv("CNC_CORS_ORIGINS", "http://127.0.0.1:5173,http://localhost:5173"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}

	if c.MaxUploadMB <= 0 {
		return nil, fmt.Errorf("CNC_MAX_UPLOAD_MB 必须为正数，当前为 %d", c.MaxUploadMB)
	}
	return c, nil
}

// MaxUploadBytes 返回上传大小上限的字节数。
func (c *Config) MaxUploadBytes() int64 { return c.MaxUploadMB * 1024 * 1024 }

func getEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
