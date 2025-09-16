package wanda

import (
	"testing"

	"os"
	"path/filepath"
	"reflect"
)

func TestWalkFilesInDir_empty(t *testing.T) {
	tmpDir := t.TempDir()

	files, err := walkFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("walkFilesInDir failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, want none", len(files))
	}
}

func TestWalkFilesInDir_singleFile(t *testing.T) {
	tmpDir := t.TempDir()

	file := filepath.Join(tmpDir, "file.txt")

	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	files, err := walkFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("walkFilesInDir failed: %v", err)
	}

	want := []string{file}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("got %v, want %v", files, want)
	}
}

func TestWalkFilesInDir_recursive(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{
		"1.txt",
		"sub/2.txt",
		"sub/subsub/3.txt",
	}
	for _, file := range files {
		dir := filepath.Join(tmpDir, filepath.Dir(file))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir for %q: %v", file, err)
		}

		if err := os.WriteFile(filepath.Join(tmpDir, file), []byte(file), 0644); err != nil {
			t.Fatalf("write file %q: %v", file, err)
		}
	}

	got, err := walkFilesInDir(tmpDir)
	if err != nil {
		t.Fatalf("walkFilesInDir failed: %v", err)
	}

	want := []string{
		filepath.Join(tmpDir, "1.txt"),
		filepath.Join(tmpDir, "sub/2.txt"),
		filepath.Join(tmpDir, "sub/subsub/3.txt"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestListFileNamesInDir(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{
		"1.txt",
		"2.txt",
		"sub/3.txt",
		"sub/5.txt",
		"sub/subsub/4.txt",
	}
	for _, file := range files {
		dir := filepath.Join(tmpDir, filepath.Dir(file))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir for %q: %v", file, err)
		}

		if err := os.WriteFile(filepath.Join(tmpDir, file), []byte(file), 0644); err != nil {
			t.Fatalf("write file %q: %v", file, err)
		}
	}

	if err := os.Symlink(
		filepath.Join(tmpDir, "1.txt"),
		filepath.Join(tmpDir, "link.txt"),
	); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	got, err := listFileNamesInDir(tmpDir)
	if err != nil {
		t.Fatalf("listFileNamesInDir failed: %v", err)
	}

	want := []string{
		"1.txt",
		"2.txt",
		"link.txt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIsFilePathGlob(t *testing.T) {
	for _, test := range []struct {
		in   string
		want bool
	}{
		{"", false},
		{"*.txt", true},
		{"a?b.yaml", true},
		{"abc.yaml", false},
	} {
		got := isFilePathGlob(test.in)
		if got != test.want {
			t.Errorf("isFilePathGlob(%q): got %v, want %v", test.in, got, test.want)
		}
	}
}

func TestCleanPath(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{".", ""},
		{"..", ""},
		{"/", ""},
		{"/a", "a"},
		{"a/../../../b", "b"}, // cannot escape root
	} {
		got := cleanPath(test.in)
		if got != test.want {
			t.Errorf("cleanPath(%q): got %v, want %v", test.in, got, test.want)
		}
	}
}
