package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
}

func get(t *testing.T, p string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler(testFS()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
	return rec
}

func TestServesIndexAtRoot(t *testing.T) {
	rec := get(t, "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestServesAsset(t *testing.T) {
	if rec := get(t, "/assets/app.js"); rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestClientRouteFallsBackToIndex(t *testing.T) {
	rec := get(t, "/drawings/12")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMissingAssetIs404(t *testing.T) {
	if rec := get(t, "/assets/missing.js"); rec.Code != 404 {
		t.Fatalf("code=%d，期望 404", rec.Code)
	}
}
