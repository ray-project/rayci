package raycicmd

import (
	"testing"

	"encoding/json"
	"fmt"
	"reflect"
)

func TestParseStepEnvs(t *testing.T) {
	for _, test := range []struct {
		in      any
		want    []*envEntry
		wantErr bool
	}{
		{in: nil, wantErr: true},
		{in: "string", wantErr: true},
		{in: []string{"A=B"}, wantErr: true},
		{in: map[string]string{"A": "B"}, wantErr: true},
		{
			in: map[string]any{"PY_VERSION": "py36", "A": "B"},
			want: []*envEntry{
				{k: "A", v: "B"},
				{k: "PY_VERSION", v: "py36"},
			},
		},
	} {
		got, err := parseStepEnvs(test.in)
		if test.wantErr {
			if err == nil {
				t.Errorf("parseStepEnvs(%+v): want error, got nil", test.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseStepEnvs(%+v) got error: %v", test.in, err)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"parseStepEnvs(%+v): got %+v, want %+v",
				test.in, got, test.want,
			)
		}
	}
}

func findDockerPlugin(plugins []any) (map[string]any, bool) {
	for _, p := range plugins {
		if m, ok := p.(map[string]any); ok {
			v, ok := m[dockerPlugin]
			if ok {
				return v.(map[string]any), true
			}
		}
	}

	return nil, false
}

func findInSlice(s []string, v string) bool {
	for _, e := range s {
		if e == v {
			return true
		}
	}
	return false
}

func TestConvertPipelineStep(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		BuildID:     buildID,
		RayCIBranch: "beta",
		GitCommit:   "abcdefg1234567890",
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CIWorkRepo:      "fakeecr",

		RunnerQueues: map[string]string{"default": "fakerunner"},

		Env: map[string]string{
			"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
		},

		HookEnvKeys: []string{"RAYCI_CHECKOUT_DIR"},
	}, info)

	const artifactDest = "s3://artifacts_bucket/abcdefg1234567890"

	for _, test := range []struct {
		in  map[string]any
		out map[string]any // buildkite pipeline step
	}{{
		in: map[string]any{"commands": []string{"echo 1", "echo 2"}},
		out: map[string]any{
			"commands":           []string{"echo 1", "echo 2"},
			"agents":             newBkAgents("fakerunner"),
			"timeout_in_minutes": defaultTimeoutInMinutes,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,
			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
	}, {
		in: map[string]any{
			"label":      "say hello",
			"key":        "key",
			"command":    "echo hello",
			"depends_on": "dep",
		},
		out: map[string]any{
			"label":      "say hello",
			"key":        "key",
			"command":    "echo hello",
			"depends_on": "dep",

			"agents": newBkAgents("fakerunner"),

			"timeout_in_minutes": defaultTimeoutInMinutes,
			"artifact_paths":     defaultArtifactPaths,
			"retry":              defaultRayRetry,
			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,
			},
		},
	}, {
		in: map[string]any{
			"name":       "forge",
			"label":      "my forge",
			"wanda":      "ci/forge.wanda.yaml",
			"depends_on": "ci-base",
			"env":        map[string]any{"PY_VERSION": "{{matrix}}"},
			"matrix":     []any{"py36", "py37"},
		},
		out: map[string]any{
			"label":    "my forge",
			"key":      "forge",
			"commands": wandaCommands,
			"env": map[string]string{
				"RAYCI_BUILD_ID":            buildID,
				"RAYCI_TEMP":                "s3://ci-temp/abc123/",
				"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
				"RAYCI_WORK_REPO":           "fakeecr",
				"RAYCI_BRANCH":              "beta",

				"RAYCI_WANDA_FILE": "ci/forge.wanda.yaml",
				"RAYCI_WANDA_NAME": "forge",

				"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifactDest,

				"PY_VERSION": "{{matrix}}",
			},
			"matrix":     []any{"py36", "py37"},
			"depends_on": "ci-base",
			"retry": map[string]any{
				"automatic": map[string]any{"limit": 1},
			},
			"timeout_in_minutes": 300,
		},
	}, {
		in:  map[string]any{"wait": nil},
		out: map[string]any{"wait": nil},
	}, {
		in: map[string]any{
			"wait": nil, "continue_on_failure": true,
			"depends_on": "dep", "if": "false",
		},
		out: map[string]any{
			"wait": nil, "continue_on_failure": true,
			"depends_on": "dep", "if": "false",
		},
	}} {
		got, err := c.convertPipelineStep(test.in)
		if err != nil {
			t.Errorf("convertPipelineStep %+v: %v", test.in, err)
			continue
		}
		if got == nil {
			if test.out != nil {
				t.Errorf(
					"convertPipelineStep %+v: got:\n %s\nwant:\n %s",
					test.in, got, test.out,
				)
			}
			continue
		}

		_, isWait := got["wait"]
		_, isWanda := test.in["wanda"]

		plugins, ok := got["plugins"]
		if ok {
			// Check non plugins only.
			delete(got, "plugins")
		}

		if !reflect.DeepEqual(got, test.out) {
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal got: +%v: %s", got, err)
			}
			wantJSON, err := json.MarshalIndent(test.out, "", "  ")
			if err != nil {
				t.Fatalf("marshal want: +%v: %s", test.out, err)
			}

			t.Errorf(
				"convertPipelineStep %+v: got:\n %s\nwant:\n %s",
				test.in, gotJSON, wantJSON,
			)
		}

		if isWait || isWanda {
			continue
		}

		dockerPlugin, ok := findDockerPlugin(plugins.([]any))
		if !ok {
			t.Errorf("convertPipelineStep %+v: no docker plugin", test.in)
			continue
		}

		image, ok := stringInMap(dockerPlugin, "image")
		if !ok {
			t.Errorf("convertPipelineStep %+v: no docker image", test.in)
		}
		if want := fmt.Sprintf("fakeecr:%s-forge", buildID); image != want {
			t.Errorf(
				"convertPipelineStep %+v: got docker image %q, want %q",
				test.in, image, want,
			)
		}

		envs := dockerPlugin["environment"].([]string)

		for _, env := range []string{
			"RAYCI_BUILD_ID",
			"RAYCI_TEMP",
			"RAYCI_WORK_REPO",
			"BUILDKITE_BAZEL_CACHE_URL",
			"RAYCI_CHECKOUT_DIR",
		} {
			if !findInSlice(envs, env) {
				t.Errorf("convertPipelineStep %+v: no %q", test.in, env)
			}
		}
	}
}

