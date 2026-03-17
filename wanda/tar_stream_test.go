package wanda

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTarStreamImplicitDirs(t *testing.T) {
	tmp := t.TempDir()

	f := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ts := newTarStream()
	ts.addFile("a/b/c/file.txt", nil, f)
	ts.addFile("a/other.txt", nil, f)

	dirs := ts.implicitDirs()
	want := []string{"a/", "a/b/", "a/b/c/"}
	if len(dirs) != len(want) {
		t.Fatalf("implicitDirs() = %v, want %v", dirs, want)
	}
	for i, d := range dirs {
		if d != want[i] {
			t.Errorf("implicitDirs()[%d] = %q, want %q", i, d, want[i])
		}
	}

	r := newWriterToReader(ts)
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, r); err != nil {
		t.Fatalf("copy tar stream: %v", err)
	}

	tr := tar.NewReader(buf)

	for _, wantDir := range want {
		hdr, err := tr.Next()
		if err != nil {
			t.Fatalf("read dir header for %q: %v", wantDir, err)
		}
		if hdr.Name != wantDir {
			t.Errorf("got name %q, want %q", hdr.Name, wantDir)
		}
		if hdr.Typeflag != tar.TypeDir {
			t.Errorf(
				"got typeflag %d for %q, want TypeDir (%d)",
				hdr.Typeflag,
				wantDir,
				tar.TypeDir,
			)
		}
		if hdr.Mode != 0755 {
			t.Errorf("got mode %o for %q, want %o", hdr.Mode, wantDir, 0755)
		}
		if hdr.Uid != 0 {
			t.Errorf("got uid %d for %q, want %d", hdr.Uid, wantDir, 0)
		}
		if hdr.Gid != 0 {
			t.Errorf("got gid %d for %q, want %d", hdr.Gid, wantDir, 0)
		}
	}

	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("read file header: %v", err)
	}
	if hdr.Name != "a/b/c/file.txt" {
		t.Errorf("got name %q, want %q", hdr.Name, "a/b/c/file.txt")
	}

	hdr, err = tr.Next()
	if err != nil {
		t.Fatalf("read file header: %v", err)
	}
	if hdr.Name != "a/other.txt" {
		t.Errorf("got name %q, want %q", hdr.Name, "a/other.txt")
	}
}

func TestTarStreamImplicitDirsNone(t *testing.T) {
	ts := newTarStream()

	tmp := t.TempDir()
	f := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ts.addFile("flat-file", nil, f)
	dirs := ts.implicitDirs()
	if len(dirs) != 0 {
		t.Errorf("implicitDirs() = %v, want empty", dirs)
	}
}

func TestTarStream(t *testing.T) {
	tmp := t.TempDir()

	f := filepath.Join(tmp, "testfile")
	data := []byte("hello world")
	if err := os.WriteFile(f, data, 0644); err != nil {
		t.Fatalf("write file %q: %v", f, err)
	}

	f2 := filepath.Join(tmp, "testfile2")
	data2 := []byte("Hello world!") // a different content
	if err := os.WriteFile(f2, data2, 0644); err != nil {
		t.Fatalf("write file %q: %v", f2, err)
	}

	ts := newTarStream()
	ts.addFile("userfile", nil, f)

	digest, err := ts.digest()
	if err != nil {
		t.Fatalf("digest tar stream: %v", err)
	}

	ts.addFile("userfile2", nil, f2)

	digest2, err := ts.digest()
	if err != nil {
		t.Fatalf("digest tar stream: %v", err)
	}

	if digest == digest2 {
		t.Errorf(
			"got same digest after adding file: %q vs %q",
			digest, digest2,
		)
	}

	r := newWriterToReader(ts)
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, r); err != nil {
		t.Fatalf("copy tar stream: %v", err)
	}

	tr := tar.NewReader(buf)

	header, err := tr.Next()
	if err != nil {
		t.Fatalf("read tar header file 1: %v", err)
	}
	// File names in context are always sorted.
	if header.Name != "userfile" {
		t.Errorf("got name %q, want %q", header.Name, "userfile")
	}
	if header.Mode != 0644 {
		t.Errorf("got mode %o, want %o", header.Mode, 0644)
	}

	content, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("read tar content: %v", err)
	}
	if !bytes.Equal(content, data) {
		t.Errorf("got content %q, want %q", content, data)
	}

	// Next file.
	header, err = tr.Next()
	if err != nil {
		t.Fatalf("read tar header file 2: %v", err)
	}
	if header.Name != "userfile2" {
		t.Errorf("got name %q, want %q", header.Name, "userfile2")
	}

	content, err = io.ReadAll(tr)
	if err != nil {
		t.Fatalf("read tar content: %v", err)
	}
	if !bytes.Equal(content, data2) {
		t.Errorf("got content %q, want %q", content, data2)
	}
}
