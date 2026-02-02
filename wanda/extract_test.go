package wanda

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDst(t *testing.T) {
	tests := []struct {
		dst     string
		baseDir string
		want    string
	}{
		// Relative paths go under /artifacts (container mount point)
		{"bin/myapp", "/tmp/artifacts", "/artifacts/bin/myapp"},
		{"wheels/", "/tmp/artifacts", "/artifacts/wheels"},
		{"output.txt", "/tmp/artifacts", "/artifacts/output.txt"},

		// Relative paths with globs
		{"dist/*.whl", "/tmp/artifacts", "/artifacts/dist/*.whl"},
		{"conf/", "/tmp/artifacts", "/artifacts/conf"},

		// Absolute paths within baseDir use relative portion under /artifacts
		{"/tmp/artifacts/bin/myapp", "/tmp/artifacts", "/artifacts/bin/myapp"},
		{"/tmp/artifacts/sub/file.txt", "/tmp/artifacts", "/artifacts/sub/file.txt"},

		// Absolute paths outside baseDir use basename under /artifacts
		{"/other/path/file.txt", "/tmp/artifacts", "/artifacts/file.txt"},
		{"/etc/passwd", "/tmp/artifacts", "/artifacts/passwd"},

		// Absolute paths with globs outside baseDir use basename
		{"/build/dist/*.whl", "/tmp/artifacts", "/artifacts/*.whl"},
	}

	for _, tc := range tests {
		got, err := resolveDst(tc.dst, tc.baseDir)
		if err != nil {
			t.Errorf("resolveDst(%q, %q) unexpected error: %v", tc.dst, tc.baseDir, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveDst(%q, %q) = %q, want %q", tc.dst, tc.baseDir, got, tc.want)
		}
	}
}

func TestResolveDst_MoreEdgeCases(t *testing.T) {
	tests := []struct {
		dst     string
		baseDir string
		want    string
	}{
		// Test that trailing slashes are cleaned by filepath.Join,
		// but the base structure remains consistent.
		{"trailing/slash/", "/tmp/artifacts", "/artifacts/trailing/slash"},

		// Test deeply nested relative paths
		{"a/b/c/d.txt", "/tmp/artifacts", "/artifacts/a/b/c/d.txt"},

		// Test absolute path that matches baseDir exactly
		{"/tmp/artifacts", "/tmp/artifacts", "/artifacts"},
	}

	for _, tc := range tests {
		got, err := resolveDst(tc.dst, tc.baseDir)
		if err != nil {
			t.Errorf("resolveDst(%q, %q) unexpected error: %v", tc.dst, tc.baseDir, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveDst(%q, %q) = %q, want %q", tc.dst, tc.baseDir, got, tc.want)
		}
	}
}

func TestResolveDst_PathEscapeError(t *testing.T) {
	// Test that paths attempting to escape /artifacts return an error
	tests := []struct {
		dst     string
		baseDir string
	}{
		{"../secret.txt", "/tmp/artifacts"},
		{"../../etc/passwd", "/tmp/artifacts"},
		{"foo/../../../bar", "/tmp/artifacts"},
	}

	for _, tc := range tests {
		_, err := resolveDst(tc.dst, tc.baseDir)
		if err == nil {
			t.Errorf("resolveDst(%q, %q) expected error for path escape, got nil", tc.dst, tc.baseDir)
		}
	}
}

func TestBuildExtractionScript_QuotingAndGlobs(t *testing.T) {
	tests := []struct {
		name      string
		artifacts []*Artifact
		contains  string // Fragment we expect to see
		excludes  string // Fragment we expect NOT to see (like single quotes around a *)
	}{
		{
			name: "Quote files with spaces",
			artifacts: []*Artifact{
				{Src: "/path with spaces/file.txt", Dst: "out.txt"},
			},
			contains: "cp -r '/path with spaces/file.txt' '/artifacts/out.txt'",
		},
		{
			name: "Do NOT quote glob characters",
			artifacts: []*Artifact{
				{Src: "/build/*.whl", Dst: "wheels/"},
			},
			contains: "cp -r /build/*.whl '/artifacts/wheels'",
			excludes: "'/build/*.whl'", // Should not be quoted
		},
		{
			name: "Handle single quotes in filenames",
			artifacts: []*Artifact{
				{Src: "/app/it's-a-file.txt", Dst: "it's-a-file.txt"},
			},
			// Check for the shell-escape sequence '"'"'
			contains: "'/app/it'\"'\"'s-a-file.txt'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script, err := buildExtractionScript(tt.artifacts, "/tmp/artifacts")
			if err != nil {
				t.Fatalf("buildExtractionScript() unexpected error: %v", err)
			}
			if !strings.Contains(script, tt.contains) {
				t.Errorf("script missing expected fragment.\nGot: %s\nWant: %s", script, tt.contains)
			}
			if tt.excludes != "" && strings.Contains(script, tt.excludes) {
				t.Errorf("script contains forbidden fragment (quoting error).\nGot: %s\nForbidden: %s", script, tt.excludes)
			}
		})
	}
}

func TestBuildExtractionScript_DirectoryHints(t *testing.T) {
	// This test ensures that mkdir -p targets the right thing
	// based on the presence of a trailing slash in Dst.
	artifacts := []*Artifact{
		{Src: "/src/file", Dst: "dst/file"}, // No slash: mkdir the parent
		{Src: "/src/dir", Dst: "dst/dir/"},  // Has slash: mkdir the dst itself
	}

	script, err := buildExtractionScript(artifacts, "/tmp/artifacts")
	if err != nil {
		t.Fatalf("buildExtractionScript() unexpected error: %v", err)
	}
	lines := strings.Split(script, "\n")

	// Line 1: mkdir the parent 'dst'
	if !strings.Contains(lines[0], "mkdir -p '/artifacts/dst'") {
		t.Errorf("expected mkdir parent for file: %s", lines[0])
	}

	// Line 2: mkdir the full 'dst/dir'
	if !strings.Contains(lines[1], "mkdir -p '/artifacts/dst/dir'") {
		t.Errorf("expected mkdir full path for directory hint: %s", lines[1])
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"/path/to/file", "'/path/to/file'"},
		{"path with spaces", "'path with spaces'"},
		{"it's a test", "'it'\"'\"'s a test'"},
		{"", "''"},
	}

	for _, tc := range tests {
		got := shellQuote(tc.input)
		if got != tc.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestBuildExtractionScript(t *testing.T) {
	artifacts := []*Artifact{
		{Src: "/build/output.bin", Dst: "bin/output.bin"},
		{Src: "/build/dist/*.whl", Dst: "wheels/"},
		{Src: "/docs/", Dst: "docs/"},
	}

	script, err := buildExtractionScript(artifacts, "/tmp/artifacts")
	if err != nil {
		t.Fatalf("buildExtractionScript() unexpected error: %v", err)
	}

	if !strings.Contains(script, "/build/output.bin") {
		t.Error("script should contain /build/output.bin")
	}
	if !strings.Contains(script, "/build/dist/*.whl") {
		t.Error("script should contain glob pattern")
	}

	// Required artifacts should NOT have || echo
	lines := strings.Split(script, "\n")
	for _, line := range lines {
		if strings.Contains(line, "cp -r") && strings.Contains(line, "|| echo") {
			t.Errorf("required artifacts should not have || echo: %s", line)
		}
	}
}

func TestBuildExtractionScript_optional(t *testing.T) {
	artifacts := []*Artifact{
		{Src: "/required.txt", Dst: "required.txt", Optional: false},
		{Src: "/optional.txt", Dst: "optional.txt", Optional: true},
	}

	script, err := buildExtractionScript(artifacts, "/tmp/artifacts")
	if err != nil {
		t.Fatalf("buildExtractionScript() unexpected error: %v", err)
	}
	lines := strings.Split(script, "\n")

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// First line (required) should not have || echo
	if strings.Contains(lines[0], "|| echo") {
		t.Errorf("required artifact should not have || echo: %s", lines[0])
	}

	// Second line (optional) should have || echo
	if !strings.Contains(lines[1], "|| echo") {
		t.Errorf("optional artifact should have || echo: %s", lines[1])
	}
}

func TestExtractArtifacts(t *testing.T) {
	const testImage = "alpine:latest"

	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      tmpDir,
		ArtifactsDir: artifactsDir,
	}

	forge, err := NewForge(config)
	if err != nil {
		t.Fatalf("NewForge: %v", err)
	}

	d := forge.newDockerCmd()
	if err := d.run("pull", testImage); err != nil {
		t.Fatalf("pull alpine: %v", err)
	}

	spec := &Spec{
		Artifacts: []*Artifact{
			{Src: "/etc/alpine-release", Dst: "alpine-release"},
		},
	}

	if err := forge.ExtractArtifacts(spec, testImage); err != nil {
		t.Fatalf("ExtractArtifacts: %v", err)
	}

	extractedFile := filepath.Join(artifactsDir, "alpine-release")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Errorf("extracted file %s does not exist", extractedFile)
	}
}

