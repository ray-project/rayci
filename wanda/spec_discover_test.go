package wanda

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverSpecs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create specs in different directories
	baseDir := filepath.Join(tmpDir, "base")
	appDir := filepath.Join(tmpDir, "app")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeSpec(t, baseDir, "base.wanda.yaml", strings.Join([]string{
		"name: base-image",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, appDir, "app.wanda.yaml", strings.Join([]string{
		"name: app-image",
		"dockerfile: Dockerfile",
	}, "\n"))

	index, err := discoverSpecs(tmpDir, noopLookup)
	if err != nil {
		t.Fatalf("discoverSpecs: %v", err)
	}

	if len(index) != 2 {
		t.Errorf("index has %d entries, want 2", len(index))
	}

	if entry, ok := index["base-image"]; !ok {
		t.Error("index missing base-image")
	} else if !strings.HasSuffix(entry.Path, "base.wanda.yaml") {
		t.Errorf("base-image path = %q, want suffix base.wanda.yaml", entry.Path)
	}

	if entry, ok := index["app-image"]; !ok {
		t.Error("index missing app-image")
	} else if !strings.HasSuffix(entry.Path, "app.wanda.yaml") {
		t.Errorf("app-image path = %q, want suffix app.wanda.yaml", entry.Path)
	}
}

func TestDiscoverSpecs_NameCollision(t *testing.T) {
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	// Two specs with same name
	writeSpec(t, dir1, "a.wanda.yaml", strings.Join([]string{
		"name: same-name",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, dir2, "b.wanda.yaml", strings.Join([]string{
		"name: same-name",
		"dockerfile: Dockerfile",
	}, "\n"))

	_, err := discoverSpecs(tmpDir, noopLookup)
	if err == nil {
		t.Fatal("expected error for name collision, got nil")
	}

	if !strings.Contains(err.Error(), "same-name") {
		t.Errorf("error should mention conflicting name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "multiple") {
		t.Errorf("error should mention 'multiple', got: %v", err)
	}
}

func TestDiscoverSpecs_WithVariables(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "base.wanda.yaml", strings.Join([]string{
		"name: base-$VERSION",
		"dockerfile: Dockerfile",
	}, "\n"))

	lookup := func(key string) (string, bool) {
		if key == "VERSION" {
			return "1.0", true
		}
		return "", false
	}

	index, err := discoverSpecs(tmpDir, lookup)
	if err != nil {
		t.Fatalf("discoverSpecs: %v", err)
	}

	if _, ok := index["base-1.0"]; !ok {
		t.Errorf("index missing expanded name base-1.0, got: %v", index)
	}
}
