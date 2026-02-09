package rayapp

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func checkIsDir(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

// Use a fixed build time to make the zip file deterministic.
// We cannot set use t=0 unix epoch time, because when the
// time zone of the machine is different from UTC, some systems
// can complain about file timestamp being invalid and
// unsupported.
var frozenTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func addToZip(z *zip.Writer, r io.Reader, pathInZip string) error {
	h := &zip.FileHeader{
		Name:     pathInZip,
		Method:   zip.Deflate,
		Modified: frozenTime,
	}

	w, err := z.CreateHeader(h)
	if err != nil {
		return fmt.Errorf("create file in zip: %w", err)
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return nil
}

func addFileToZip(z *zip.Writer, file, pathInZip string) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("open file %q: %w", file, err)
	}
	defer f.Close()

	return addToZip(z, f, pathInZip)
}

type zipFile struct {
	// Path to use in the zip file. When srcFilePath is empty,
	// the same file path will be used for finding the source file.
	path string

	// Optional. If set, the content will be read from this reader.
	rc io.ReadCloser

	// Optional. If set, the content will be read from this file.
	// If rc is set, this field is ignored.
	srcFilePath string
}

func buildZip(srcDir string, files []*zipFile, out string) error {
	outFile, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create release zip file: %w", err)
	}
	defer outFile.Close()

	z := zip.NewWriter(outFile)
	for _, f := range files {
		if f.rc == nil {
			src := f.srcFilePath
			if src == "" {
				src = filepath.Join(srcDir, filepath.FromSlash(f.path))
			}
			if err := addFileToZip(z, src, f.path); err != nil {
				return fmt.Errorf("add file %q to zip: %w", f, err)
			}
		} else {
			if err := addToZip(z, f.rc, f.path); err != nil {
				return fmt.Errorf("add %q to zip: %w", f.path, err)
			}
		}
	}
	if err := z.Close(); err != nil {
		return fmt.Errorf("close zip writer: %w", err)
	}
	if err := outFile.Sync(); err != nil {
		return fmt.Errorf("flush zip file to storage: %w", err)
	}
	return nil
}

// zipDirectory creates a zip file containing all files from the source directory.
// The files are stored with paths relative to the source directory.
// outPath is the path where the zip file will be created.
func zipDirectory(srcDir, outPath string) (err error) {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create zip file: %w", err)
	}
	defer outFile.Close()

	z := zip.NewWriter(outFile)

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories - they're created implicitly by file paths
		if info.IsDir() {
			return nil
		}

		// Get the relative path for the zip entry
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("get relative path: %w", err)
		}

		if err := addFileToZip(z, path, relPath); err != nil {
			return fmt.Errorf("add file %q to zip: %w", relPath, err)
		}

		return nil
	})

	if err != nil {
		z.Close()
		return fmt.Errorf("walk directory: %w", err)
	}

	if err := z.Close(); err != nil {
		return fmt.Errorf("close zip writer: %w", err)
	}

	if err := outFile.Sync(); err != nil {
		return fmt.Errorf("flush zip file to storage: %w", err)
	}

	return nil
}