func TestExtractArtifacts_glob(t *testing.T) {
	const testImage = "alpine:latest"

	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      tmpDir,
		ArtifactsDir: artifactsDir,
	}

	forge, err := NewForge(config)
	if err != nil {
		t.Fatalf("NewForge: %v", err)
	}

	d := forge.newDockerCmd()
	if err := d.run("pull", testImage); err != nil {
		t.Fatalf("pull alpine: %v", err)
	}

	spec := &Spec{
		Artifacts: []*Artifact{
			{Src: "/etc/*.conf", Dst: "conf/"},
		},
	}

	if err := forge.ExtractArtifacts(spec, testImage); err != nil {
		t.Fatalf("ExtractArtifacts: %v", err)
	}

	confDir := filepath.Join(artifactsDir, "conf")
	entries, err := os.ReadDir(confDir)
	if err != nil {
		t.Fatalf("read conf dir: %v", err)
	}

	foundConf := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".conf") {
			foundConf = true
			break
		}
	}

	if !foundConf {
		t.Error("no .conf files were extracted")
	}
}

func TestExtractArtifacts_optional(t *testing.T) {
	const testImage = "alpine:latest"

	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      tmpDir,
		ArtifactsDir: artifactsDir,
	}

	forge, err := NewForge(config)
	if err != nil {
		t.Fatalf("NewForge: %v", err)
	}

	d := forge.newDockerCmd()
	if err := d.run("pull", testImage); err != nil {
		t.Fatalf("pull alpine: %v", err)
	}

	// Missing optional files should not fail
	spec := &Spec{
		Artifacts: []*Artifact{
			{Src: "/nonexistent/file.txt", Dst: "missing.txt", Optional: true},
		},
	}

	if err := forge.ExtractArtifacts(spec, testImage); err != nil {
		t.Errorf("ExtractArtifacts should not fail for optional artifacts: %v", err)
	}
}

