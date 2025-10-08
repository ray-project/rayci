package raycicmd

type pipelineGroup struct {
	filename string
	sortKey  string

	Group   string   `yaml:"group"`
	Key     string   `yaml:"key"`
	Tags    []string `yaml:"tags"`
	SortKey string   `yaml:"sort_key,omitempty"`

	DependsOn []string `yaml:"depends_on"`

	// DefaultJobEnv is the default job environment variables for all the
	// command steps in the group.
	DefaultJobEnv string `yaml:"default_job_env"`

	Steps []map[string]any `yaml:"steps"`
}

func (g *pipelineGroup) lessThan(other *pipelineGroup) bool {
	if g.sortKey == other.sortKey {
		return g.filename < other.filename
	}
	return g.sortKey < other.sortKey
}

var (
	waitStepAllowedKeys = []string{
		"wait", "continue_on_failure", "if", "depends_on",
		"allow_dependency_failure", "tags", "key",
	}
	waitStepDropKeys = []string{"tags"}

	blockStepAllowedKeys = []string{
		"block", "if", "depends_on", "tags", "key",
		"fields", "prompt", "blocked_state", "allow_dependency_failure",
	}
	blockStepDropKeys = []string{"tags"}

	triggerStepAllowedKeys = []string{
		"trigger", "label", "async", "build", "depends_on",
		"tags", "if", "soft_fail", "allow_dependency_failure",
		"key",
	}
	triggerStepDropKeys = []string{"tags"}

	commandStepAllowedKeys = []string{
		"command", "commands", "priority", "parallelism", "if",
		"label", "name", "key", "depends_on", "soft_fail", "matrix",
		"allow_dependency_failure", "concurrency", "concurrency_group",
		"timeout_in_minutes",

		// The following keys will be processed by rayci and dropped.
		"instance_type", "queue", "job_env", "tags",
		"docker_publish_tcp_ports", "docker_network",
		"mount_buildkite_agent", "mount_windows_artifacts",
		"aws_assume_role", "aws_assume_role_duration_seconds",
	}
	commandStepDropKeys = []string{
		"instance_type", "queue", "job_env", "tags",
		"docker_publish_tcp_ports", "docker_network",
		"mount_buildkite_agent", "mount_windows_artifacts",
		"aws_assume_role", "aws_assume_role_duration_seconds",

		// The following keys are processed by rayci.
		"timeout_in_minutes",
	}

	wandaStepAllowedKeys = []string{
		"name", "label", "wanda", "depends_on",
		"matrix", "env", "tags", "instance_type", "priority",
	}
)
