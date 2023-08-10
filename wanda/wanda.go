package wanda

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

	forge, err := NewForge(config)
	if err != nil {
		return fmt.Errorf("make forge: %w", err)
	}
	return forge.Build(spec)
}

// ForgeConfig is a configuration for a forge to build container images.
type ForgeConfig struct {
	WorkDir       string
	CacheRepo     string
	ReadOnlyCache bool
}

// Forge is a forge to build container images.
type Forge struct {
	config *ForgeConfig

	workDir string
}

// NewForge creates a new forge with the given configuration.
func NewForge(config *ForgeConfig) (*Forge, error) {
	absWorkDir, err := filepath.Abs(config.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("abs path for work dir: %w", err)
	}

	return &Forge{config: config, workDir: absWorkDir}, nil
}

func (f *Forge) addSrcFile(ts *tarStream, src string) {
	ts.addFile(src, nil, filepath.Join(f.workDir, src))
}

// Build builds a container image from the given specification.
func (f *Forge) Build(spec *Spec) error {
	// Prepare the tar stream.
	ts := newTarStream()
	f.addSrcFile(ts, spec.Dockerfile)
	for _, src := range spec.Srcs {
		f.addSrcFile(ts, src)
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

	// TODO(aslonnie): fetch and determine the image digests.
	// For now, we just assume that the digest does not change.
	// we are not caching the result anyways right now, so it does not matter.
	froms := make(map[string]string)
	for _, from := range spec.Froms {
		froms[from] = ""
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

	// TODO(aslonnie): check if the image output already exists
	// if yes, then just perform retag, rather than rebuilding.

	if err := buildDocker(input, ts, spec.Tags); err != nil {
		return fmt.Errorf("build docker: %w", err)
	}

	if !f.config.ReadOnlyCache {
		// Push image to content-address keyed cache repository.
		log.Println("TODO: push back to cr")
	}

	return nil
}
