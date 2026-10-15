package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"cnccool/internal/config"
	"cnccool/internal/domain"
	"cnccool/internal/ncstore"
	"cnccool/internal/repo"
	"cnccool/internal/store"
)

func TestDownloadVersionReturnsOriginalProgram(t *testing.T) {
	temp := t.TempDir()
	db, err := store.Open(filepath.Join(temp, "download.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	repository := repo.New(db)
	ctx := context.Background()
	drawing, err := repository.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "TEST-001", Name: "download"})
	if err != nil {
		t.Fatalf("create drawing: %v", err)
	}
	operation, err := repository.CreateOperation(ctx, drawing.ID, domain.OperationInput{OpNo: 10, OpName: "roughing"})
	if err != nil {
		t.Fatalf("create operation: %v", err)
	}
	program, err := repository.CreateProgram(ctx, operation.ID, domain.ProgramInput{ProgramNo: "O1234"})
	if err != nil {
		t.Fatalf("create program: %v", err)
	}

	files, err := ncstore.New(filepath.Join(temp, "nc"), 1024*1024)
	if err != nil {
		t.Fatalf("create NC store: %v", err)
	}
	const body = "%\nO1234\nG01 X10.0\nM30\n%\n"
	saved, err := files.Save(strings.NewReader(body), "O1234.nc")
	if err != nil {
		t.Fatalf("save NC file: %v", err)
	}
	version, err := repository.AddVersion(ctx, program.ID, domain.NCFileInput{
		SHA256: saved.SHA256, SizeBytes: saved.SizeBytes, OriginalName: saved.OriginalName, RelPath: saved.RelPath,
	}, "initial upload")
	if err != nil {
		t.Fatalf("add version: %v", err)
	}

	server := NewServer(repository, files, &config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	req := httptest.NewRequest(http.MethodGet, "/api/versions/1/download", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	recorder := httptest.NewRecorder()
	server.handleDownloadVersion(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("download status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Body.String(); got != body {
		t.Fatalf("download body = %q, want %q", got, body)
	}
	if disposition := recorder.Header().Get("Content-Disposition"); !strings.Contains(disposition, version.FileName) || !strings.HasPrefix(disposition, "attachment;") {
		t.Fatalf("Content-Disposition = %q, want attachment with %q", disposition, version.FileName)
	}
}
