package wanda

import (
	"fmt"
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
	return files, err
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

// isFilePathGlob checks if the path looks like a glob patterh.
// The input is treated as a glob pattern if it contains a '*' or '?'.
func isFilePathGlob(s string) bool {
	return strings.Contains(s, "*") || strings.Contains(s, "?")
}

// cleanPath cleans the path, and also makes sure that it does
// not escape the root directory.
func cleanPath(s string) string {
	return strings.TrimPrefix(path.Clean(path.Join("/", s)), "/")
}

func listSrcFilesSingle(src string) ([]string, error) {
	if strings.HasSuffix(src, "/") { // a directory
		dir := cleanPath(strings.TrimSuffix(src, "/"))
		if dir == "" {
			dir = "."
		}
		return walkFilesInDir(filepath.FromSlash(dir))
	}

	// This might be a file or a pattern.
	srcClean := cleanPath(src)
	if srcClean == "" {
		return nil, fmt.Errorf("src %q is empty", src)
	}

	base := path.Base(srcClean)
	dir := path.Dir(srcClean)

	if !isFilePathGlob(base) {
		// Treat it as a file.
		return []string{filepath.FromSlash(srcClean)}, nil
	}

	// This is a glob pattern.
	osDir := filepath.FromSlash(dir)

	names, err := listFileNamesInDir(osDir)
	if err != nil {
		return nil, fmt.Errorf("list files in dir %q: %w", dir, err)
	}

	var files []string
	for _, name := range names {
		osName := filepath.Join(osDir, name)

		match, err := filepath.Match(base, name)
		if err != nil {
			return nil, fmt.Errorf("match file %q for %q: %w", osName, src, err)
		}
		if match {
			files = append(files, osName)
		}
	}
	return files, nil
}

func listSrcFiles(srcs []string, dockerFile string) ([]string, error) {
	fileMap := make(map[string]struct{})
	fileMap[dockerFile] = struct{}{}

	for _, src := range srcs {
		files, err := listSrcFilesSingle(src)
		if err != nil {
			return nil, fmt.Errorf("list src files for %q: %w", src, err)
		}
		for _, file := range files {
			fileMap[file] = struct{}{}
		}
	}

	var files []string
	for file := range fileMap {
		files = append(files, file)
	}
	sort.Strings(files)
	return files, nil
}
