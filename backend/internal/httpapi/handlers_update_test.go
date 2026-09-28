package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsLoopback(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8080", true},
		{"127.0.0.1:1", true},
		{"[::1]:8080", true},
		{"192.168.1.20:5000", false},
		{"10.0.0.5:1234", false},
		{"0.0.0.0:8080", false},
		{"8.8.8.8:80", false},
		{"", false},
		{"garbage", false},
	}
	for _, c := range cases {
		if got := isLoopback(c.addr); got != c.want {
			t.Errorf("isLoopback(%q) = %v，期望 %v", c.addr, got, c.want)
		}
	}
}

// TestInstallRejectedFromOtherMachines 是这组里最要紧的一条。
//
// 这个接口能把 exe 换掉，等价于在本机执行任意代码。README 里写了
// 「局域网访问就把 CNC_ADDR 改成 0.0.0.0:8080」，一旦有人这么配，
// 少了这道判断，整个局域网都能往这儿传包。
func TestInstallRejectedFromOtherMachines(t *testing.T) {
	h := newWebServer(t, "")

	req := httptest.NewRequest(http.MethodPost, "/api/update/install", nil)
	req.RemoteAddr = "192.168.1.50:41234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("来自局域网的状态码 = %d，期望 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "本机") {
		t.Errorf("错误信息应当说清只能在本机操作：%q", rec.Body.String())
	}
}

// TestInstallRejectedWhenNotPortable 验证开发模式下会明确拒绝，而不是做一半。
func TestInstallRejectedWhenNotPortable(t *testing.T) {
	h := newWebServer(t, "")

	req := httptest.NewRequest(http.MethodPost, "/api/update/install", nil)
	req.RemoteAddr = "127.0.0.1:41234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非免安装布局下的状态码 = %d，期望 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "免安装") {
		t.Errorf("错误信息应当点明不是免安装布局：%q", rec.Body.String())
	}
}

// TestUpdateCheckNeverFailsTheRequest 验证检查更新失败时也返回 200。
//
// 车间没网是常态。要是这里返回 5xx，前端就得区分「查不到」和「没有新版本」，
// 而这两件事对用户来说完全一样：什么都不用做。
func TestUpdateCheckNeverFailsTheRequest(t *testing.T) {
	h := newWebServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/api/update/check", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"hasUpdate":false`) {
		t.Errorf("应当明确给出 hasUpdate=false：%q", body)
	}
	// 没配仓库时必须带上原因，否则前端只能瞎猜
	if !strings.Contains(body, "error") {
		t.Errorf("查不到时应当带上原因：%q", body)
	}
}

// TestMetaReportsInstallCapability 验证 /api/meta 会告诉界面能不能自更新。
//
// 开发模式下要如实报 false，界面才知道该把「离线安装」藏起来，
// 而不是让用户点了才发现不支持。
func TestMetaReportsInstallCapability(t *testing.T) {
	h := newWebServer(t, "")

	rec := doGet(t, h, "/api/meta")
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "canInstall") {
		t.Errorf("/api/meta 应当带上 canInstall：%q", rec.Body.String())
	}
}
