package raycicmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"reflect"
)

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

func lookupEnvInArray(envs []string, key string) (string, bool) {
	for _, e := range envs {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			if k == key {
				return v, true
			}
		} else {
			if e == key {
				return "", true
			}
		}
	}
	return "", false
}

func TestConvertPipelineStep(t *testing.T) {
	const buildID = "abc123"

	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		CITempRepo:      "fakeecr",

		RunnerQueues: map[string]string{"default": "fakerunner"},
	}, buildID)

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
		},
	}, {
		in:  map[string]any{"wait": nil},
		out: map[string]any{"wait": nil},
	}, {
		in:  map[string]any{"wait": nil, "continue_on_failure": true},
		out: map[string]any{"wait": nil, "continue_on_failure": true},
	}} {
		got, err := c.convertPipelineStep(test.in)
		if err != nil {
			t.Errorf("convertPipelineStep %+v: %v", test.in, err)
			continue
		}

		_, isWait := got["wait"]

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

		if isWait {
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
		envBuildID, ok := lookupEnvInArray(envs, "RAYCI_BUILD_ID")
		if !ok {
			t.Errorf("convertPipelineStep %+v: no RAYCI_BUILD_ID", test.in)
		}
		if envBuildID != buildID {
			t.Errorf(
				"convertPipelineStep %+v: got RAYCI_BUILD_ID %q, want %q",
				test.in, envBuildID, buildID,
			)
		}

		envTemp, ok := lookupEnvInArray(envs, "RAYCI_TEMP")
		if !ok {
			t.Errorf("convertPipelineStep %+v: no RAYCI_TEMP", test.in)
		}
		if want := "s3://ci-temp/abc123/"; envTemp != want {
			t.Errorf(
				"convertPipelineStep %+v: got RAYCI_TEMP %q, want %q",
				test.in, envTemp, want,
			)
		}

	}
}

func TestConvertPipelineGroup(t *testing.T) {
	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",

		RunnerQueues: map[string]string{"default": "runner"},
		Dockerless:   true,
	}, "buildid")

	g := &pipelineGroup{
		Group: "fancy",
		Steps: []map[string]any{
			{"commands": []string{"echo 1"}},
			{"wait": nil},
			{"commands": []string{"echo 1"}},
		},
	}
	bk, err := c.convertPipelineGroup(g)
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
