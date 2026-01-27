package goqualgate

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileLengthConfig holds settings for file length checking.
type FileLengthConfig struct {
	MaxLines int
}

// Run executes the file length check.
func (cfg FileLengthConfig) Run() error {
	files, err := findGoFiles()
	if err != nil {
		return fmt.Errorf("find go files: %w", err)
	}

	if len(files) == 0 {
		fmt.Printf("No Go files found\n")
		return nil
	}

	sort.Strings(files)

	fmt.Printf("=== File Length Results (max %d) ===\n", cfg.MaxLines)

	var failures []string
	for _, file := range files {
		lines, err := countLines(file)
		if err != nil {
			return fmt.Errorf("count lines %s: %w", file, err)
		}

		if lines > cfg.MaxLines {
			over := lines - cfg.MaxLines
			fmt.Printf("FAIL  %s: %d lines (%d over)\n", file, lines, over)
			failures = append(failures, file)
		}
	}

	if len(failures) > 0 {
		fmt.Printf("FAILED: %d file(s) exceed limit\n", len(failures))
		return fmt.Errorf("file length check failed")
	}
	fmt.Printf("PASSED: %d file(s) checked\n", len(files))
	return nil
}

func findGoFiles() ([]string, error) {
	var files []string

	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor directory
		if d.IsDir() && d.Name() == "vendor" {
			return filepath.SkipDir
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip symlinks
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		// Only .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}
