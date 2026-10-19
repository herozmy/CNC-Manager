package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalLayoutDefaultsNextToExecutable(t *testing.T) {
	dir := t.TempDir()
	webDir := filepath.Join(dir, "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, web := localLayoutDefaults(filepath.Join(dir, "cnccool-server.exe"), filepath.Join(dir, "other"))
	if data != filepath.Join(dir, "data") {
		t.Fatalf("data = %q，期望 exe 同目录的 data", data)
	}
	if web != webDir {
		t.Fatalf("web = %q，期望 %q", web, webDir)
	}
}

func TestLocalLayoutDefaultsKeepSourceDevelopmentBehavior(t *testing.T) {
	dir := t.TempDir()
	workingDir := filepath.Join(dir, "backend")
	data, web := localLayoutDefaults(filepath.Join(dir, "server.exe"), workingDir)
	if data != filepath.Join(workingDir, "data") {
		t.Fatalf("data = %q，期望工作目录下的 data", data)
	}
	if web != "" {
		t.Fatalf("没有前端构建时 web 应为空，实际为 %q", web)
	}
}

func TestLoadAllowsEmptyUpdateRepo(t *testing.T) {
	t.Setenv("CNC_UPDATE_REPO", "")
	t.Setenv("CNC_DATA_DIR", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UpdateRepo != "" {
		t.Fatalf("UpdateRepo = %q，显式空值应关闭更新检查", cfg.UpdateRepo)
	}
}

func TestGetEnvAllowEmptyUsesDefaultOnlyWhenUnset(t *testing.T) {
	const key = "CNC_TEST_ALLOW_EMPTY"
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(key) })
	if got := getEnvAllowEmpty(key, "default"); got != "default" {
		t.Fatalf("未设置时得到 %q，期望 default", got)
	}

	t.Setenv(key, "   ")
	if got := getEnvAllowEmpty(key, "default"); got != "" {
		t.Fatalf("显式空值时得到 %q，期望空字符串", got)
	}
}

func TestLoadAcceptsDockerDeployment(t *testing.T) {
	t.Setenv("CNC_DEPLOYMENT", "docker")
	t.Setenv("CNC_DATA_DIR", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Deployment != "docker" {
		t.Fatalf("Deployment = %q，期望 docker", cfg.Deployment)
	}
}