func TestConvertPipelineStep_priority(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		BuildID:     buildID,
		RayCIBranch: "beta",
		GitCommit:   "abcdefg1234567890",
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CIWorkRepo:      "fakeecr",

		RunnerQueues: map[string]string{"default": "fakerunner"},

		Env: map[string]string{
			"BUILDKITE_BAZEL_CACHE_URL": "https://bazel-build-cache",
		},

		RunnerPriority: 1,
	}, info)

	g := &pipelineGroup{
		Group: "fancy",
		Steps: []map[string]any{
			{"commands": []string{"high priority"}, "priority": 10},
			{"wait": nil},
			{"commands": []string{"default priority"}},
		},
	}
	bk, err := c.convertPipelineGroup(g, &tagFilter{tags: []string{}, runAll: true})
	if err != nil {
		t.Fatalf("convertPipelineGroup: %v", err)
	}

	if len(bk.Steps) != 3 {
		t.Fatalf("convertPipelineGroup: got %d steps, want 3", len(bk.Steps))
	}

	steps := bk.Steps
	if p := (steps[0].(map[string]any))["priority"]; p != 10 {
		t.Errorf("high priority step: got priority %v, want 10", p)
	}
	if p := (steps[2].(map[string]any))["priority"]; p != 1 {
		t.Errorf("low priority step: got priority %v, want 1", p)
	}
}

func TestConvertPipelineGroup(t *testing.T) {
	const buildID = "abc123"
	info := &buildInfo{
		BuildID: buildID,
	}

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",

		RunnerQueues: map[string]string{"default": "runner"},
	}, info)

	g := &pipelineGroup{
		Group: "fancy",
		Steps: []map[string]any{
			{"commands": []string{"echo 1"}},
			{"wait": nil},
			{"commands": []string{"echo 1"}, "tags": []interface{}{"foo"}},
			{"commands": []string{"echo 2"}, "tags": []interface{}{"bar"}},
		},
	}

	bk, err := c.convertPipelineGroup(g, &tagFilter{tags: []string{"foo"}})
	if err != nil {
		t.Fatalf("convertPipelineGroup: %v", err)
	}

	if bk.Group != "fancy" {
		t.Errorf("convertPipelineGroup: got group %s, want fancy", bk.Group)
	}
	if len(bk.Steps) != 3 {
		t.Errorf("convertPipelineGroup: got %d steps, want 3", len(bk.Steps))
	}
}
