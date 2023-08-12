package wanda

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

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

	return sha256DigestString(h), nil
}
