package raycicmd

import (
	"testing"
)

func TestBkPipelineTotalSteps(t *testing.T) {
	p := &bkPipeline{
		Steps: []*bkPipelineGroup{
			{
				Group: "group1",
				Steps: []any{
					map[string]any{"command": "echo step1"},
					map[string]any{"command": "echo step2"},
				},
			},
			{
				Group: "group2",
				Steps: []any{
					map[string]any{"command": "echo step3"},
				},
			},
		},
	}
	if total := p.totalSteps(); total != 3 {
		t.Errorf("totalSteps() = %d; want 3", total)
	}
}
