package raycicmd

type pipelineGroup struct {
	Group string `yaml:"group"`
	Key   string `yaml:"key"`

	Steps []map[string]any `yaml:"steps"`
}

var (
	waitStepAllowedKeys    = []string{"wait", "continue_on_failure"}
	commandStepAllowedKeys = []string{
		"command", "commands", "priority", "parallelism",
		"label", "name", "key", "depends_on", "soft_fail", "matrix",
		"instance_type", "queue", "job_env",
	}

	commandStepDropKeys = []string{
		"instance_type", "queue", "job_env",
	}
)
