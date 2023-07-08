package raycicmd

type bkCommandStep struct {
	Label     string   `yaml:"label,omitempty"`
	Key       string   `yaml:"key,omitempty"`
	If        string   `yaml:"if,omitempty"`
	Commands  []string `yaml:"commands"`
	Env       []string `yaml:"env,omitempty"`
	DependsOn []string `yaml:"depends_on,omitempty"`

	SoftFail bool     `yaml:"soft_fail,omitempty"`
	Retry    *bkRetry `yaml:"retry,omitempty"`

	Parallelism int `yaml:"parallelism,omitempty"`

	// Inserted by rayci.
	Plugins          []any     `yaml:"plugins,omitempty"`
	Agents           *bkAgents `yaml:"agents,omitempty"`
	TimeoutInMinutes int       `yaml:"timeout_in_minutes,omitempty"`
	AritfactPaths    []string  `yaml:"artifact_paths,omitempty"`
}

type bkAgents struct {
	Queue string `yaml:"queue,omitempty"`
}

func newBkAgents(queue string) *bkAgents { return &bkAgents{Queue: queue} }

type bkRetry struct {
	Manual    *bkRetryManual      `yaml:"manual,omitempty"`
	Automatic []*bkRetryAutomatic `yaml:"automatic,omitempty"`
}

type bkRetryManual struct {
	PermitOnPassed bool `yaml:"permit_on_passed,omitempty"`
}

type bkRetryAutomatic struct {
	ExitStatus int `yaml:"exit_status,omitempty"`
	Limit      int `yaml:"limit,omitempty"`
}

var defaultRayRetry = &bkRetry{
	Manual: &bkRetryManual{PermitOnPassed: true},
	Automatic: []*bkRetryAutomatic{
		{ExitStatus: -1, Limit: 3},
		{ExitStatus: 255, Limit: 3},
	},
}

type bkWaitStep struct {
	// Always leave this as nil is fine, which will generate
	// `wait: ~` in the yaml file.
	Wait *struct{} `yaml:"wait"`

	If string `yaml:"if,omitempty"`

	// For wait step only
	// wait step also has an `if` and `depends_on` field.
	ContinueOnFailure bool `yaml:"continue_on_failure,omitempty"`
}

type bkPipelineGroup struct {
	Group string `yaml:"group,omitempty"`
	Key   string `yaml:"key,omitempty"`
	Steps []any  `yaml:"steps,omitempty"`
}

type bkPipeline struct {
	Steps []*bkPipelineGroup `yaml:"steps,omitempty"`
}

type bkDockerPlugin struct {
	Image        string   `yaml:"image"`
	Shell        []string `yaml:"shell,omitempty"`
	WorkDir      string   `yaml:"workdir,omitempty"`
	AddCaps      []string `yaml:"add-caps,omitempty"`
	SecurityOpts []string `yaml:"security-opts,omitempty"`
	Environment  []string `yaml:"environment,omitempty"`
}

func makeRayDockerPlugin(image string) *bkDockerPlugin {
	return &bkDockerPlugin{
		Image:        image,
		Shell:        []string{"/bin/bash", "-elic"},
		WorkDir:      "/ray",
		AddCaps:      []string{"SYS_PTRACE", "SYS_ADMIN", "NET_ADMIN"},
		SecurityOpts: []string{"apparmor=unconfined"},
		Environment: []string{
			"CI",
			"BUILDKITE",
			"BUILDKITE_BRANCH",
			"BUILDKITE_COMMIT",
			"BUILDKITE_LABEL",
			"BUILDKITE_PIPELINE_ID",
			"BUILDKITE_PIPELINE_SLUG",
			"BUILDKITE_BUILD_ID",
			"BUILDKITE_BUILD_NUMBER",
			"BUILDKITE_BUILD_URL",
			"BUILDKITE_JOB_ID",
			"BUILDKITE_PARALLEL_JOB",
			"BUILDKITE_PARALLEL_JOB_COUNT",
			"BUILDKITE_MESSAGE",
		},
	}
}
