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
		p := filepath.Join(tmp, "pipeline.yaml")
		if err := os.WriteFile(p, []byte(goodTestPipeline), 0o600); err != nil {
			t.Fatalf("write pipeline file: %v", err)
		}

		g, err := parsePipelineFile(p)
		if err != nil {
			t.Fatalf("parsePipelineFile: %v", err)
		}

		want := &pipelineGroup{
			Group: "g",
			Key:   "k",
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
			`steps: `,
			`  - label: "test1"`,
			`    key: "test1"`,
			`    commands: [ "echo test1" ]`,
		),
	}, {
		name:    ".buildkite/forge/Dockerfile.forge",
		content: `FROM ubuntu:latest`,
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

	config := &config{
		ArtifactsBucket: "artifacts",
		CITemp:          "s3://ci-temp",
		CIWorkRepo:      "fakeecr",

		BuilderQueues: map[string]string{
			"builder": "builder_queue",
		},
		RunnerQueues: map[string]string{"default": "runner_x"},
		ForgeDirs:    defaultForgeDirs,
	}

	buildID := "fakebuild"

	got, err := makePipeline(tmp, config, buildID, "")
	if err != nil {
		t.Fatalf("makePipeline: %v", err)
	}

	if len(got.Steps) != 2 { // all steps are groups.
		t.Errorf("got %d groups, want 2", len(got.Steps))
	}

	// sub funtions are already tested in their unit tests.
	// so we only check the total number of groups here.
	// we also have an e2e test at the repo level.

	totalSteps := 0
	for _, g := range got.Steps {
		totalSteps += len(g.Steps)
	}

	if totalSteps != 2 {
		t.Fatalf("got %d steps, want 2", totalSteps)
	}
}
