package wanda

import (
	"fmt"
	"path/filepath"
	"strings"
)

// buildExtractionScript generates a shell script to extract artifacts from a
// built image to the artifacts directory.
//
// Example outputs:
//   - File: mkdir -p /artifacts/bin && cp -r /build/out.bin /artifacts/bin/out.bin
//   - Glob: mkdir -p /artifacts/wheels && cp -r /build/dist/*.whl /artifacts/wheels/
//   - Optional: mkdir -p /artifacts/doc && cp -r /docs /artifacts/doc || echo 'warning: ...'
//
// Returns an error if any artifact path would escape the artifacts directory.
func buildExtractionScript(artifacts []*Artifact, hostArtifactsDir string) (string, error) {
	var lines []string

	for _, a := range artifacts {
		dst, err := resolveDst(a.Dst, hostArtifactsDir)
		if err != nil {
			return "", fmt.Errorf("artifact %q: %w", a.Dst, err)
		}

		// Determine which directory needs to be pre-created.
		// If Dst ends with '/', the user expects the path itself to be the directory.
		mkdirPath := filepath.Dir(dst)
		if strings.HasSuffix(a.Dst, "/") {
			mkdirPath = dst
		}

		// Critical: Do not quote a.Src if it contains globs, otherwise
		// the shell won't expand them.
		srcParam := a.Src
		if !hasGlobChars(a.Src) {
			srcParam = shellQuote(a.Src)
		}

		cmd := fmt.Sprintf("mkdir -p %s && cp -r %s %s",
			shellQuote(mkdirPath),
			srcParam,
			shellQuote(dst),
		)

		if a.Optional {
			cmd += fmt.Sprintf(" || echo 'warning: optional artifact not found: %s'", a.Src)
		}

		lines = append(lines, cmd)
	}

	return strings.Join(lines, "\n"), nil
}

// resolveDst determines the final path under /artifacts (the container mount point).
// hostArtifactsDir is the host-side artifacts directory, used only to resolve
// absolute paths that might reference it. For example, if hostArtifactsDir is
// "/home/user/output" and dst is "/home/user/output/bin/app", the result is
// "/artifacts/bin/app".
// Returns an error if the path would escape /artifacts.
func resolveDst(dst, hostArtifactsDir string) (string, error) {
	rel := dst
	if filepath.IsAbs(dst) {
		// If it's absolute, try to make it relative to hostArtifactsDir.
		if r, err := filepath.Rel(hostArtifactsDir, dst); err == nil && !strings.HasPrefix(r, "..") {
			rel = r
		} else {
			// If it's absolute but outside hostArtifactsDir, flatten to the filename.
			rel = filepath.Base(dst)
		}
	}

	result := filepath.Join("/artifacts", rel)

	// Verify the resolved path stays within /artifacts
	if !strings.HasPrefix(result, "/artifacts") {
		return "", fmt.Errorf("path %q escapes artifacts directory", dst)
	}

	return result, nil
}

// hasGlobChars returns true if the path contains glob metacharacters.
func hasGlobChars(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
