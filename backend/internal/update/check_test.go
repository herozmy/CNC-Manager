package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeGitHub 起一个假的 GitHub API，返回指定的 /releases/latest 响应。
func fakeGitHub(t *testing.T, status int, body string, hits *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		if r.URL.Path != "/repos/herozmy/CNC-Manager/releases/latest" {
			http.NotFound(w, r)
			return
		}
		// GitHub 要求带 User-Agent，缺了会 403
		if r.Header.Get("User-Agent") == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const releaseJSON = `{
  "tag_name": "v0.04",
  "html_url": "https://github.com/herozmy/CNC-Manager/releases/tag/v0.04",
  "published_at": "2026-10-01T00:00:00Z",
  "draft": false,
  "prerelease": false,
  "assets": [
    {"name": "checksums.txt", "browser_download_url": "https://x/checksums.txt", "size": 100},
    {"name": "cnccool-v0.04-windows-amd64.zip", "browser_download_url": "https://x/pkg.zip", "size": 5740845}
  ]
}`

func TestCheckFindsNewVersion(t *testing.T) {
	srv := fakeGitHub(t, http.StatusOK, releaseJSON, nil)
	c := NewClient(srv.URL, "herozmy/CNC-Manager", "v0.03")

	st := c.Check(context.Background(), "v0.03", false)

	if st.Error != "" {
		t.Fatalf("不该有错误：%s", st.Error)
	}
	if !st.HasUpdate {
		t.Errorf("v0.03 -> v0.04 应该判定为有新版本")
	}
	if st.Latest != "v0.04" {
		t.Errorf("Latest = %q，期望 v0.04", st.Latest)
	}
	// 附件要挑出 zip，而不是同目录下的 checksums.txt
	if st.AssetName != "cnccool-v0.04-windows-amd64.zip" {
		t.Errorf("AssetName = %q，期望挑中 windows-amd64 的 zip", st.AssetName)
	}
	if st.ReleaseURL == "" || st.AssetURL == "" {
		t.Errorf("发布页地址和附件地址都应该带出来")
	}
}

func TestCheckSameVersionHasNoUpdate(t *testing.T) {
	srv := fakeGitHub(t, http.StatusOK, releaseJSON, nil)
	c := NewClient(srv.URL, "herozmy/CNC-Manager", "v0.04")

	if st := c.Check(context.Background(), "v0.04", false); st.HasUpdate {
		t.Errorf("版本相同时不该提示有新版本")
	}
}

func TestCheckNeverSuggestsDowngrade(t *testing.T) {
	// 本地比仓库还新（开发中的版本），绝不能提示「有新版本」诱导用户降级
	srv := fakeGitHub(t, http.StatusOK, releaseJSON, nil)
	c := NewClient(srv.URL, "herozmy/CNC-Manager", "v0.05")

	if st := c.Check(context.Background(), "v0.05", false); st.HasUpdate {
		t.Errorf("本地版本更新时不该提示有新版本")
	}
}

func TestCheckCachesResult(t *testing.T) {
	var hits int32
	srv := fakeGitHub(t, http.StatusOK, releaseJSON, &hits)
	c := NewClient(srv.URL, "herozmy/CNC-Manager", "v0.03")

	for i := 0; i < 5; i++ {
		c.Check(context.Background(), "v0.03", false)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Errorf("连续查 5 次打了 %d 次接口，期望只打 1 次（缓存没生效）", n)
	}

	// fresh=true 是用户手工点「检查更新」，必须绕过缓存
	c.Check(context.Background(), "v0.03", true)
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Errorf("fresh 检查后累计 %d 次，期望 2 次", n)
	}
}

func TestCheckNoReleaseIsNotAnError(t *testing.T) {
	// 仓库一个正式发布都没有时 GitHub 返回 404
	srv := fakeGitHub(t, http.StatusNotFound, `{"message":"Not Found"}`, nil)
	c := NewClient(srv.URL, "herozmy/CNC-Manager", "v0.03")

	st := c.Check(context.Background(), "v0.03", false)
	if st.HasUpdate {
		t.Errorf("没有发布时不该提示有新版本")
	}
	if st.Error == "" {
		t.Errorf("应该带上一个能看懂的原因，方便排查")
	}
}

func TestCheckRateLimited(t *testing.T) {
	srv := fakeGitHub(t, http.StatusForbidden, `{"message":"rate limit"}`, nil)
	c := NewClient(srv.URL, "herozmy/CNC-Manager", "v0.03")

	st := c.Check(context.Background(), "v0.03", false)
	if st.HasUpdate {
		t.Errorf("限流时不该提示有新版本")
	}
	if st.Error == "" {
		t.Fatalf("应该带上原因")
	}
	// 限流是最常见的失败，提示里要说到点子上，否则用户以为是自己网断了
	if !strings.Contains(st.Error, "频繁") {
		t.Errorf("限流的提示没说清是查得太勤：%q", st.Error)
	}
}

func TestCheckDisabledWhenNoRepo(t *testing.T) {
	c := NewClient("", "", "v0.03")
	if c.Enabled() {
		t.Fatalf("没配仓库时不该认为自己可用")
	}
	st := c.Check(context.Background(), "v0.03", false)
	if st.HasUpdate || st.Error == "" {
		t.Errorf("没配仓库时应当安静地返回一个原因，而不是报有新版本")
	}
}

func TestCheckUnreachableAPI(t *testing.T) {
	// 车间没网就是这个样子：连不上，但界面不能因此报错
	c := NewClient("http://127.0.0.1:1", "herozmy/CNC-Manager", "v0.03")
	st := c.Check(context.Background(), "v0.03", false)
	if st.HasUpdate {
		t.Errorf("连不上时不该提示有新版本")
	}
	if st.Error == "" {
		t.Errorf("应该带上原因")
	}
}

func TestPickAssetFallsBackToAnyZip(t *testing.T) {
	// 将来改了命名规则也不该立刻失效
	assets := []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	}{
		{Name: "notes.md", BrowserDownloadURL: "https://x/n.md"},
		{Name: "cnccool-v1.0-win.zip", BrowserDownloadURL: "https://x/a.zip"},
	}
	name, url, _ := pickAsset(assets)
	if name != "cnccool-v1.0-win.zip" || url != "https://x/a.zip" {
		t.Errorf("应当退而求其次挑第一个 zip，实际 %q / %q", name, url)
	}
}
