package raycicmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// testGitRepo holds the result of setting up a test git repository.
type testGitRepo struct {
	lister  *ChangeLister
	envs    *envsMap
	workDir string
}

// setupTestGitRepo creates a git repo with the specified changed files and returns
// a ChangeLister and environment variables configured for testing.
func setupTestGitRepo(t *testing.T, changedFiles []string) *testGitRepo {
	t.Helper()

	origin := t.TempDir()
	workDir := t.TempDir()

	// Initialize bare origin repo
	runGit(t, origin, "init", "--bare")

	// Initialize working repo
	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.email", "test@test.com")
	runGit(t, workDir, "config", "user.name", "Test")
	runGit(t, workDir, "remote", "add", "origin", origin)

	// Create initial commit on main branch
	readmePath := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, workDir, "add", "README.md")
	runGit(t, workDir, "commit", "-m", "initial commit")
	runGit(t, workDir, "branch", "-M", "main")
	runGit(t, workDir, "push", "-u", "origin", "main")

	// Create feature branch with changed files
	runGit(t, workDir, "checkout", "-b", "feature-branch")
	for _, f := range changedFiles {
		fullPath := filepath.Join(workDir, f)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", f, err)
		}
		if err := os.WriteFile(fullPath, []byte("content\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}
	runGit(t, workDir, "add", ".")
	runGit(t, workDir, "commit", "-m", "add changed files")

	// Get the commit hash
	commit := getGitOutput(t, workDir, "rev-parse", "HEAD")

	envs := newEnvsMap(map[string]string{
		"BUILDKITE":                          "true",
		"BUILDKITE_PULL_REQUEST":             "123",
		"BUILDKITE_PULL_REQUEST_BASE_BRANCH": "main",
		"BUILDKITE_COMMIT":                   commit,
		"BUILDKITE_BRANCH":                   "feature-branch",
	})

	return &testGitRepo{
		lister:  &ChangeLister{WorkDir: workDir},
		envs:    envs,
		workDir: workDir,
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func getGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// writeTestRules creates a temp rules file and returns its path.
func writeTestRules(t *testing.T, rulesContent string) string {
	t.Helper()
	rulesPath := filepath.Join(t.TempDir(), "test_rules.txt")
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0o600); err != nil {
		t.Fatalf("write rules: %v", err)
	}
	return rulesPath
}

func TestNewTagsStepFilter(t *testing.T) {
	for _, test := range []struct {
		name         string
		filterConfig []string
		skipTags     []string
		want         *stepFilter
	}{{
		name: "empty filterConfig",
		want: &stepFilter{runAll: true},
	}, {
		name:         "nil filterConfig",
		filterConfig: nil,
		want:         &stepFilter{runAll: true},
	}, {
		name:     "skipTags only",
		skipTags: []string{"disabled"},
		want:     &stepFilter{skipTags: stringSet("disabled"), runAll: true},
	}, {
		name:     "multiple skipTags",
		skipTags: []string{"disabled", "skip"},
		want:     &stepFilter{skipTags: stringSet("disabled", "skip"), runAll: true},
	}} {
		t.Run(test.name, func(t *testing.T) {
			got, err := newStepFilter(test.skipTags, nil, test.filterConfig, nil, nil)
			if err != nil {
				t.Fatalf("newStepFilter: %s", err)
			}

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestStepFilter_tags(t *testing.T) {
	filter := &stepFilter{
		skipTags: stringSet("disabled"),
		tags:     stringSet("tune"),

		noTagMeansAlways: true,
	}

	for _, tags := range [][]string{
		{},
		{"tune"},
		{"tune", "foo"},
		{"bar", "tune"},
	} {
		node := &stepNode{tags: tags}

		if !filter.hit(node) {
			t.Errorf("miss %+v", tags)
		}

		if !filter.accept(node) {
			t.Errorf("not accepting %+v", tags)
		}
	}

	for _, tags := range [][]string{
		// Even with "disabled" in the tags list, accept will return true, as it
		// only checks for tags matching.
		{"tune", "data", "disabled"},
	} {
		node := &stepNode{tags: tags}
		if !filter.accept(node) {
			t.Errorf("not accepting %+v", tags)
		}
	}

	for _, tags := range [][]string{
		{"data"},
	} {
		if filter.accept(&stepNode{tags: tags}) {
			t.Errorf("accept %+v, should not", tags)
		}
	}

	for _, tags := range [][]string{
		{"disabled"},
		{"data"},
		{"tune", "disabled"},
		{"disabled", "tune"},
	} {
		if filter.hit(&stepNode{tags: tags}) {
			t.Errorf("hit %+v", tags)
		}
	}
}

func TestStepFilter_tagsReject(t *testing.T) {
	filter := &stepFilter{
		skipTags: stringSet("disabled"),
		tags:     stringSet("tune"),
	}

	for _, tags := range [][]string{
		{},
		{"tune"},
		{"tune", "foo"},
		{"bar", "tune"},
		{"data"},
	} {
		if filter.reject(&stepNode{tags: tags}) {
			t.Errorf("rejects %+v", tags)
		}
	}

	for _, tags := range [][]string{
		{"disabled"},
		{"tune", "disabled"},
		{"disabled", "tune"},
	} {
		if !filter.reject(&stepNode{tags: tags}) {
			t.Errorf("does not reject %+v", tags)
		}
	}
}

func TestStepFilter_runAll(t *testing.T) {
	filter := &stepFilter{
		skipTags: stringSet("disabled"),
		runAll:   true,
	}

	for _, tags := range [][]string{
		nil,
		{},
		{"data"},
		{"tune"},
		{"tune", "foo"},
		{"bar", "tune"},
	} {
		if !filter.hit(&stepNode{tags: tags}) {
			t.Errorf("miss %+v", tags)
		}
	}

	for _, tags := range [][]string{
		{"tune", "disabled"},
		{"disabled", "tune"},
	} {
		if filter.hit(&stepNode{tags: tags}) {
			t.Errorf("hit %+v", tags)
		}
	}
}

func TestStepFilter_selects(t *testing.T) {
	filter, _ := newStepFilter(
		[]string{"disabled"},
		[]string{"foo", "bar"},
		nil, nil, nil,
	)
	for _, node := range []*stepNode{
		{key: "foo"},
		{id: "foo"},
		{id: "bar"},
		{id: "foo", key: "k"},
		{id: "id", key: "foo"},
		{id: "disabled", key: "bar"},
		{id: "foo", tags: []string{"bar"}},

		// even disabled nodes can be selected
		{id: "foo", tags: []string{"disabled"}},
		{key: "bar", tags: []string{"disabled"}},
	} {
		if !filter.accept(node) {
			t.Errorf("miss %+v", node)
		}
	}

	filter, _ = newStepFilter([]string{"disabled"}, []string{"foo", "bar"}, nil, nil, nil)
	for _, node := range []*stepNode{
		{key: "f"},
		{id: "f"},
		{id: "f", tags: []string{"disabled"}},
		{key: "b", tags: []string{"disabled"}},
	} {
		if filter.accept(node) {
			t.Errorf("hit %+v", node)
		}
	}
}

func TestStepFilter_tagSelects(t *testing.T) {
	filter, _ := newStepFilter(nil, []string{"tag:foo", "bar"}, nil, nil, nil)
	for _, node := range []*stepNode{
		{key: "bar"},
		{id: "id", tags: []string{"foo"}},
	} {
		if !filter.accept(node) {
			t.Errorf("tag select miss %+v", node)
		}
	}
}

func TestStepFilter_selectsAndTags_noTagMeansAlways(t *testing.T) {
	rulesPath := writeTestRules(t, "! tune\nsrc/tune/\n@ tune\n;\n")
	repo := setupTestGitRepo(t, []string{"src/tune/file.py"})

	filter, err := newStepFilter(
		[]string{"disabled"},
		[]string{"foo", "bar", "tag:pick"},
		[]string{rulesPath},
		repo.envs,
		repo.lister,
	)
	if err != nil {
		t.Fatalf("newStepFilter: %v", err)
	}
	filter.noTagMeansAlways = true

	for _, node := range []*stepNode{
		{key: "foo"},
		{id: "foo", tags: []string{"tune"}},
		{id: "other", tags: []string{"pick", "tune"}},
		{id: "bar"},
	} {
		if !filter.accept(node) {
			t.Errorf("miss %+v", node)
		}
	}

	for _, node := range []*stepNode{
		{id: "foo", tags: []string{"not_tune"}},
		{id: "bar", tags: []string{"tune_not"}},
		{key: "w00t"},
	} {
		if filter.accept(node) {
			t.Errorf("hit %+v", node)
		}
	}
}

func TestStepFilter_selectsAndTags(t *testing.T) {
	rulesPath := writeTestRules(t, "! tune\nsrc/tune/\n@ tune\n;\n")
	repo := setupTestGitRepo(t, []string{"src/tune/file.py"})

	filter, err := newStepFilter(
		[]string{"disabled"},
		[]string{"foo", "bar", "tag:pick"},
		[]string{rulesPath},
		repo.envs,
		repo.lister,
	)
	if err != nil {
		t.Fatalf("newStepFilter: %v", err)
	}

	for _, node := range []*stepNode{
		{id: "foo", tags: []string{"tune"}},
		{id: "other", tags: []string{"pick", "tune"}},
	} {
		if !filter.accept(node) {
			t.Errorf("miss %+v", node)
		}
	}

	for _, node := range []*stepNode{
		{key: "foo"},
		{id: "bar"},
		{id: "foo", tags: []string{"not_tune"}},
		{id: "bar", tags: []string{"tune_not"}},
		{key: "w00t"},
	} {
		if filter.accept(node) {
			t.Errorf("hit %+v", node)
		}
	}
}

func TestRunFilterConfig(t *testing.T) {
	t.Run("empty filterConfig", func(t *testing.T) {
		got, err := runFilterConfig(nil, nil, nil)
		if err != nil {
			t.Fatalf("runFilterConfig: %s", err)
		}
		if !got.runAll {
			t.Errorf("runAll: got %v, want true", got.runAll)
		}
	})

	t.Run("nil filterConfig", func(t *testing.T) {
		got, err := runFilterConfig([]string{}, nil, nil)
		if err != nil {
			t.Fatalf("runFilterConfig: %s", err)
		}
		if !got.runAll {
			t.Errorf("runAll: got %v, want true", got.runAll)
		}
	})

	t.Run("with filterConfig", func(t *testing.T) {
		rulesPath := writeTestRules(t, "! mytag\nsrc/mydir/\n@ mytag\n;\n")
		repo := setupTestGitRepo(t, []string{"src/mydir/file.py"})

		got, err := runFilterConfig([]string{rulesPath}, repo.envs, repo.lister)
		if err != nil {
			t.Fatalf("runFilterConfig: %s", err)
		}

		if got.runAll {
			t.Errorf("runAll: got true, want false")
		}

		// Should have "mytag" plus default tags "always" and "lint".
		for _, tag := range []string{"mytag", "always", "lint"} {
			if !got.tags[tag] {
				t.Errorf("missing tag %q in %v", tag, got.tags)
			}
		}
	})
}
