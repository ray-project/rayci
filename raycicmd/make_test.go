package raycicmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// setupGitRepoInDir initializes a git repository in workDir and returns a
// GitChangeLister configured for testing.
//
// The workDir should already contain files that should exist on the main branch
// (the "base" state). This function will:
//  1. Initialize a git repo in workDir and commit all existing files to main
//  2. Create a feature branch and add the specified changedFiles
//  3. Return a repo and envs configured for the feature branch
//
// This uses gitTestHelper from change_lister_test.go.
func setupGitRepoInDir(t *testing.T, workDir string, changedFiles []string) (*GitChangeLister, *envsMap) {
	t.Helper()

	h := newGitTestHelper(t)
	// Override WorkDir to use the provided directory, then initialize git there.
	h.WorkDir = workDir
	h.git("init")
	h.git("config", "user.email", "test@test.com")
	h.git("config", "user.name", "Test")
	h.git("remote", "add", "origin", h.Origin)

	// Commit all existing files in workDir to main branch (the "base" state).
	h.git("add", ".")
	h.git("commit", "-m", "initial commit with base files")
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

	lister := &GitChangeLister{
		WorkDir:    workDir,
		BaseBranch: "main",
		Commit:     commit,
	}

	return lister, envs
}

func TestIsRayCIYaml(t *testing.T) {
	for _, f := range []string{
		"foo.rayci.yaml",
		"foo.rayci.yml",
		"foo.ci.yaml",
		"foo.ci.yml",
		"dir/foo.rayci.yml",
	} {
		if !isRayCIYaml(f) {
			t.Errorf("want %q to be a rayci yaml", f)
		}
	}

	for _, f := range []string{
		"rayci.yaml",
		"ci.yaml",
		"ci.yml",
		"pipeline.build.yaml",
		"pipeline.tests.yml",
	} {
		if isRayCIYaml(f) {
			t.Errorf("want %q to not be a rayci yaml", f)
		}
	}
}

func TestListCIYamlFiles(t *testing.T) {
	tmp := t.TempDir()

	for _, f := range []string{
		"foo.rayci.yaml",
		"bar.rayci.yaml",
		"foo.rayci.yml",
		"dir/foo.rayci.yml",
		"pipeline.build.yaml",
		"foo.ci.yaml",
		"bar.ci.yaml",
		"foo.ci.yml",
		"dir/foo.ci.yml",
	} {
		dir := filepath.Join(tmp, filepath.Dir(f))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir for %q: %v", f, err)
		}

		if err := os.WriteFile(filepath.Join(tmp, f), nil, 0o600); err != nil {
			t.Fatalf("write file %q: %v", f, err)
		}
	}

	got, err := listCIYamlFiles(tmp)
	if err != nil {
		t.Fatalf("listCIYamlFiles: %v", err)
	}

	want := []string{
		"bar.ci.yaml",
		"bar.rayci.yaml",
		"foo.ci.yaml",
		"foo.ci.yml",
		"foo.rayci.yaml",
		"foo.rayci.yml",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

const goodTestPipeline = `
group: g
key: k
sort_key: sk
steps:
  - label: "test1"
    key: "test1"
    commands: [ "echo test1" ]
  - label: "test2"
    key: "test2"
    commands: [ "echo test2" ]
`

const badTestPipeline = `
name: n
key: k
steps:
	- label: "test1"
`

func TestParsePipelineFile(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "pipe.rayci.yaml")
		if err := os.WriteFile(p, []byte(goodTestPipeline), 0o600); err != nil {
			t.Fatalf("write pipeline file: %v", err)
		}

		g, err := parsePipelineFile(p)
		if err != nil {
			t.Fatalf("parsePipelineFile: %v", err)
		}

		if g.filename != p {
			t.Errorf("got filename %q, want %q", g.filename, p)
		}

		want := &pipelineGroup{
			Group:   "g",
			Key:     "k",
			SortKey: "sk",
			Steps: []map[string]any{{
				"label":    "test1",
				"key":      "test1",
				"commands": []string{"echo test1"},
			}, {
				"label":    "test2",
				"key":      "test2",
				"commands": []string{"echo test2"},
			}},
		}

		gotJSON, err := json.MarshalIndent(g, "", "  ")
		if err != nil {
			t.Fatalf("json marshal got : %v", err)
		}

		wantJSON, err := json.MarshalIndent(want, "", "  ")
		if err != nil {
			t.Fatalf("json marshal want: %v", err)
		}

		if !bytes.Equal(gotJSON, wantJSON) {
			t.Errorf("got %s\n, want %s", gotJSON, wantJSON)
		}
	})

	t.Run("bad", func(t *testing.T) {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "pipeline.yaml")
		if err := os.WriteFile(p, []byte(badTestPipeline), 0o600); err != nil {
			t.Fatalf("write pipeline file: %v", err)
		}

		if _, err := parsePipelineFile(p); err == nil {
			t.Fatalf("parsePipelineFile: got nil, want error")
		}
	})
}

