package imgspec

type Spec struct {
	Version string `yaml:"version"`

	Name string `yaml:"name,omitempty"`

	Tags []string `yaml:"tags,omitempty"`

	BuildArgs  []string `yaml:"build_args,omitempty"`
	Dockerfile string   `yaml:"dockerfile,omitempty"`
	Srcs       []string `yaml:"srcs,omitempty"`
	Froms      []string `yaml:"bases,omitempty"`
}
