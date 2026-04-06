package wanda

import (
	"testing"

	"fmt"
	"os"
	"path/filepath"
)

func TestParseContextOwner(t *testing.T) {
	owner, err := parseContextOwner("2000:100")
	if err != nil {
		t.Fatalf("parseContextOwner: %v", err)
	}
	if owner.UserID != 2000 {
		t.Errorf("got uid %d, want %d", owner.UserID, 2000)
	}
	if owner.GroupID != 100 {
		t.Errorf("got gid %d, want %d", owner.GroupID, 100)
	}
}

func TestParseContextOwnerInvalid(t *testing.T) {
	for _, input := range []string{
		"", "2000", "abc:100", "2000:xyz",
		"-1:100", "100:-1", "100:200:300", " 100:200",
	} {
		_, err := parseContextOwner(input)
		if err == nil {
			t.Errorf(
				"parseContextOwner(%q) = nil error, want error",
				input,
			)
		}
	}
}

func TestTarMetaFromFileInfo(t *testing.T) {
	tmp := t.TempDir()

	for _, mod := range []int64{
		0755, // bazel's sandbox umask does not support 777.
		0644,
		0600,
		0400,
		0700,
	} {
		path := filepath.Join(tmp, fmt.Sprintf("file-%d", mod))
		data := []byte(fmt.Sprintf("%d", mod))
		if err := os.WriteFile(path, data, os.FileMode(mod)); err != nil {
			t.Fatalf("write file %q: %v", path, err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %q: %v", path, err)
		}

		meta := tarMetaFromFileInfo(info)
		if meta.Mode != mod {
			t.Errorf("got mode %o, want %o", meta.Mode, mod)
		}
		if meta.GroupID != 0 {
			t.Errorf("got group id %d, want 0", meta.GroupID)
		}
		if meta.UserID != 0 {
			t.Errorf("got user id %d, want 0", meta.UserID)
		}
	}
}
