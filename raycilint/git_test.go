package raycilint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

	h.gitIn(origin, "init", "--bare", "--initial-branch=master")

	h.git("init", "--initial-branch=master")
	h.git("config", "user.email", "test@test.com")
	h.git("config", "user.name", "Test")
	h.git("remote", "add", "origin", origin)

	return h
}

func (h *gitTestHelper) gitIn(dir string, args ...string) {
	h.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("git %v in %s failed: %v\n%s", args, dir, err, out)
	}
}

func (h *gitTestHelper) git(args ...string) {
	h.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = h.WorkDir
	if out, err := cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
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

func (h *gitTestHelper) commitAll(msg string) string {
	h.t.Helper()
	h.git("add", ".")
	h.git("commit", "-m", msg)
	return h.head()
}

func (h *gitTestHelper) makeConfig(
	maxAdd, maxDel int, ignore []string,
) *config {
	h.t.Helper()
	cfg := newConfig()
	cfg.Prsize.MaxAdditions = maxAdd
	cfg.Prsize.MaxDeletions = maxDel
	cfg.Prsize.Ignore = ignore
	return cfg
}

func (h *gitTestHelper) diffStats(t *testing.T, ignore []string) *diffStats {
	t.Helper()
	g := &gitClient{workDir: h.WorkDir}
	if err := g.fetchRef("master"); err != nil {
		t.Fatalf("fetchRef(master): %v", err)
	}
	if err := g.fetchRef("feature"); err != nil {
		t.Fatalf("fetchRef(feature): %v", err)
	}
	mb, err := g.mergeBase("master", "feature")
	if err != nil {
		t.Fatalf("mergeBase: %v", err)
	}
	output, err := g.diffNumstat(mb, "origin/feature")
	if err != nil {
		t.Fatalf("diffNumstat: %v", err)
	}
	stats, err := parseDiffNumstat(output, ignore)
	if err != nil {
		t.Fatalf("parseDiffNumstat: %v", err)
	}
	return stats
}

func TestFetchRef(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial commit")
	h.git("push", "origin", "master")

	g := &gitClient{workDir: h.WorkDir}
	if err := g.fetchRef("master"); err != nil {
		t.Fatalf("fetchRef() error: %v", err)
	}

	out, err := g.command("rev-parse", "origin/master").Output()
	if err != nil {
		t.Fatalf("origin/master not found after fetch: %v", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Error("origin/master resolved to empty string")
	}
}

func TestFetchRef_InvalidRef(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial commit")
	h.git("push", "origin", "master")

	g := &gitClient{workDir: h.WorkDir}
	err := g.fetchRef("nonexistent")
	if err == nil {
		t.Fatal("fetchRef() error = nil, want error for invalid ref")
	}
}

func TestMergeBase(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	baseSHA := h.commitAll("initial commit")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.writeFile("feature.go", "package main\n")
	h.commitAll("add feature")
	h.git("push", "origin", "feature")

	g := &gitClient{workDir: h.WorkDir}
	g.fetchRef("master")
	g.fetchRef("feature")

	got, err := g.mergeBase("master", "feature")
	if err != nil {
		t.Fatalf("mergeBase() error: %v", err)
	}
	if got != baseSHA {
		t.Errorf("mergeBase() = %s, want %s", got, baseSHA)
	}
}

func TestMergeBase_InvalidRef(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial commit")

	g := &gitClient{workDir: h.WorkDir}
	_, err := g.mergeBase("nonexistent", "alsonotreal")
	if err == nil {
		t.Fatal("mergeBase() error = nil, want error for invalid refs")
	}
}

func TestDiffNumstat(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	baseSHA := h.commitAll("initial commit")

	h.writeFile("a.go", strings.Join([]string{"line1", "line2", "line3"}, "\n")+"\n")
	headSHA := h.commitAll("add file")

	g := &gitClient{workDir: h.WorkDir}
	out, err := g.diffNumstat(baseSHA, headSHA)
	if err != nil {
		t.Fatalf("diffNumstat() error: %v", err)
	}

	line := strings.TrimSpace(string(out))
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) != 3 {
		t.Fatalf("diffNumstat() output = %q, want tab-separated triple", line)
	}
	if got, want := parts[0], "3"; got != want {
		t.Errorf("additions = %s, want %s", got, want)
	}
	if got, want := parts[1], "0"; got != want {
		t.Errorf("deletions = %s, want %s", got, want)
	}
	if got, want := parts[2], "a.go"; got != want {
		t.Errorf("filename = %s, want %s", got, want)
	}
}

func TestDiffNumstat_NoChanges(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	sha := h.commitAll("initial commit")

	g := &gitClient{workDir: h.WorkDir}
	out, err := g.diffNumstat(sha, sha)
	if err != nil {
		t.Fatalf("diffNumstat() error: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "" {
		t.Errorf("diffNumstat() = %q, want empty", got)
	}
}

func TestCountChangedLines_AddFile(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.writeFile("src/main.go", strings.Join([]string{
		"package main",
		"func main() {}",
		"// end",
	}, "\n")+"\n")
	h.commitAll("add main.go")
	h.git("push", "origin", "feature")

	stats := h.diffStats(t, nil)
	if got, want := stats.linesAdded, 3; got != want {
		t.Errorf("added = %d, want %d", got, want)
	}
	if got, want := stats.linesDeleted, 0; got != want {
		t.Errorf("deleted = %d, want %d", got, want)
	}
}

func TestCountChangedLines_NoChanges(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.git("commit", "--allow-empty", "-m", "empty commit")
	h.git("push", "origin", "feature")

	stats := h.diffStats(t, nil)
	if got, want := stats.linesAdded, 0; got != want {
		t.Errorf("added = %d, want %d", got, want)
	}
	if got, want := stats.linesDeleted, 0; got != want {
		t.Errorf("deleted = %d, want %d", got, want)
	}
}

func TestCountChangedLines_Ignore(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.writeFile("src/main.go", "line1\nline2\n")
	h.writeFile("vendor/lib.go", "vendored1\nvendored2\nvendored3\n")
	h.writeFile("docs/readme.md", "doc line\n")
	h.commitAll("add files")
	h.git("push", "origin", "feature")

	stats := h.diffStats(t, []string{"vendor/", "docs/"})
	if got, want := stats.linesAdded, 2; got != want {
		t.Errorf("added = %d, want %d", got, want)
	}
	if got, want := stats.linesDeleted, 0; got != want {
		t.Errorf("deleted = %d, want %d", got, want)
	}
}

func TestCountChangedLines_Deletions(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("existing.go", "line1\nline2\nline3\nline4\nline5\n")
	h.commitAll("initial")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.writeFile("existing.go", "line1\nline5\n")
	h.commitAll("shrink file")
	h.git("push", "origin", "feature")

	stats := h.diffStats(t, nil)
	if got, want := stats.linesDeleted, 3; got != want {
		t.Errorf("deleted = %d, want %d", got, want)
	}
}

func TestCountChangedLines_DeletedFile(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("doomed.go", "line1\nline2\nline3\n")
	h.commitAll("initial")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.git("rm", "doomed.go")
	h.commitAll("delete file")
	h.git("push", "origin", "feature")

	stats := h.diffStats(t, nil)
	if got, want := stats.linesAdded, 0; got != want {
		t.Errorf("added = %d, want %d", got, want)
	}
	if got, want := stats.linesDeleted, 3; got != want {
		t.Errorf("deleted = %d, want %d", got, want)
	}
}

func TestCountChangedLines_RenamedFile(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("old_name.go", "line1\nline2\nline3\n")
	h.commitAll("initial")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.git("mv", "old_name.go", "new_name.go")
	h.commitAll("rename file")
	h.git("push", "origin", "feature")

	stats := h.diffStats(t, nil)
	if got, want := stats.linesAdded, 3; got != want {
		t.Errorf("added = %d, want %d", got, want)
	}
	if got, want := stats.linesDeleted, 3; got != want {
		t.Errorf("deleted = %d, want %d", got, want)
	}
}

func TestCountChangedLines_BinaryFile(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.writeFile("image.png", "\x89PNG\r\n\x1a\n"+strings.Repeat("\x00", 100))
	h.writeFile("small.go", "package main\n")
	h.commitAll("add binary and text")
	h.git("push", "origin", "feature")

	stats := h.diffStats(t, nil)
	if got, want := stats.linesAdded, 1; got != want {
		t.Errorf("added = %d, want %d (binary should not count)", got, want)
	}
	if got, want := stats.linesDeleted, 0; got != want {
		t.Errorf("deleted = %d, want %d", got, want)
	}
}

func TestCountChangedLines_MixedOperations(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("keep.go", "line1\nline2\nline3\n")
	h.writeFile("modify.go", "old1\nold2\nold3\nold4\n")
	h.writeFile("delete_me.go", "gone1\ngone2\n")
	h.commitAll("initial")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.writeFile("new.go", "new1\nnew2\nnew3\nnew4\nnew5\n")
	h.writeFile("modify.go", "old1\nchanged\nold3\nold4\nextra\n")
	h.git("rm", "delete_me.go")
	h.commitAll("mixed changes")
	h.git("push", "origin", "feature")

	stats := h.diffStats(t, nil)
	// new.go: +5, modify.go: +2 -1, delete_me.go: -2
	if got, want := stats.linesAdded, 7; got != want {
		t.Errorf("added = %d, want %d", got, want)
	}
	if got, want := stats.linesDeleted, 3; got != want {
		t.Errorf("deleted = %d, want %d", got, want)
	}
}
