package goqualgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name:    "empty file",
			content: "",
			want:    0,
		},
		{
			name:    "single line no newline",
			content: "hello",
			want:    1,
		},
		{
			name:    "single line with newline",
			content: "hello\n",
			want:    1,
		},
		{
			name:    "multiple lines",
			content: strings.Join([]string{"line1", "line2", "line3"}, "\n"),
			want:    3,
		},
		{
			name:    "multiple lines with trailing newline",
			content: strings.Join([]string{"line1", "line2", "line3", ""}, "\n"),
			want:    3,
		},
		{
			name:    "blank lines count",
			content: strings.Join([]string{"line1", "", "line3", ""}, "\n"),
			want:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "test.go")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write temp file: %v", err)
			}

			got, err := countLines(path)
			if err != nil {
				t.Fatalf("countLines() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("countLines() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountLinesNonExistent(t *testing.T) {
	_, err := countLines("/nonexistent/path/file.go")
	if err == nil {
		t.Error("countLines() expected error for non-existent file")
	}
}

func TestFileLengthConfigRun(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]int // filename -> line count
		maxLines int
		wantErr  bool
	}{
		{
			name:     "all files under limit",
			files:    map[string]int{"small.go": 100, "medium.go": 200},
			maxLines: 400,
			wantErr:  false,
		},
		{
			name:     "file over limit",
			files:    map[string]int{"small.go": 100, "large.go": 500},
			maxLines: 400,
			wantErr:  true,
		},
		{
			name:     "no go files",
			files:    map[string]int{},
			maxLines: 400,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create test files
			for name, lines := range tt.files {
				content := strings.Repeat("x\n", lines)
				path := filepath.Join(tmpDir, name)
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("write temp file: %v", err)
				}
			}

			// Change to temp directory to run
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("get working directory: %v", err)
			}
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("change to temp directory: %v", err)
			}
			defer os.Chdir(oldWd)

			cfg := FileLengthConfig{
				MaxLines: tt.maxLines,
			}

			err = cfg.Run()
			if (err != nil) != tt.wantErr {
				t.Errorf("FileLengthConfig.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFindGoFilesExclusions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create various files
	files := []string{
		"main.go",
		"util.go",
		"main_test.go",
		"util_gen.go",
		"proto.pb.go",
		"types_generated.go",
		"vendor/lib/lib.go",
		"sub/sub.go",
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("package x\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("change to temp directory: %v", err)
	}
	defer os.Chdir(oldWd)

	got, err := findGoFiles()
	if err != nil {
		t.Fatalf("findGoFiles() error = %v", err)
	}

	// Should exclude test files and vendor directory
	want := []string{"main.go", "proto.pb.go", "sub/sub.go", "types_generated.go", "util.go", "util_gen.go"}
	if len(got) != len(want) {
		t.Errorf("findGoFiles() = %v, want %v", got, want)
		return
	}
	for i, f := range want {
		if got[i] != f {
			t.Errorf("findGoFiles()[%d] = %q, want %q", i, got[i], f)
		}
	}
}
