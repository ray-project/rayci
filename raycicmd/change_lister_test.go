package raycicmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

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
		// Use unique content for each file to avoid git's rename detection
		h.writeFile(f, "content of "+f+"\n")
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

// TestListChangedFiles verifies basic changed file detection.
//
// Git history (time flows left to right):
//
//	feature-branch:              [A] ---------> [B]
//	                              |              + src/main.go
//	                              |              + src/util.go
//	                              |              + docs/readme.txt
//	origin/master:   ... ------> [A]
//
//	A = initial commit (README.md)
//	B = feature branch HEAD (adds the new files)
//
//	ListChangedFiles("master", B) returns: [src/main.go, src/util.go, docs/readme.txt]
func TestListChangedFiles(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	wantFiles := []string{"src/main.go", "src/util.go", "docs/readme.txt"}
	commit := h.commitFiles("add files", wantFiles...)

	lister := &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "master", Commit: commit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	if len(files) != len(wantFiles) {
		t.Fatalf("got %d files, want %d: %v", len(files), len(wantFiles), files)
	}
	for _, want := range wantFiles {
		if !slices.Contains(files, want) {
			t.Errorf("got %v, want %s in result", files, want)
		}
	}
}

// TestListChangedFiles_MergeBase verifies that ListChangedFiles uses merge-base
// to compute the diff. This is important because it means changes on the base
// branch after the feature branch was created don't affect the diff.
//
// Git history (time flows left to right):
//
//	feature-branch:   		   [A] --------------------> [B]
//	                            |   branch off        	+ feature.go
//	                            |
//	origin/master:    ... ---> [A] ---> [C] --------> [D]
//	                                    + other.go    + another.go
//
//	A = initial commit (README.md), the branch point
//	B = feature branch HEAD (adds feature.go)
//	C, D = later commits on master (adds other.go, another.go)
//
//	merge-base(origin/master, B) = A  (the common ancestor)
//
//	Correct (using merge-base):  diff(A, B) = [feature.go]
//	Wrong (without merge-base):  diff(D, B) = [feature.go, other.go, another.go]
//
//	ListChangedFiles("master", B) should return only: [feature.go]
func TestListChangedFiles_MergeBase(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	// Create feature branch and add a file.
	h.git("checkout", "-b", "feature-branch")
	featureCommit := h.commitFiles("add feature", "feature.go")

	// Go back to master and add another file (simulating other PRs being merged).
	h.git("checkout", "master")
	h.commitFiles("add other file on master", "other.go")
	h.git("push", "origin", "master")

	// The diff should only show feature.go, not other.go,
	// because we diff against the merge-base (common ancestor).
	lister := &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "master", Commit: featureCommit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("got %d files, want 1: %v", len(files), files)
	}
	if files[0] != "feature.go" {
		t.Errorf("got %s, want feature.go", files[0])
	}
}

