package wanda

import (
	"testing"

	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

func TestTarFile(t *testing.T) {
	tmp := t.TempDir()

	f := filepath.Join(tmp, "testfile")
	data := []byte("hello world")
	if err := os.WriteFile(f, data, 0644); err != nil {
		t.Fatalf("write file %q: %v", f, err)
	}

	tf := &tarFile{
		name:    "userfile", // change the name
		srcFile: f,
		meta: &tarMeta{
			Mode:    0600, // uses a different mode from the file on disk.
			UserID:  2000, // and assign a special user
			GroupID: 2000,
		},
	}

	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	// Tarball only stores the mod time with second precision.
	modTime := time.Now().Truncate(time.Second)

	if err := tf.writeTo(tw, modTime); err != nil {
		t.Fatalf("write to tar stream: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar stream: %v", err)
	}

	tr := tar.NewReader(tarBuf)
	header, err := tr.Next()
	if err != nil {
		t.Fatalf("read tar header: %v", err)
	}
	if header.Name != "userfile" {
		t.Errorf("got name %q, want %q", header.Name, "userfile")
	}
	if header.Mode != 0600 {
		t.Errorf("got mode %o, want %o", header.Mode, 0600)
	}
	if header.Uid != 2000 {
		t.Errorf("got uid %d, want %d", header.Uid, 2000)
	}
	if header.Gid != 2000 {
		t.Errorf("got gid %d, want %d", header.Gid, 2000)
	}
	if header.ModTime != modTime {
		t.Errorf("got mod time %v, want %v", header.ModTime, modTime)
	}

	content, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("read tar content: %v", err)
	}
	if !bytes.Equal(content, data) {
		t.Errorf("got content %q, want %q", content, data)
	}
}

func TestTarFileRecord(t *testing.T) {
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

	tf := &tarFile{
		name:    "userfile", // change the name
		srcFile: f,
		meta: &tarMeta{
			Mode:    0600, // uses a different mode from the file on disk.
			UserID:  2000, // and assign a special user
			GroupID: 1000,
		},
	}

	r, err := tf.record()
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	bs, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	loopback := new(tarFileRecord)
	if err := json.Unmarshal(bs, loopback); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	if !reflect.DeepEqual(r, loopback) {
		t.Errorf("got loopback record %+v, want %+v", loopback, r)
	}

	if r.Name != "userfile" {
		t.Errorf("got name %q, want %q", r.Name, "userfile")
	}
	if r.Mode != 0600 {
		t.Errorf("got mode %o, want %o", r.Mode, 0600)
	}
	if r.UserID != 2000 {
		t.Errorf("got uid %d, want %d", r.UserID, 2000)
	}
	if r.GroupID != 1000 {
		t.Errorf("got gid %d, want %d", r.GroupID, 1000)
	}
	if r.Size != int64(len(data)) {
		t.Errorf("got size %d, want %d", r.Size, len(data))
	}
	if r.ContentDigest == "" {
		t.Errorf("got empty content digest")
	}

	tf2 := &tarFile{
		name:    "userfile", // change the name
		srcFile: f2,
		meta:    tf.meta, // The same meta as the first file.
	}

	r2, err := tf2.record()
	if err != nil {
		t.Fatalf("record for file 2: %v", err)
	}
	if r2.ContentDigest == r.ContentDigest {
		t.Errorf(
			"got the same content digest for different content: %q vs %q",
			r2.ContentDigest, r.ContentDigest,
		)
	}

}
