package raycicmd

import (
	"strings"
	"testing"

	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
)

func TestIsRayCIYaml(t *testing.T) {
	for _, f := range []string{
		"foo.rayci.yaml",
		"foo.rayci.yml",
		"dir/foo.rayci.yml",
	} {
		if !isRayCIYaml(f) {
			t.Errorf("want %q to be a rayci yaml", f)
		}
	}

	for _, f := range []string{
		"rayci.yaml",
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
		"bar.rayci.yaml",
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

	t.Run("filter", func(t *testing.T) {
		config := *commonConfig
		config.TagFilterCommand = []string{"echo", "enabled"}

		buildID := "fakebuild"
		info := &buildInfo{
			buildID: buildID,
		}

		got, err := makePipeline(tmp, &config, info)
		if err != nil {
			t.Fatalf("makePipeline: %v", err)
		}

		if want := 3; len(got.Steps) != want { // all steps are groups.
			t.Errorf("got %d groups, want %d", len(got.Steps), want)
		}

		// sub funtions are already tested in their unit tests.
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

		got, err := makePipeline(tmp, &config, info)
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
