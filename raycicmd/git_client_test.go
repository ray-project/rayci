package raycicmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestListChangedFiles_InvalidCommitRange(t *testing.T) {
	client := &RealGitClient{}

	tests := []struct {
		name        string
		commitRange string
	}{
		{"no separator", "abc123"},
		{"wrong separator", "abc123..def456"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ListChangedFiles("main", tt.commitRange)
			if err == nil {
				t.Error(
					"ListChangedFiles() expected error for invalid commit range",
				)
			}
		})
	}
}

// runGitCommand runs a git command in the specified directory.
func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// gitTestHelper provides a test git repository with origin and working directory.
type gitTestHelper struct {
	t       *testing.T
	Origin  string
	WorkDir string
}

func newGitTestHelper(t *testing.T) *gitTestHelper {
	t.Helper()
	origin := t.TempDir()
	workDir := t.TempDir()

	h := &gitTestHelper{t: t, Origin: origin, WorkDir: workDir}

	// Initialize bare origin repo
	h.gitIn(origin, "init", "--bare")

	// Initialize working repo
	h.gitIn(workDir, "init")
	h.gitIn(workDir, "config", "user.email", "test@test.com")
	h.gitIn(workDir, "config", "user.name", "Test")
	h.gitIn(workDir, "remote", "add", "origin", origin)

	return h
}

func (h *gitTestHelper) gitIn(dir string, args ...string) {
	h.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func (h *gitTestHelper) git(args ...string) {
	h.gitIn(h.WorkDir, args...)
}

func (h *gitTestHelper) head() string {
	h.t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = h.WorkDir
	out, err := cmd.Output()
	if err != nil {
		h.t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func (h *gitTestHelper) writeFile(path, content string) {
	h.t.Helper()
	fullPath := filepath.Join(h.WorkDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		h.t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		h.t.Fatalf("write %s: %v", path, err)
	}
}

func (h *gitTestHelper) commitFiles(msg string, files ...string) string {
	h.t.Helper()
	for _, f := range files {
		h.writeFile(f, "content\n")
	}
	h.git("add", ".")
	h.git("commit", "-m", msg)
	return h.head()
}

func (h *gitTestHelper) initialCommit() {
	h.t.Helper()
	h.writeFile("README.md", "# README\n")
	h.git("add", "README.md")
	h.git("commit", "-m", "initial commit")
}

func TestListChangedFiles_Integration(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	wantFiles := []string{"src/main.go", "src/util.go", "docs/readme.txt"}
	commit := h.commitFiles("add files", wantFiles...)

	client := &RealGitClient{WorkDir: h.WorkDir}
	files, err := client.ListChangedFiles("master", "origin/master..."+commit)
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	if len(files) != len(wantFiles) {
		t.Fatalf("got %d files, want %d: %v", len(files), len(wantFiles), files)
	}
	for _, want := range wantFiles {
		if !slices.Contains(files, want) {
			t.Errorf("missing file %s in result %v", want, files)
		}
	}
}

func TestListChangedFiles_EmptyBaseBranch(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	baseCommit := h.head()

	newCommit := h.commitFiles("add new file", "new_file.txt")

	client := &RealGitClient{WorkDir: h.WorkDir}
	files, err := client.ListChangedFiles("", baseCommit+"..."+newCommit)
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	if len(files) != 1 || files[0] != "new_file.txt" {
		t.Errorf("got files %v, want [new_file.txt]", files)
	}
}