func TestExtractArtifacts_requiredMissing(t *testing.T) {
	const testImage = "alpine:latest"

	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      tmpDir,
		ArtifactsDir: artifactsDir,
	}

	forge, err := NewForge(config)
	if err != nil {
		t.Fatalf("NewForge: %v", err)
	}

	d := forge.newDockerCmd()
	if err := d.run("pull", testImage); err != nil {
		t.Fatalf("pull alpine: %v", err)
	}

	// Missing required files should fail
	spec := &Spec{
		Artifacts: []*Artifact{
			{Src: "/nonexistent/file.txt", Dst: "missing.txt", Optional: false},
		},
	}

	if err := forge.ExtractArtifacts(spec, testImage); err == nil {
		t.Error("ExtractArtifacts should fail for missing required artifacts")
	}
}

func TestExtractArtifacts_withinArtifactsDir(t *testing.T) {
	const testImage = "alpine:latest"

	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      tmpDir,
		ArtifactsDir: artifactsDir,
	}

	forge, err := NewForge(config)
	if err != nil {
		t.Fatalf("NewForge: %v", err)
	}

	d := forge.newDockerCmd()
	if err := d.run("pull", testImage); err != nil {
		t.Fatalf("pull alpine: %v", err)
	}

	// Test with a relative path - should go into artifacts dir
	spec := &Spec{
		Artifacts: []*Artifact{
			{Src: "/etc/alpine-release", Dst: "alpine-release"},
		},
	}

	if err := forge.ExtractArtifacts(spec, testImage); err != nil {
		t.Fatalf("ExtractArtifacts: %v", err)
	}

	expectedPath := filepath.Join(artifactsDir, "alpine-release")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("extracted file %s does not exist", expectedPath)
	}
}
