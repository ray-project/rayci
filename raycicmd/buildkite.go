package raycicmd

type bkCommandStep struct {
	Label     string   `yaml:"label,omitempty"`
	Key       string   `yaml:"key,omitempty"`
	If        string   `yaml:"if,omitempty"`
	Commands  []string `yaml:"commands"`
	Env       []string `yaml:"env,omitempty"`
	DependsOn []string `yaml:"depends_on,omitempty"`

	// Inserted by rayci.
	Plugins []any `yaml:"plugins,omitempty"`
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