func TestMakePipeline(t *testing.T) {
	tmp := t.TempDir()

	multi := func(s ...string) string {
		return strings.Join(s, "\n")
	}

	for _, f := range []struct {
		name    string
		content string
	}{{
		name: ".buildkite/test.rayci.yaml",
		content: multi(
			`group: g`,
			`steps:`,
			`  - name: "forge"`,
			`    wanda: "fake-forge.wanda.yaml"`,
			`  - label: "tagged test2"`,
			`    key: "test2"`,
			`    tags: "enabled"`,
			`    commands: [ "echo test2" ]`,
			`    depends_on: forge2`,
			`  - label: "disabled"`,
			`    tags: disabled`,
			`    commands: [ "exit 1" ]`,
		),
	}, {
		name: "private/buildkite/private.rayci.yaml",
		content: multi(
			`group: private`,
			`steps:`,
			`  - label: "private test"`,
			`    key: "private-test"`,
			`    commands: [ "echo a private test" ]`,
		),
	}, {
		name: "private/buildkite/forge.rayci.yaml",
		content: multi(
			`group: forge`,
			`steps:`,
			`  - name: forge2`,
			`    wanda: "fake-forge2.wanda.yaml"`,
		),
	}, {
		name: ".buildkite/disabled.rayci.yaml",
		content: multi(
			`group: g`,
			`tags: ["disabled"]`,
			`steps:`,
			`  - label: "test3"`,
			`    key: "test3"`,
			`    commands: [ "echo test3" ]`,
		),
	}} {
		dir := filepath.Join(tmp, filepath.Dir(f.name))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir for %q: %v", f.name, err)
		}

		p := filepath.Join(tmp, f.name)
		if err := os.WriteFile(p, []byte(f.content), 0o600); err != nil {
			t.Fatalf("write file %q: %v", f.name, err)
		}
	}

	commonConfig := &config{
		ArtifactsBucket: "artifacts",
		CITemp:          "s3://ci-temp",
		CIWorkRepo:      "fakeecr",

		BuilderQueues: map[string]string{
			"builder": "builder_queue",
		},
		RunnerQueues:  map[string]string{"default": "runner_x"},
		SkipTags:      []string{"disabled"},
		BuildkiteDirs: []string{".buildkite", "private/buildkite"},
	}

	// Create a test rules file that maps "src/enabled/" to the "enabled" tag.
	rulesContent := strings.Join([]string{
		"! enabled",
		"src/enabled/",
		"@ enabled",
		";",
	}, "\n")
	rulesPath := filepath.Join(tmp, "test_rules.txt")
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0o600); err != nil {
		t.Fatalf("write test rules: %v", err)
	}

	// Set up a real git repo with a changed file that matches the "enabled" rule.
	lister, testEnvs := setupGitRepoInDir(t, tmp, []string{"src/enabled/test.py"})

	t.Run("filter", func(t *testing.T) {
		config := *commonConfig
		config.TestRulesFiles = []string{rulesPath}

		buildID := "fakebuild"
		info := &buildInfo{
			buildID: buildID,
		}

		got, err := makePipeline(&pipelineContext{
			repoDir:      tmp,
			changeLister: lister,
			config:       &config,
			info:         info,
			envs:         testEnvs,
		})
		if err != nil {
			t.Fatalf("makePipeline: %v", err)
		}

		// Should only select the enabled steps and its dependencies.
		// which are 2 steps in 2 different groups.
		if want := 2; len(got.Steps) != want {
			t.Errorf("got %d groups, want %d", len(got.Steps), want)
		}

		// sub functions are already tested in their unit tests.
		// so we only check the total number of groups here.
		// we also have an e2e test at the repo level.

		totalSteps := 0
		for _, g := range got.Steps {
			totalSteps += len(g.Steps)
		}

		if want := 2; totalSteps != want {
			t.Fatalf("got %d steps, want %d", totalSteps, want)
		}
	})

	t.Run("filter_cmd", func(t *testing.T) {
		config := *commonConfig
		config.TagFilterCommand = []string{"echo", "enabled"}

		buildID := "fakebuild"
		info := &buildInfo{
			buildID: buildID,
		}

		got, err := makePipeline(&pipelineContext{
			repoDir:      tmp,
			changeLister: lister,
			config:       &config,
			info:         info,
			envs:         newEnvsMap(nil),
		})
		if err != nil {
			t.Fatalf("makePipeline: %v", err)
		}
		// Should only select the enabled steps and its dependencies.
		// which are 2 steps in 2 different groups.
		if want := 2; len(got.Steps) != want {
			t.Errorf("got %d groups, want %d", len(got.Steps), want)
		}

		// sub functions are already tested in their unit tests.
		// so we only check the total number of groups here.
		// we also have an e2e test at the repo level.

		totalSteps := 0
		for _, g := range got.Steps {
			totalSteps += len(g.Steps)
		}
		if want := 2; totalSteps != want {
			t.Fatalf("got %d steps, want %d", totalSteps, want)
		}
	})

	t.Run("filter_noTagMeansAlways", func(t *testing.T) {
		config := *commonConfig
		config.TestRulesFiles = []string{rulesPath}
		config.NoTagMeansAlways = true

		buildID := "fakebuild"
		info := &buildInfo{
			buildID: buildID,
		}

		got, err := makePipeline(&pipelineContext{
			repoDir:      tmp,
			changeLister: lister,
			config:       &config,
			info:         info,
			envs:         testEnvs,
		})
		if err != nil {
			t.Fatalf("makePipeline: %v", err)
		}

		if want := 3; len(got.Steps) != want { // all steps are groups.
			t.Errorf("got %d groups, want %d", len(got.Steps), want)
		}

		// sub functions are already tested in their unit tests.
		// so we only check the total number of groups here.
		// we also have an e2e test at the repo level.

		totalSteps := 0
		for _, g := range got.Steps {
			totalSteps += len(g.Steps)
		}

		if want := 4; totalSteps != want {
			t.Fatalf("got %d steps, want %d", totalSteps, want)
		}
	})

	t.Run("filter_noTagMeansAlways_cmd", func(t *testing.T) {
		config := *commonConfig
		config.TagFilterCommand = []string{"echo", "enabled"}
		config.NoTagMeansAlways = true

		buildID := "fakebuild"
		info := &buildInfo{
			buildID: buildID,
		}

		got, err := makePipeline(&pipelineContext{
			repoDir:      tmp,
			changeLister: lister,
			config:       &config,
			info:         info,
			envs:         newEnvsMap(nil),
		})
		if err != nil {
			t.Fatalf("makePipeline: %v", err)
		}

		if want := 3; len(got.Steps) != want { // all steps are groups.
			t.Errorf("got %d groups, want %d", len(got.Steps), want)
		}

		// sub functions are already tested in their unit tests.
		// so we only check the total number of groups here.
		// we also have an e2e test at the repo level.

		totalSteps := 0
		for _, g := range got.Steps {
			totalSteps += len(g.Steps)
		}

		if want := 4; totalSteps != want {
			t.Fatalf("got %d steps, want %d", totalSteps, want)
		}
	})

	t.Run("selector", func(t *testing.T) {
		config := *commonConfig

		buildID := "fakebuild"
		info := &buildInfo{
			buildID: buildID,
			selects: []string{"test2"},
		}

		got, err := makePipeline(&pipelineContext{
			repoDir:      tmp,
			changeLister: lister,
			config:       &config,
			info:         info,
		})
		if err != nil {
			t.Fatalf("makePipeline: %v", err)
		}
		if want := 2; len(got.Steps) != want { // all steps are groups.
			t.Errorf("got %d groups, want %d", len(got.Steps), want)
		}

		totalSteps := 0
		var keys []string
		for _, g := range got.Steps {
			totalSteps += len(g.Steps)
			for _, s := range g.Steps {
				step := s.(map[string]any)
				keys = append(keys, stepKey(step))
			}
		}

		if want := 2; totalSteps != want {
			t.Errorf("got %d steps, want %d", totalSteps, want)
		}

		if want := []string{"forge2", "test2"}; !reflect.DeepEqual(keys, want) {
			t.Errorf("got step keys %v, want %v", keys, want)
		}
	})

	t.Run("notify", func(t *testing.T) {
		buildID := "fakebuild"
		info := &buildInfo{
			buildID: buildID,
		}

		config := *commonConfig
		config.NotifyOwnerOnFailure = false

		got, err := makePipeline(&pipelineContext{
			repoDir:      tmp,
			changeLister: lister,
			config:       &config,
			info:         info,
		})
		if err != nil {
			t.Fatalf("makePipeline: %v", err)
		}
		if len(got.Notify) != 0 {
			t.Errorf("got %d notify, want 0", len(got.Notify))
		}

		const email = "reef@anyscale.com"
		infoWithEmail := &buildInfo{
			buildID:          buildID,
			buildAuthorEmail: email,
		}
		config.NotifyOwnerOnFailure = true
		got, err = makePipeline(&pipelineContext{
			repoDir:      tmp,
			changeLister: lister,
			config:       &config,
			info:         infoWithEmail,
		})
		if err != nil {
			t.Fatalf("makePipeline: %v", err)
		}
		if len(got.Notify) == 0 || got.Notify[0].Email != email || got.Notify[0].If != `build.state == "failing"` {
			t.Errorf(`got %v, want email %v, want if build.state == "failing"`, got.Notify, email)
		}
	})
}

func TestSortPipelineGroups(t *testing.T) {
	gs := []*pipelineGroup{{
		filename: "tune.rayci.yaml",
		sortKey:  "tune",
	}, {
		filename: "macos.rayci.yaml",
		sortKey:  "~macos",
	}, {
		filename: "forge.rayci.yaml",
		sortKey:  "_forge",
	}}

	sortPipelineGroups(gs)

	want := []string{
		"forge.rayci.yaml",
		"tune.rayci.yaml",
		"macos.rayci.yaml",
	}
	var got []string
	for _, g := range gs {
		got = append(got, g.filename)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got key order %v, want %v", got, want)
	}
}
