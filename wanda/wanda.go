package wanda

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"

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

	BuildArgs map[string]string `yaml:"build_args,omitempty"`
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

// Build builds a container image from the given specification file.
func Build(specFile string, config *ForgeConfig) error {
	if config == nil {
		config = &ForgeConfig{}
	}

	spec, err := parseSpecFile(specFile)
	if err != nil {
		return fmt.Errorf("parse spec file: %w", err)
	}

	forge := NewForge(config)
	return forge.Build(spec)
}

// ForgeConfig is a configuration for a forge to build container images.
type ForgeConfig struct {
	CacheRepo string

	ReadOnlyCache bool
}

// Forge is a forge to build container images.
type Forge struct {
	config *ForgeConfig
}

// NewForge creates a new forge with the given configuration.
func NewForge(config *ForgeConfig) *Forge {
	return &Forge{config: config}
}

func (f *Forge) pullImage(from string) error {
	// TODO: pull with crane.
	log.Println("pulling image: ", from)
	cmd := exec.Command("docker", "pull", from)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Build builds a container image from the given specification.
func (f *Forge) Build(spec *Spec) error {
	// Prepare all the input.

	// TODO(aslonnie): fetch and check the image digests.
	// Pull all the from/base images.
	for _, from := range spec.Froms {
		if err := f.pullImage(from); err != nil {
			return fmt.Errorf("pull image %q: %w", from, err)
		}
	}

	// Prepare the tar stream.
	ts := newTarStream()
	ts.addSrcFile(spec.Dockerfile)
	for _, f := range spec.Srcs {
		ts.addSrcFile(f)
	}

	// Resolve build args.
	buildArgs := make(map[string]string)
	for k, v := range spec.BuildArgs {
		buildArgs[k] = v
	}

	buildContext, err := ts.digest()
	if err != nil {
		return fmt.Errorf("compute build context digest: %w", err)
	}

	// TODO(aslonnie): fetch and check the image digests.
	froms := make(map[string]string)
	for _, from := range spec.Froms {
		froms[from] = "" // TODO: resolve image digests.
	}

	input := &buildInput{
		Dockerfile:   spec.Dockerfile,
		Froms:        froms,
		BuildContext: buildContext,
		BuildArgs:    buildArgs,
	}
	inputDigest, err := input.digest()
	if err != nil {
		return fmt.Errorf("compute build input digest: %w", err)
	}

	log.Println("build input digest: ", inputDigest)

	// TODO: check if the image output already exists
	// if yes, then just retag.
	_ = inputDigest

	if err := buildDocker(input, ts, spec.Tags); err != nil {
		return fmt.Errorf("build docker: %w", err)
	}

	if !f.config.ReadOnlyCache {
		// Push image to content-address keyed cache repository.
		log.Println("TODO: push back to cr")
	}

	return nil
}
