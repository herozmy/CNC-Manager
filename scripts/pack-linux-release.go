// Command pack-linux-release creates the Linux release archive with Unix modes.
// Windows tar tools otherwise record every staged file as non-executable.
package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	source := flag.String("source", "", "staged release directory")
	output := flag.String("output", "", "target tar.gz path")
	name := flag.String("name", "", "top-level archive directory")
	flag.Parse()
	if *source == "" || *output == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "source, output and name are required")
		os.Exit(2)
	}
	if err := pack(*source, *output, *name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func pack(source, output, name string) (err error) {
	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()

	gz := gzip.NewWriter(out)
	defer func() {
		if closeErr := gz.Close(); err == nil {
			err = closeErr
		}
	}()
	tw := tar.NewWriter(gz)
	defer func() {
		if closeErr := tw.Close(); err == nil {
			err = closeErr
		}
	}()

	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		archiveName := name
		if rel != "." {
			archiveName += "/" + filepath.ToSlash(rel)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = archiveName
		header.Uid, header.Gid = 0, 0
		header.Uname, header.Gname = "root", "root"
		if info.IsDir() || rel == "CNC-Manager" || rel == "install.sh" || rel == "uninstall.sh" {
			header.Mode = 0o755
		} else {
			header.Mode = 0o644
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
