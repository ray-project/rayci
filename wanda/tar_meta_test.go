package wanda

import (
	"testing"

	"fmt"
	"os"
	"path/filepath"
)

func TestTarMetaFromFileInfo(t *testing.T) {
	tmp := t.TempDir()

	// Test to trigger only wanda changes.
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
