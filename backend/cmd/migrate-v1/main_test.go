package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFailedImportCleansTarget(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "new-directory"
		if existing {
			name = "existing-empty-directory"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			to := filepath.Join(root, "target")
			if existing {
				if err := os.Mkdir(to, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := run(filepath.Join(root, "missing-v1"), to); err == nil {
				t.Fatal("missing source must fail")
			}
			entries, err := os.ReadDir(to)
			if existing {
				if err != nil || len(entries) != 0 {
					t.Fatalf("original empty directory must remain reusable: %v, %v", entries, err)
				}
				// A corrected source can be retried without a stale nonempty-target error.
				if err := run(filepath.Join(root, "missing-v1"), to); err == nil || strings.Contains(err.Error(), "非空") {
					t.Fatalf("retry blocked by leftover data: %v", err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("new failed directory must be removed: %v", err)
			}
		})
	}
}

func TestRefusesExistingDataWithoutChangingIt(t *testing.T) {
	to := t.TempDir()
	path := filepath.Join(to, "cnc.db")
	want := []byte("preserve existing installation")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(t.TempDir(), to); err == nil || !strings.Contains(err.Error(), "非空") {
		t.Fatalf("expected nonempty-target refusal: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(want) {
		t.Fatalf("existing data changed: %q, %v", got, err)
	}
}

func TestRefusesFileAsTarget(t *testing.T) {
	to := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(to, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(t.TempDir(), to); err == nil || !strings.Contains(err.Error(), "读取目标目录") {
		t.Fatalf("expected target validation error: %v", err)
	}
	got, err := os.ReadFile(to)
	if err != nil || string(got) != "preserve" {
		t.Fatalf("target file changed: %q, %v", got, err)
	}
}
