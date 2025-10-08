package wanda

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPredictCacheHit_DisabledCaching(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a simple wanda spec with caching disabled
	specContent := `name: test
dockerfile: Dockerfile
disable_caching: true
`
	specFile := filepath.Join(tmpDir, "test.wanda.yaml")
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create a simple Dockerfile
	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM alpine\n"), 0644); err != nil {
		t.Fatalf("failed to write dockerfile: %v", err)
	}

	config := &ForgeConfig{
		WorkDir:  tmpDir,
		WorkRepo: "test.example.com/repo",
		Epoch:    "test",
		RayCI:    true,
	}

	cacheHit, err := PredictCacheHit(specFile, config)
	if err != nil {
		t.Fatalf("PredictCacheHit failed: %v", err)
	}

	if cacheHit {
		t.Error("expected cache hit to be false when caching is disabled")
	}
}

func TestPredictCacheHit_LocalOnly(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a simple wanda spec
	specContent := `name: test
dockerfile: Dockerfile
`
	specFile := filepath.Join(tmpDir, "test.wanda.yaml")
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create a simple Dockerfile
	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM alpine\n"), 0644); err != nil {
		t.Fatalf("failed to write dockerfile: %v", err)
	}

	// Config without WorkRepo (local only)
	config := &ForgeConfig{
		WorkDir: tmpDir,
		Epoch:   "test",
	}

	cacheHit, err := PredictCacheHit(specFile, config)
	if err != nil {
		t.Fatalf("PredictCacheHit failed: %v", err)
	}

	if cacheHit {
		t.Error("expected cache hit to be false for local-only builds")
	}
}

func TestPredictCacheHit_RebuildForced(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a simple wanda spec
	specContent := `name: test
dockerfile: Dockerfile
`
	specFile := filepath.Join(tmpDir, "test.wanda.yaml")
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Create a simple Dockerfile
	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM alpine\n"), 0644); err != nil {
		t.Fatalf("failed to write dockerfile: %v", err)
	}

	config := &ForgeConfig{
		WorkDir:  tmpDir,
		WorkRepo: "test.example.com/repo",
		Epoch:    "test",
		RayCI:    true,
		Rebuild:  true,
	}

	cacheHit, err := PredictCacheHit(specFile, config)
	if err != nil {
		t.Fatalf("PredictCacheHit failed: %v", err)
	}

	if cacheHit {
		t.Error("expected cache hit to be false when rebuild is forced")
	}
}
