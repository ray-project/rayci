package raycicmd

type pipelineGroup struct {
	Group string   `yaml:"group"`
	Key   string   `yaml:"key"`
	Tags  []string `yaml:"tags"`

	Steps []map[string]any `yaml:"steps"`
}

var (
	waitStepAllowedKeys = []string{
		"wait", "continue_on_failure", "if", "depends_on",
		"tags",
	}
	waitStepDropKeys = []string{"tags"}

	commandStepAllowedKeys = []string{
		"command", "commands", "priority", "parallelism", "if",
		"label", "name", "key", "depends_on", "soft_fail", "matrix",
		"instance_type", "queue", "job_env", "tags",
		"docker_publish_tcp_ports", "mount_buildkite_agent",
	}
	commandStepDropKeys = []string{
		"instance_type", "queue", "job_env", "tags",
		"docker_publish_tcp_ports", "mount_buildkite_agent",
	}

	wandaStepAllowedKeys = []string{
		"name", "label", "wanda", "depends_on",
		"matrix", "env", "tags", "instance_type",
	}
)
