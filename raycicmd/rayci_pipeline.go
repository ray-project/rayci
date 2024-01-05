package raycicmd

type pipelineGroup struct {
	Group string   `yaml:"group"`
	Key   string   `yaml:"key"`
	Tags  []string `yaml:"tags"`

	DependsOn []string `yaml:"depends_on"`

	Steps []map[string]any `yaml:"steps"`
}

var (
	waitStepAllowedKeys = []string{
		"wait", "continue_on_failure", "if", "depends_on",
		"tags",
	}
	waitStepDropKeys = []string{"tags"}

	blockStepAllowedKeys = []string{
		"block", "if", "depends_on", "tags",
	}
	blockStepDropKeys = []string{"tags"}

	commandStepAllowedKeys = []string{
		"command", "commands", "priority", "parallelism", "if",
		"label", "name", "key", "depends_on", "soft_fail", "matrix",
		"instance_type", "queue", "job_env", "tags",
		"docker_publish_tcp_ports", "docker_network",
		"mount_buildkite_agent", "mount_windows_artifacts",
	}
	commandStepDropKeys = []string{
		"instance_type", "queue", "job_env", "tags",
		"docker_publish_tcp_ports", "docker_network",
		"mount_buildkite_agent", "mount_windows_artifacts",
	}

	wandaStepAllowedKeys = []string{
		"name", "label", "wanda", "depends_on",
		"matrix", "env", "tags", "instance_type",
	}
)
