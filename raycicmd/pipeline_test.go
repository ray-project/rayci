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

		AgentQueueMap: map[string]string{"default": "runner"},
		Dockerless:    true,
	})

	for _, test := range []struct {
		in  *pipelineStep
		out any // buildkite pipeline step
	}{{
		in: &pipelineStep{Commands: []string{"echo 1", "echo 2"}},
		out: &bkCommandStep{
			Commands:         []string{"echo 1", "echo 2"},
			Agents:           newBkAgents("runner"),
			TimeoutInMinutes: defaultTimeoutInMinutes,
			AritfactPaths:    defaultArtifactsPaths,
			Retry:            defaultRayRetry,
		},
	}, {
		in: &pipelineStep{
			Label:     "say hello",
			Key:       "key",
			Commands:  []string{"echo hello"},
			DependsOn: []string{"dep"},
		},
		out: &bkCommandStep{
			Label:     "say hello",
			Key:       "key",
			Commands:  []string{"echo hello"},
			DependsOn: []string{"dep"},

			Agents: newBkAgents("runner"),

			TimeoutInMinutes: defaultTimeoutInMinutes,
			AritfactPaths:    defaultArtifactsPaths,
			Retry:            defaultRayRetry,
		},
	}, {
		in:  &pipelineStep{Type: stepTypeWait},
		out: &bkWaitStep{},
	}, {
		in:  &pipelineStep{Type: stepTypeWait, If: "false"},
		out: &bkWaitStep{If: "false"},
	}} {
		got, err := c.convertPipelineStep(test.in)
		if err != nil {
			t.Errorf("convertPipelineStep %+v: %v", test.in, err)
			continue
		}

		if !reflect.DeepEqual(got, test.out) {
			gotJSON, _ := json.MarshalIndent(got, "", "  ")
			wantJSON, _ := json.MarshalIndent(test.out, "", "  ")

			t.Errorf(
				"convertPipelineStep %+v: got:\n %s\nwant:\n %s",
				test.in, gotJSON, wantJSON,
			)
		}
	}
}
