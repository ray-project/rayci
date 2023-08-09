package wanda

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

type tarMeta struct {
	Mode    int64
	UserID  int
	GroupID int
}

func tarMetaFromFileInfo(info os.FileInfo) *tarMeta {
	const rootUser = 0
	return &tarMeta{
		Mode:    int64(info.Mode()) & 0777,
		UserID:  rootUser,
		GroupID: rootUser,
	}
}

type tarFile struct {
	name    string   // Name to write into the tar stream.
	srcFile string   // File to read from the file system.
	meta    *tarMeta // Metadata of the file.
}

func (f *tarFile) writeTo(tw *tar.Writer, modTime time.Time) error {
	src, err := os.Open(f.srcFile)
	if err != nil {
		return fmt.Errorf("open file %q: %w", f.srcFile, err)
	}
	defer src.Close()

	stat, err := src.Stat()
	if err != nil {
		return err
	}

	meta := f.meta
	if meta == nil {
		meta = tarMetaFromFileInfo(stat)
	}

	if err := tw.WriteHeader(&tar.Header{
		Name:    f.name,
		Size:    stat.Size(),
		Mode:    meta.Mode,
		Gid:     meta.GroupID,
		Uid:     meta.UserID,
		ModTime: modTime,
	}); err != nil {
		return fmt.Errorf("write header %q: %w", f.name, err)
	}

	if _, err := io.Copy(tw, src); err != nil {
		return err
	}

	return nil
}

type tarFileRecord struct {
	Name string
	Mode int64

	IsSymlink bool `json:",omitempty"`
	IsDir     bool `json:",omitempty"`

	Gid int `json:",omitempty"`
	Uid int `json:",omitempty"`

	Size    int64
	Content string
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
	contentDigest := fmt.Sprintf("sha256:%x", h.Sum(nil))

	meta := t.meta
	if meta == nil {
		meta = tarMetaFromFileInfo(stat)
	}

	h.Reset()

	r := &tarFileRecord{
		Name:    t.name,
		Mode:    meta.Mode,
		Gid:     meta.GroupID,
		Uid:     meta.UserID,
		Size:    stat.Size(),
		Content: contentDigest,
	}

	return r, nil
}

// tarStream is a stream of files that can be output as a tar stream.
// It implements io.WriterTo.
type tarStream struct {
	files map[string]*tarFile

	// all files will use this mod time default. This makes the stream
	// deterministic and cachable.
	modTime time.Time
}

// newTarStream creates a new tarball stream.
func newTarStream() *tarStream {
	return &tarStream{
		files:   make(map[string]*tarFile),
		modTime: DefaultTime,
	}
}

// DefaultTime is the default timestamp to use in all files for docker input.
// This makes the build deterministic and cachable.
var DefaultTime = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

// addFile adds a file to the tar stream. If meta is null, it will read the
// file from the file system to determin the mode, and use the root user as
// the user and group ID.
func (s *tarStream) addFile(name string, meta *tarMeta, src string) {
	s.files[name] = &tarFile{
		name:    name,
		srcFile: src,
		meta:    meta,
	}
}

func (s *tarStream) sortedNames() []string {
	var names []string
	for name := range s.files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *tarStream) writeTo(tw *tar.Writer) error {
	names := s.sortedNames()

	for _, name := range names {
		f := s.files[name]
		if err := f.writeTo(tw, s.modTime); err != nil {
			return fmt.Errorf("write file %q to stream", name)
		}
	}
	return nil
}

// WriteTo writes the entire stream out to w, implements io.WriterTo.
func (s *tarStream) WriteTo(w io.Writer) (int64, error) {
	cw := newCountingWriter(w)
	tw := tar.NewWriter(cw)

	writErr := s.writeTo(tw)
	closeErr := tw.Close() // Close flushes the tar stream, writting more bytes.
	if writErr != nil {
		return cw.n, writErr
	}
	return cw.n, closeErr
}

// digest calculates the digest of the content of input files.
func (s *tarStream) digest() (string, error) {
	// We have our own record format, so that the digest is controlled by
	// ourselves, and won't be affected by changes in the archive/tar package.

	h := sha256.New()
	names := s.sortedNames()

	enc := json.NewEncoder(h)

	for _, name := range names {
		f := s.files[name]

		r, err := f.record()
		if err != nil {
			return "", fmt.Errorf("digest file %q", name)
		}

		if err := enc.Encode(r); err != nil {
			return "", fmt.Errorf("write record for file %q", name)
		}
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}