func TestListChangedFiles_CustomRemote(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()

	// Add a second remote called "upstream".
	upstream := t.TempDir()
	h.gitIn(upstream, "init", "--bare")
	h.git("remote", "add", "upstream", upstream)
	h.git("push", "upstream", "master")

	h.git("checkout", "-b", "feature-branch")
	commit := h.commitFiles("add feature", "feature.go")

	// Use a lister with custom remote.
	lister := &GitChangeLister{WorkDir: h.WorkDir, Remote: "upstream", BaseBranch: "master", Commit: commit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	if len(files) != 1 || files[0] != "feature.go" {
		t.Errorf("got %v, want [feature.go]", files)
	}
}

// TestListChangedFiles_MultipleCommits verifies that all changes across multiple
// commits on the feature branch are detected.
//
// Git history (time flows left to right):
//
//	feature-branch:              [A] -----> [B] -----> [C]
//	                              |         + first.go + second.go
//	                              |
//	origin/master:   ... ------> [A]
//
//	ListChangedFiles("master", C) returns: [first.go, second.go]
func TestListChangedFiles_MultipleCommits(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	h.commitFiles("first commit", "first.go")
	commit := h.commitFiles("second commit", "second.go")

	lister := &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "master", Commit: commit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	wantFiles := []string{"first.go", "second.go"}
	if len(files) != len(wantFiles) {
		t.Fatalf("got %d files, want %d: %v", len(files), len(wantFiles), files)
	}
	for _, want := range wantFiles {
		if !slices.Contains(files, want) {
			t.Errorf("got %v, want %s in result", files, want)
		}
	}
}

// TestListChangedFiles_ModifyExistingFile verifies that modifications to
// existing files are detected, not just new files.
//
// Git history (time flows left to right):
//
//	feature-branch:              [A] ---------> [B]
//	                              |              ~ README.md (modified)
//	                              |
//	origin/master:   ... ------> [A]
//	                              + README.md
//
//	ListChangedFiles("master", B) returns: [README.md]
func TestListChangedFiles_ModifyExistingFile(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	// Modify existing file
	h.writeFile("README.md", "# Modified README\n\nNew content here.\n")
	h.git("add", ".")
	h.git("commit", "-m", "modify readme")
	commit := h.head()

	lister := &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "master", Commit: commit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	if len(files) != 1 || files[0] != "README.md" {
		t.Errorf("got %v, want [README.md]", files)
	}
}

// TestListChangedFiles_DeleteFile verifies that deleted files are detected.
//
// Git history (time flows left to right):
//
//	feature-branch:              [A] ---------> [B]
//	                              |              - existing.go (deleted)
//	                              |
//	origin/master:   ... ------> [A]
//	                              + existing.go
//
//	ListChangedFiles("master", B) returns: [existing.go]
func TestListChangedFiles_DeleteFile(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.commitFiles("add file to delete", "existing.go")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	// Delete the file
	if err := os.Remove(filepath.Join(h.WorkDir, "existing.go")); err != nil {
		t.Fatalf("remove file: %v", err)
	}
	h.git("add", ".")
	h.git("commit", "-m", "delete file")
	commit := h.head()

	lister := &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "master", Commit: commit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	if len(files) != 1 || files[0] != "existing.go" {
		t.Errorf("got %v, want [existing.go]", files)
	}
}

// TestListChangedFiles_RenameFile verifies that renamed files are detected
// (shows as both old and new path, or just new path depending on git config).
//
// Git history (time flows left to right):
//
//	feature-branch:              [A] ---------> [B]
//	                              |              old.go -> new.go (renamed)
//	                              |
//	origin/master:   ... ------> [A]
//	                              + old.go
//
//	ListChangedFiles("master", B) returns: [old.go, new.go] or similar
func TestListChangedFiles_RenameFile(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.commitFiles("add file to rename", "old.go")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	// Rename the file
	h.git("mv", "old.go", "new.go")
	h.git("commit", "-m", "rename file")
	commit := h.head()

	lister := &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "master", Commit: commit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	// git diff --name-only shows both old and new paths for renames
	if len(files) < 1 {
		t.Fatalf("got %d files, want at least 1: %v", len(files), files)
	}
	hasOld := slices.Contains(files, "old.go")
	hasNew := slices.Contains(files, "new.go")
	if !hasOld && !hasNew {
		t.Errorf("got %v, want old.go or new.go in result", files)
	}
}

// TestListChangedFiles_NoChanges verifies empty result when there are no changes.
//
// Git history (time flows left to right):
//
//	feature-branch:              [A]
//	                              |
//	origin/master:   ... ------> [A]
//
//	ListChangedFiles("master", A) returns: []
func TestListChangedFiles_NoChanges(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	// No changes, just branch off
	commit := h.head()

	lister := &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "master", Commit: commit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("got %v, want []", files)
	}
}

// TestListChangedFiles_SameFileModifiedOnBothBranches verifies behavior when
// the same file is modified on both branches (common merge conflict scenario).
//
// Git history (time flows left to right):
//
//	feature-branch:              [A] --------------------> [B]
//	                              |                        ~ shared.go (v2)
//	                              |
//	origin/master:   ... ------> [A] ---> [C]
//	                              + shared.go (v1)  ~ shared.go (v1.1)
//
//	ListChangedFiles("master", B) returns: [shared.go]
func TestListChangedFiles_SameFileModifiedOnBothBranches(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.commitFiles("add shared file", "shared.go")
	h.git("push", "origin", "master")

	// Create feature branch and modify shared.go
	h.git("checkout", "-b", "feature-branch")
	h.writeFile("shared.go", "// feature branch version\npackage main\n")
	h.git("add", ".")
	h.git("commit", "-m", "modify shared.go on feature")
	featureCommit := h.head()

	// Go back to master and also modify shared.go
	h.git("checkout", "master")
	h.writeFile("shared.go", "// master branch version\npackage main\n")
	h.git("add", ".")
	h.git("commit", "-m", "modify shared.go on master")
	h.git("push", "origin", "master")

	lister := &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "master", Commit: featureCommit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	// Should only show shared.go as changed in the feature branch
	if len(files) != 1 || files[0] != "shared.go" {
		t.Errorf("got %v, want [shared.go]", files)
	}
}

// TestListChangedFiles_DeepDirectoryStructure verifies files in nested directories.
//
// Git history (time flows left to right):
//
//	feature-branch:              [A] ---------> [B]
//	                              |              + a/b/c/deep.go
//	                              |              + x/y/z/another.go
//	origin/master:   ... ------> [A]
//
//	ListChangedFiles("master", B) returns: [a/b/c/deep.go, x/y/z/another.go]
func TestListChangedFiles_DeepDirectoryStructure(t *testing.T) {
	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature-branch")
	wantFiles := []string{"a/b/c/deep.go", "x/y/z/another.go"}
	commit := h.commitFiles("add nested files", wantFiles...)

	lister := &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "master", Commit: commit}
	files, err := lister.ListChangedFiles()
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}

	if len(files) != len(wantFiles) {
		t.Fatalf("got %d files, want %d: %v", len(files), len(wantFiles), files)
	}
	for _, want := range wantFiles {
		if !slices.Contains(files, want) {
			t.Errorf("got %v, want %s in result", files, want)
		}
	}
}
