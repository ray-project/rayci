package wanda

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// walkFilesInDir recursively walks the files in the given directory.
// It returns a list of files in the directory.
// It does not follow symlinks, and it does not return directories.
func walkFilesInDir(dir string) ([]string, error) {
	var files []string
	dirfs := os.DirFS(dir)

	err := fs.WalkDir(dirfs, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, filepath.Join(dir, p))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// listFileNamesInDir lists the files in the given directory.
// It lists symlinks as files; it does not return directories.
func listFileNamesInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// isFilePathGlob checks if the path looks like a glob pattern.
// The input is treated as a glob pattern if it contains a '*' or '?'.
func isFilePathGlob(s string) bool {
	return strings.Contains(s, "*") || strings.Contains(s, "?")
}

// cleanPath cleans the path, and also makes sure that it does
// not escape the root directory.
func cleanPath(s string) string {
	return strings.TrimPrefix(path.Clean(path.Join("/", s)), "/")
}
