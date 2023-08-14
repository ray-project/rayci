package wanda

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	WorkDir    string
	DockerBin  string
	WorkRepo   string
	NamePrefix string
	BuildID    string

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

func (f *Forge) workTag(name string) string {
	if f.config.BuildID != "" {
		return fmt.Sprintf(
			"%s:%s-%s", f.config.WorkRepo, f.config.BuildID, name,
		)
	}
	return fmt.Sprintf("%s:%s", f.config.WorkRepo, name)
}

func resolveBuildArgs(buildArgs []string) map[string]string {
	m := make(map[string]string)
	for _, s := range buildArgs {
		k, v, ok := strings.Cut(s, "=")
		if ok {
			m[k] = v
		} else {
			m[s] = os.Getenv(s)
		}
	}
	return m
}

// Build builds a container image from the given specification.
func (f *Forge) Build(spec *Spec) error {
	// Prepare the tar stream.
	ts := newTarStream()
	f.addSrcFile(ts, spec.Dockerfile)
	for _, src := range spec.Srcs {
		f.addSrcFile(ts, src)
	}

	buildArgs := resolveBuildArgs(spec.BuildArgs)

	in := newBuildInput(ts)

	// TODO(aslonnie): fetch and determine the image digests.
	// For now, we just assume that the digest does not change.
	// we are not caching the result anyways right now, so it does not matter.
	froms := make(map[string]string)
	for _, from := range spec.Froms {
		if f.config.NamePrefix != "" {
			if strings.HasPrefix(from, f.config.NamePrefix) {
				fromName := strings.TrimPrefix(from, f.config.NamePrefix)
				workTag := f.workTag(fromName)
				_ = workTag
			}
		} else {
			froms[from] = ""
		}
	}

	inputCore, err := in.makeCore(spec.Dockerfile, buildArgs)
	if err != nil {
		return fmt.Errorf("make build input core: %w", err)
	}

	inputDigest, err := inputCore.digest()
	if err != nil {
		return fmt.Errorf("compute build input digest: %w", err)
	}
	log.Println("build input digest: ", inputDigest)

	// TODO(aslonnie): check if the image output already exists
	// if yes, then just perform retag, rather than rebuilding.

	// Get all the tags.

	// Work tag is the tag we use to save the image in the work repo.
	var workTag string
	if f.config.WorkRepo != "" {
		if f.config.BuildID != "" {
			workTag = fmt.Sprintf(
				"%s:%s-%s", f.config.WorkRepo, f.config.BuildID, spec.Name,
			)
		} else {
			workTag = fmt.Sprintf("%s:%s", f.config.WorkRepo, spec.Name)
		}
		in.addTag(workTag)
	}
	// Name tag is the tag we use to reference the image locally.
	// It is also what can be referenced by following steps.
	if f.config.NamePrefix != "" {
		nameTag := f.config.NamePrefix + spec.Name
		in.addTag(nameTag)
	}
	// And add any extra tags.
	for _, tag := range spec.Tags {
		in.addTag(tag)
	}

	// Now we can build the image.
	d := newDockerCmd(f.config.DockerBin)
	if err := d.build(in, inputCore); err != nil {
		return fmt.Errorf("build docker: %w", err)
	}

	// Push the image to the work repo.
	if f.config.WorkRepo != "" {
		if err := d.run("push", workTag); err != nil {
			return fmt.Errorf("push docker: %w", err)
		}
	}

	// TODO(aslonnie): push back to cr on !f.config.ReadOnlyCache

	return nil
}
