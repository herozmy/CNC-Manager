package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestPackSetsLinuxFileModes(t *testing.T) {
	source := t.TempDir()
	for name, content := range map[string]string{
		"CNC-Manager":         "binary",
		"install.sh":          "#!/bin/sh\n",
		"uninstall.sh":        "#!/bin/sh\n",
		"cnc-manager.service": "service",
		"README.txt":          "readme",
	} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(source, "web"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "web", "index.html"), []byte("html"), 0o666); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(t.TempDir(), "release.tar.gz")
	if err := pack(source, archive, "cnccool-test-linux-amd64"); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	modes := map[string]int64{}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		modes[header.Name] = header.Mode
	}

	for _, name := range []string{"cnccool-test-linux-amd64", "cnccool-test-linux-amd64/web", "cnccool-test-linux-amd64/CNC-Manager", "cnccool-test-linux-amd64/install.sh", "cnccool-test-linux-amd64/uninstall.sh"} {
		if modes[name] != 0o755 {
			t.Errorf("%s mode = %#o，期望 0755", name, modes[name])
		}
	}
	for _, name := range []string{"cnccool-test-linux-amd64/README.txt", "cnccool-test-linux-amd64/cnc-manager.service", "cnccool-test-linux-amd64/web/index.html"} {
		if modes[name] != 0o644 {
			t.Errorf("%s mode = %#o，期望 0644", name, modes[name])
		}
	}
}
