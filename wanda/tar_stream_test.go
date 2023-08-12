package wanda

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

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
