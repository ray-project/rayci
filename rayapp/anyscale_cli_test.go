package rayapp

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFakeAnyscale writes a fake anyscale script to a temp directory
// and returns its path. If script is empty, returns a path that does not exist.
func writeFakeAnyscale(t *testing.T, script string) string {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "anyscale")

	if script == "" {
		return bin // non-existent path
	}

	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake script: %v", err)
	}
	return bin
}
