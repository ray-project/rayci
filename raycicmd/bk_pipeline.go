package raycicmd

type bkPipelineGroup struct {
	Group string `yaml:"group,omitempty"`
	Key   string `yaml:"key,omitempty"`
	Steps []any  `yaml:"steps,omitempty"`
}

type bkPipeline struct {
	Steps []*bkPipelineGroup `yaml:"steps,omitempty"`
}

func newBkAgents(queue string) map[string]any {
	return map[string]any{"queue": queue}
}

func makeNoopBkPipeline(q string) *bkPipeline {
	step := map[string]any{"command": "echo no pipeline steps"}
	if q != "" {
		step["agents"] = newBkAgents(q)
	}

	return &bkPipeline{
		Steps: []*bkPipelineGroup{{
			Group: "noop",
			Steps: []any{step},
		}},
	}
}
