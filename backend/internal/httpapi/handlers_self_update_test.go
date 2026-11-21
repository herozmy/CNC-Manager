package httpapi

import (
	"bufio"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateRequiresAdminAndForwardsOnlyReleaseTag(t *testing.T) {
	h, _ := newTestServer(t)
	if rec := do(t, h, "POST", "/api/admin/update", `{"target":"v2.6.0"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	cookie := setupAdmin(t, h)
	t.Setenv("CNC_UPDATE_SOCKET", "")
	if rec := do(t, h, "POST", "/api/admin/update", `{"target":"v2.6.0"}`, cookie); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled: %d", rec.Code)
	}
	dir, err := os.MkdirTemp("/tmp", "cnc-update-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "ctl")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	t.Setenv("CNC_UPDATE_SOCKET", path)
	if rec := do(t, h, "POST", "/api/admin/update", `{"target":"latest"}`, cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid: %d", rec.Code)
	}
	command := make(chan string, 1)
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		line, _ := bufio.NewReader(c).ReadString('\n')
		command <- line
		_, _ = c.Write([]byte(`{"phase":"checking","target":"v2.6.0"}`))
	}()
	if rec := do(t, h, "POST", "/api/admin/update", `{"target":"v2.6.0"}`, cookie); rec.Code != http.StatusOK {
		t.Fatalf("start: %d", rec.Code)
	}
	if got := <-command; got != "v2.6.0\n" {
		t.Fatal(got)
	}
}
