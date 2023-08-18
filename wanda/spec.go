package wanda

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Spec is a specification for a container image.
type Spec struct {
	Name string `yaml:"name,omitempty"`

	Tags []string `yaml:"tags"`

	// Inputs
	Froms      []string `yaml:"froms"`
	Srcs       []string `yaml:"srcs,omitempty"`
	Dockerfile string   `yaml:"dockerfile"`

	BuildArgs []string `yaml:"build_args,omitempty"`

	// Wastefully include everything on work directory into the image.
	CopyEverything bool `yaml:"copy_everything,omitempty"`
}

func parseSpecFile(f string) (*Spec, error) {
	bs, err := os.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	spec := new(Spec)
	dec := yaml.NewDecoder(bytes.NewReader(bs))
	dec.KnownFields(true)
	if err := dec.Decode(spec); err != nil {
		return nil, fmt.Errorf("decode spec: %w", err)
	}

	return spec, nil
}
