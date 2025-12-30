package raycicmd

import (
	"os"
	"path/filepath"
	"reflect"
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
			got, err := newStepFilter(test.skipTags, nil, nil, test.filterConfig, nil, nil)
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
		// Rules with a fallthrough rule for src/mydir/ and default rules for always/lint.
		// The fallthrough directive means matching continues, so default rules also apply.
		rules := "! mytag always lint\n" +
			"src/mydir/\n" +
			"\\fallthrough\n" +
			"@ mytag\n" +
			";\n" +
			"\\default\n" +
			"@ always lint\n" +
			";\n"
		rulesPath := writeTestRules(t, rules)
		repo := setupTestGitRepo(t, []string{"src/mydir/file.py"})

		got, err := runFilterConfig([]string{rulesPath}, repo.envs, repo.lister)
		if err != nil {
			t.Fatalf("runFilterConfig: %s", err)
		}

		if got.runAll {
			t.Errorf("runAll: got true, want false")
		}

		// Should have "mytag" from the matching rule plus default tags "always" and "lint".
		for _, tag := range []string{"mytag", "always", "lint"} {
			if !got.tags[tag] {
				t.Errorf("missing tag %q in %v", tag, got.tags)
			}
		}
	})
}

func TestRunFilterCmd(t *testing.T) {
	for _, test := range []struct {
		cmd []string
		res *filterCmdResult
	}{{
		cmd: []string{"echo", "RAYCI_COVERAGE"},
		res: &filterCmdResult{cmdExists: true, tags: stringSet("RAYCI_COVERAGE")},
	}, {
		cmd: []string{"echo", "RAYCI_COVERAGE\n"},
		res: &filterCmdResult{cmdExists: true, tags: stringSet("RAYCI_COVERAGE")},
	}, {
		cmd: []string{"echo", "\t  \n  \t"},
		res: &filterCmdResult{cmdExists: true},
	}, {
		cmd: []string{},
		res: &filterCmdResult{},
	}, {
		cmd: nil,
		res: &filterCmdResult{},
	}, {
		cmd: []string{"echo", "*"},
		res: &filterCmdResult{cmdExists: true, runAll: true},
	}, {
		cmd: []string{"./not-exist"},
		res: &filterCmdResult{},
	}} {
		got, err := runFilterCmd(test.cmd)
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
