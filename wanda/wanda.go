package wanda

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"

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
	spec, err := parseSpecFile(specFile)
	if err != nil {
		return fmt.Errorf("parse spec file: %w", err)
	}

	forge := NewForge(config)
	return forge.Build(spec)
}

// ForgeConfig is a configuration for a forge to build container images.
type ForgeConfig struct {
	RepositoryPrefix string

	CacheRepository string
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

func sha256Sum(bs []byte) string {
	h := sha256.New()
	h.Write(bs)
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

// Build builds a container image from the given specification.
func (f *Forge) Build(spec *Spec) error {
	// Prepare all the input.

	// Pull all the from/base images.
	for _, from := range spec.Froms {
		if err := f.pullImage(from); err != nil {
			return fmt.Errorf("pull image %q: %w", from, err)
		}
	}

	// Prepare the tar stream.
	ts := newTarStream()
	for _, f := range spec.Srcs {
		ts.addFile(f, nil, f)
	}

	// Resolve build args.
	buildArgs := make(map[string]string)
	for k, v := range spec.BuildArgs {
		buildArgs[k] = v
	}

	type buildInput struct {
		Froms        map[string]string // Map from image names to image digests.
		BuildContext string            // Digests of the build context.
		Dockerfile   string            // Name of the Dockerfile to use.
		BuildArgs    map[string]string // Resolved build args.
	}

	buildContext, err := ts.digest()
	if err != nil {
		return fmt.Errorf("compute build context digest: %w", err)
	}

	input := &buildInput{
		Froms:        make(map[string]string),
		BuildContext: buildContext,
		Dockerfile:   spec.Dockerfile,
		BuildArgs:    buildArgs,
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal build input: %w", err)
	}
	inputDigest := sha256Sum(inputBytes)

	log.Println("build input digest: ", inputDigest)

	// TODO: check if the image output already exists
	// if yes, then just retag.
	_ = inputDigest

	for _, from := range spec.Froms {
		input.Froms[from] = "" // TODO: resolve image digests.
	}

	// Build the image.
	var args []string

	args = append(args, "build", "--progress=plain")
	args = append(args, "-f", spec.Dockerfile)

	for _, t := range spec.Tags {
		args = append(args, "-t", t)
	}

	var buildArgKeys []string
	for k := range buildArgs {
		buildArgKeys = append(buildArgKeys, k)
	}
	sort.Strings(buildArgKeys)
	for _, k := range buildArgKeys {
		v := buildArgs[k]
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = newWriterToReader(ts)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build image: %w", err)
	}

	// Push image to content-address keyed cache repository.

	return nil
}
