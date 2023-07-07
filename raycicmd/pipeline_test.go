package raycicmd

import (
	"testing"

	"reflect"
)

func TestConvertPipelineStep(t *testing.T) {
	for _, test := range []struct {
		in  *pipelineStep
		out any // buildkite pipeline step
	}{{
		in:  &pipelineStep{Commands: []string{"echo 1", "echo 2"}},
		out: &bkCommandStep{Commands: []string{"echo 1", "echo 2"}},
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
		},
	}, {
		in:  &pipelineStep{Type: stepTypeWait},
		out: &bkWaitStep{},
	}, {
		in:  &pipelineStep{Type: stepTypeWait, If: "false"},
		out: &bkWaitStep{If: "false"},
	}} {
		got, err := convertPipelineStep(test.in)
		if err != nil {
			t.Errorf("convertPipelineStep %+v: %v", test.in, err)
			continue
		}

		if !reflect.DeepEqual(got, test.out) {
			t.Errorf(
				"convertPipelineStep %+v: got %+v, want %+v",
				test.in, got, test.out,
			)
		}
	}
}
