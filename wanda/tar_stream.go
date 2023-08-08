package wanda

import (
	"time"
)

// TarMeta contains the metadata of a tar file.
type TarMeta struct {
	Mode    int64
	UserID  int
	GroupID int
}

type tarFile struct {
	name    string   // Name to write into the tar stream.
	srcFile string   // File to read from the file system.
	meta    *TarMeta // Metadata of the file.
}

// TarStream is a stream of files that can be output as a tar stream.
// It implements io.WriterTo.
type TarStream struct {
	files   []*tarFile
	modTime time.Time
}

// DefaultTime is the default timestamp to use in all files for docker input.
// This makes the build deterministic and cachable.
var DefaultTime = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

func (s *TarStream) AddFile(name string, meta *TarMeta, src string) {
	s.files = append(s.files, &tarFile{
		name:    name,
		srcFile: src,
		meta:    meta,
	})
}
