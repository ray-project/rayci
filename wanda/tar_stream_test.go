package wanda

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTarStreamContextOwner(t *testing.T) {
	tmp := t.TempDir()

	f := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ts := newTarStream()
	ts.owner = &contextOwner{UserID: 2000, GroupID: 100}
	ts.addFile("a/b/file.txt", nil, f)

	r := newWriterToReader(ts)
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, r); err != nil {
		t.Fatalf("copy tar stream: %v", err)
	}

	tr := tar.NewReader(buf)

	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("read file header: %v", err)
	}
	if hdr.Name != "a/b/file.txt" {
		t.Errorf("got name %q, want %q", hdr.Name, "a/b/file.txt")
	}
	if hdr.Uid != 2000 {
		t.Errorf("got uid %d, want %d", hdr.Uid, 2000)
	}
	if hdr.Gid != 100 {
		t.Errorf("got gid %d, want %d", hdr.Gid, 100)
	}

	d1, err := ts.digest()
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	ts.owner = &contextOwner{UserID: 3000, GroupID: 300}
	d2, err := ts.digest()
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if d1 == d2 {
		t.Error("digest should change when owner is modified")
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
