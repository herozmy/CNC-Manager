package config

import (
	"path/filepath"
	"testing"
)

func TestNewLayout(t *testing.T) {
	l := NewLayout("/data")
	if l.DBPath != filepath.Join("/data", "cnc.db") || l.NCDir != filepath.Join("/data", "nc") || l.KeyPath != filepath.Join("/data", "nc.key") {
		t.Fatalf("布局异常: %+v", l)
	}
}
