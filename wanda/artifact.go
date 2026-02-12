package wanda

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Artifact defines a file or directory to extract from a built image.
type Artifact struct {
	// Src is the path inside the container to extract.
	// Can be a file or directory. Must be an absolute path.
	Src string `yaml:"src"`

	// Dst is the destination path relative to the artifacts directory.
	// Must be a relative path (no ".." allowed).
	Dst string `yaml:"dst"`

	// Optional marks this artifact as best-effort.
	// If true, extraction failure will be logged but won't fail the build.
	Optional bool `yaml:"optional,omitempty"`
}

// Validate checks that the artifact paths are safe.
// Src must be an absolute path (inside the container).
// Dst must be relative and cannot escape the artifacts directory.
func (a *Artifact) Validate() error {
	if !strings.HasPrefix(a.Src, "/") {
		return fmt.Errorf("artifact src must be absolute path: %q", a.Src)
	}

	if filepath.IsAbs(a.Dst) {
		return fmt.Errorf("artifact dst must be relative path: %q", a.Dst)
	}

	cleaned := filepath.Clean(a.Dst)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return fmt.Errorf("artifact dst cannot escape artifacts directory: %q", a.Dst)
	}

	return nil
}

// ResolveDst resolves the destination path for the artifact.
// Returns an error if the resolved path would escape the artifacts directory.
func (a *Artifact) ResolveDst(artifactsDir string) (string, error) {
	if filepath.IsAbs(a.Dst) {
		return "", fmt.Errorf("artifact dst must be relative path: %q", a.Dst)
	}

	resolved := filepath.Join(artifactsDir, a.Dst)
	resolved = filepath.Clean(resolved)

	absArtifactsDir, err := filepath.Abs(artifactsDir)
	if err != nil {
		return "", fmt.Errorf("resolve artifacts dir: %w", err)
	}
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve dst: %w", err)
	}

	if !strings.HasPrefix(absResolved, absArtifactsDir+string(filepath.Separator)) &&
		absResolved != absArtifactsDir {
		return "", fmt.Errorf("artifact dst escapes artifacts directory: %q", a.Dst)
	}

	return resolved, nil
}
