package wanda

import (
	"archive/tar"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"
)

type tarFile struct {
	name    string // Name to write into the tar stream.
	srcFile string // File to read from the file system.

	// Metadata of the file. When it is nil, use the file system to
	// determine the metadata.
	meta *tarMeta
}

func (t *tarFile) writeTo(tw *tar.Writer, modTime time.Time) error {
	src, err := os.Open(t.srcFile)
	if err != nil {
		return fmt.Errorf("open file %q: %w", t.srcFile, err)
	}
	defer src.Close()

	stat, err := src.Stat()
	if err != nil {
		return err
	}

	meta := t.meta
	if meta == nil {
		meta = tarMetaFromFileInfo(stat)
	}

	if err := tw.WriteHeader(&tar.Header{
		Name:    t.name,
		Size:    stat.Size(),
		Mode:    meta.Mode,
		Gid:     meta.GroupID,
		Uid:     meta.UserID,
		ModTime: modTime,
	}); err != nil {
		return fmt.Errorf("write header %q: %w", t.name, err)
	}

	if _, err := io.Copy(tw, src); err != nil {
		return err
	}

	return nil
}

// tarFileRecord is a record for a tar file. It is meant to be encoded into
// JSON for calculating the digest of the build input.
type tarFileRecord struct {
	Name string `json:"name"`
	Mode int64  `json:"mode"`

	// TODO(aslonnie): add fields and support for dir, hard-links and symlinks.

	GroupID int `json:"gid,omitempty"`
	UserID  int `json:"uid,omitempty"`

	Size int64 `json:"size"`

	ContentDigest string `json:"content"`
}

func (t *tarFile) record() (*tarFileRecord, error) {
	f, err := os.Open(t.srcFile)
	if err != nil {
		return nil, fmt.Errorf("open file %q: %w", t.srcFile, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("read file %q: %w", t.srcFile, err)
	}
	contentDigest := sha256DigestString(h)

	meta := t.meta
	if meta == nil {
		meta = tarMetaFromFileInfo(stat)
	}

	h.Reset()

	r := &tarFileRecord{
		Name:    t.name,
		Mode:    meta.Mode,
		GroupID: meta.GroupID,
		UserID:  meta.UserID,
		Size:    stat.Size(),

		ContentDigest: contentDigest,
	}

	return r, nil
}
