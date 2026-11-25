package datalock

import (
	"path/filepath"
	"testing"
)

func TestExclusiveUntilClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".cnc-manager.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Acquire(path); err == nil {
		second.Close()
		first.Close()
		t.Fatal("second acquisition succeeded while the first lock was held")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
}
