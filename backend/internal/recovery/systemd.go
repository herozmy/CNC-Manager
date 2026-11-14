package recovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Systemd struct{ Name, Data string }

func (s Systemd) command(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if s.Name == "" || strings.HasPrefix(s.Name, "-") || strings.ContainsAny(s.Name, "\r\n/") {
		return "", errors.New("无效的 systemd 服务名称")
	}
	output, err := exec.CommandContext(ctx, "systemctl", append(args, "--", s.Name)...).Output()
	if err != nil {
		return "", fmt.Errorf("systemctl %s %s 失败: %w", args[0], s.Name, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (s Systemd) Running(ctx context.Context) (bool, error) {
	if os.Geteuid() != 0 {
		return false, errors.New("管理 systemd 服务需要服务器 root 权限；离线副本可用 --no-service")
	}
	load, err := s.command(ctx, "show", "--property=LoadState", "--value")
	if err != nil {
		return false, err
	}
	if load != "loaded" {
		return false, errors.New("指定的 systemd 服务未安装")
	}
	state, err := s.command(ctx, "show", "--property=ActiveState", "--value")
	if err != nil {
		return false, err
	}
	if state != "active" && state != "inactive" && state != "failed" {
		return false, errors.New("服务正在切换状态，请稍后重试")
	}
	env, err := s.command(ctx, "show", "--property=Environment", "--value")
	if err != nil {
		return false, err
	}
	if err := matchDataDir(env, s.Data); err != nil {
		return false, err
	}
	return state == "active", nil
}

func (s Systemd) Stop(ctx context.Context) error { _, err := s.command(ctx, "stop"); return err }
func (s Systemd) Start(ctx context.Context) error {
	if _, err := s.command(ctx, "start"); err != nil {
		return err
	}
	_, err := s.command(ctx, "is-active", "--quiet")
	return err
}

// systemctl show quotes an entire assignment when it contains spaces and escapes
// quotes/backslashes with C escapes. Reject malformed/relative directories.
func matchDataDir(env, data string) error {
	var configured string
	for len(strings.TrimSpace(env)) > 0 {
		env = strings.TrimSpace(env)
		var word string
		if env[0] == '"' {
			end := 1
			for ; end < len(env); end++ {
				if env[end] == '\\' {
					end++
					continue
				}
				if env[end] == '"' {
					break
				}
			}
			if end >= len(env) {
				return errors.New("无法解析服务 Environment")
			}
			var err error
			word, err = strconv.Unquote(env[:end+1])
			if err != nil {
				return errors.New("无法解析服务 Environment")
			}
			env = env[end+1:]
		} else {
			end := strings.IndexAny(env, " \t\n")
			if end < 0 {
				end = len(env)
			}
			word, env = env[:end], env[end:]
		}
		if value, ok := strings.CutPrefix(word, "CNC_DATA_DIR="); ok {
			configured = value
		}
	}
	if !filepath.IsAbs(configured) {
		return errors.New("服务需显式配置绝对路径 CNC_DATA_DIR，已停止操作")
	}
	a, err := filepath.EvalSymlinks(configured)
	if err != nil {
		return err
	}
	b, err := filepath.EvalSymlinks(data)
	if err != nil {
		return err
	}
	b, err = filepath.Abs(b)
	if err != nil {
		return err
	}
	if a != b {
		return errors.New("服务数据目录与指定目录不一致，已停止操作")
	}
	return nil
}
