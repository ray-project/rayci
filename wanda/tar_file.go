package wanda

import (
	"archive/tar"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
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
	stat, err := os.Lstat(t.srcFile)
	if err != nil {
		return fmt.Errorf("stat file %q: %w", t.srcFile, err)
	}

	meta := t.meta
	if meta == nil {
		meta = tarMetaFromFileInfo(stat)
	}

	hdr := &tar.Header{
		Name:    t.name,
		Mode:    meta.Mode,
		Gid:     meta.GroupID,
		Uid:     meta.UserID,
		ModTime: modTime,
	}

	switch stat.Mode() & os.ModeType {
	case os.ModeSymlink:
		target, err := os.Readlink(t.srcFile)
		if err != nil {
			return fmt.Errorf("read symlink %q: %w", t.srcFile, err)
		}

		hdr.Typeflag = tar.TypeSymlink
		hdr.Linkname = target

		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write symlink header %q: %w", t.name, err)
		}
		return nil
	default:
		if !stat.Mode().IsRegular() {
			log.Printf("WARNING: unexpected file type for %q: %v", t.srcFile, stat.Mode())
		}

		src, err := os.Open(t.srcFile)
		if err != nil {
			return fmt.Errorf("open file %q: %w", t.srcFile, err)
		}
		defer src.Close()

		hdr.Size = stat.Size()
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write header %q: %w", t.name, err)
		}

		if _, err := io.Copy(tw, src); err != nil {
			return fmt.Errorf("copy %q into tar: %w", t.srcFile, err)
		}

		return nil
	}
}

// tarFileRecord is a record for a tar file. It is meant to be encoded into
// JSON for calculating the digest of the build input.
type tarFileRecord struct {
	Name string `json:"name"`
	Mode int64  `json:"mode"`

	// TODO(aslonnie): add fields and support for dir and hard-links.

	GroupID int `json:"gid,omitempty"`
	UserID  int `json:"uid,omitempty"`

	Size int64 `json:"size"`

	ContentDigest string `json:"content"`

	Symlink string `json:"symlink,omitempty"`
}

func (t *tarFile) record() (*tarFileRecord, error) {
	// Use Lstat to detect symlinks without following them.
	stat, err := os.Lstat(t.srcFile)
	if err != nil {
		return nil, fmt.Errorf("stat file %q: %w", t.srcFile, err)
	}

	meta := t.meta
	if meta == nil {
		meta = tarMetaFromFileInfo(stat)
	}

	switch stat.Mode() & os.ModeType {
	case os.ModeSymlink:
		target, err := os.Readlink(t.srcFile)
		if err != nil {
			return nil, fmt.Errorf("read symlink %q: %w", t.srcFile, err)
		}

		// For symlinks, digest the target path.
		h := sha256.New()
		h.Write([]byte(target))
		contentDigest := sha256DigestString(h)

		return &tarFileRecord{
			Name:          t.name,
			Mode:          meta.Mode,
			GroupID:       meta.GroupID,
			UserID:        meta.UserID,
			Size:          0, // Symlinks have no content size in tar.
			ContentDigest: contentDigest,
			Symlink:       target,
		}, nil
	default:
		f, err := os.Open(t.srcFile)
		if err != nil {
			return nil, fmt.Errorf("open file %q: %w", t.srcFile, err)
		}
		defer f.Close()

		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return nil, fmt.Errorf("read file %q: %w", t.srcFile, err)
		}
		contentDigest := sha256DigestString(h)

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
}
