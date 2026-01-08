package raycicmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// testGitRepo holds the result of setting up a test git repository.
type testGitRepo struct {
	lister  ChangeLister
	envs    *envsMap
	workDir string
}

// setupTestGitRepo creates a git repo with the specified changed files and returns
// a ChangeLister and environment variables configured for testing.
// Uses gitTestHelper from change_lister_test.go.
func setupTestGitRepo(t *testing.T, changedFiles []string) *testGitRepo {
	t.Helper()

	h := newGitTestHelper(t)
	h.initialCommit()
	h.git("branch", "-M", "main")
	h.git("push", "-u", "origin", "main")

	// Create feature branch with changed files
	h.git("checkout", "-b", "feature-branch")
	for _, f := range changedFiles {
		h.writeFile(f, "content\n")
	}
	h.git("add", ".")
	h.git("commit", "-m", "add changed files")

	commit := h.head()

	envs := newEnvsMap(map[string]string{
		"BUILDKITE":                          "true",
		"BUILDKITE_PULL_REQUEST":             "123",
		"BUILDKITE_PULL_REQUEST_BASE_BRANCH": "main",
		"BUILDKITE_COMMIT":                   commit,
		"BUILDKITE_BRANCH":                   "feature-branch",
	})

	return &testGitRepo{
		lister:  &GitChangeLister{WorkDir: h.WorkDir, BaseBranch: "main", Commit: commit},
		envs:    envs,
		workDir: h.WorkDir,
	}
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
		filterCmd    []string
		filterConfig []string
		skipTags     []string
		want         *stepFilter
	}{{
		name: "neither filterCmd nor testRulesFiles set",
		want: &stepFilter{runAll: true},
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
			got, err := newStepFilter(test.skipTags, nil, test.filterCmd, test.filterConfig, nil, nil)
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
		nil, nil, nil, nil,
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

	filter, _ = newStepFilter([]string{"disabled"}, []string{"foo", "bar"}, nil, nil, nil, nil)
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
	filter, _ := newStepFilter(nil, []string{"tag:foo", "bar"}, nil, nil, nil, nil)
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
		nil, // filterCmd (deprecated fallback)
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
		nil, // filterCmd (deprecated fallback)
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

func TestFilterFromRuleFiles(t *testing.T) {
	t.Run("star tag triggers runAll", func(t *testing.T) {
		rules := "! *\n*\n@ *\n;\n"
		rulesPath := writeTestRules(t, rules)
		repo := setupTestGitRepo(t, []string{"src/any/file.py"})

		got, err := filterFromRuleFiles([]string{rulesPath}, repo.envs, repo.lister)
		if err != nil {
			t.Fatalf("filterFromRuleFiles: %s", err)
		}

		if !got.runAll {
			t.Errorf("runAll: got false, want true")
		}
	})

	t.Run("with filterConfig", func(t *testing.T) {
		// Rules with a specific rule for src/mydir/ and a catch-all for always/lint.
		// First matching rule wins - src/mydir/ files get mytag, others get always/lint.
		joinLines := func(lines ...string) string {
			return strings.Join(lines, "\n")
		}

		rules := joinLines("! mytag always lint",
			"src/mydir/",
			"@ mytag always lint",
			";",
			"*",
			"@ always lint",
			";",
		)
		rulesPath := writeTestRules(t, rules)
		repo := setupTestGitRepo(t, []string{"src/mydir/file.py"})

		got, err := filterFromRuleFiles([]string{rulesPath}, repo.envs, repo.lister)
		if err != nil {
			t.Fatalf("filterFromRuleFiles: %s", err)
		}

		if got.runAll {
			t.Errorf("runAll: got true, want false")
		}

		// Should have "mytag", "always", and "lint" from the src/mydir/ rule.
		for _, tag := range []string{"mytag", "always", "lint"} {
			if !got.tags[tag] {
				t.Errorf("missing tag %q in %v", tag, got.tags)
			}
		}
	})
}

func TestFilterFromCmd(t *testing.T) {
	for _, test := range []struct {
		cmd []string
		res *filterSetup
	}{{
		cmd: []string{"echo", "RAYCI_COVERAGE"},
		res: &filterSetup{tags: stringSet("RAYCI_COVERAGE")},
	}, {
		cmd: []string{"echo", "RAYCI_COVERAGE\n"},
		res: &filterSetup{tags: stringSet("RAYCI_COVERAGE")},
	}, {
		cmd: []string{"echo", "\t  \n  \t"},
		res: &filterSetup{tags: stringSet()},
	}, {
		cmd: []string{"echo", "*"},
		res: &filterSetup{runAll: true},
	}, {
		cmd: []string{"echo", "tag1 tag2 *"},
		res: &filterSetup{runAll: true},
	}, {
		cmd: []string{"./not-exist"},
		res: &filterSetup{runAll: true},
	}} {
		got, err := filterFromCmd(test.cmd)
		if err != nil {
			t.Fatalf("run %q: %s", test.cmd, err)
		}

		if !reflect.DeepEqual(got, test.res) {
			t.Errorf(
				"run %q: got %+v, want %+v",
				test.cmd, got, test.res,
			)
		}
	}
}
