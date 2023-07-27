package raycicmd

import (
	"encoding/json"
	"testing"

	"reflect"
)

func TestConvertPipelineStep(t *testing.T) {
	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",

		RunnerQueues: map[string]string{"default": "runner"},
		Dockerless:   true,
	}, "buildid")

	for _, test := range []struct {
		in  map[string]any
		out map[string]any // buildkite pipeline step
	}{{
		in: map[string]any{"commands": []string{"echo 1", "echo 2"}},
		out: map[string]any{
			"commands":           []string{"echo 1", "echo 2"},
			"agents":             newBkAgents("runner"),
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

			"agents": newBkAgents("runner"),

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
