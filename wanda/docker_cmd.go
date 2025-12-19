package wanda

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

func dockerCmdEnvs() []string {
	var envs []string
	for _, k := range []string{
		"HOME",
		"USER",
		"PATH",
		"DOCKER_CONFIG",
		"AWS_REGION",
	} {
		if v, ok := os.LookupEnv(k); ok {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return envs
}

// dockerCmd wraps the docker CLI for building container images.
type dockerCmd struct {
	baseContainerCmd

	// useLegacyEngine disables BuildKit. When false, uses --progress=plain.
	useLegacyEngine bool
}

// DockerCmdConfig configures the docker command.
type DockerCmdConfig struct {
	Bin string

	UseLegacyEngine bool
}

// NewDockerCmd creates a new docker container command.
func NewDockerCmd(config *DockerCmdConfig) ContainerCmd {
	bin := config.Bin
	if bin == "" {
		bin = "docker"
	}
	envs := dockerCmdEnvs()

	if config.UseLegacyEngine {
		envs = append(envs, "DOCKER_BUILDKIT=0")
	} else {
		// Default using buildkit.
		envs = append(envs, "DOCKER_BUILDKIT=1")
	}

	return &dockerCmd{
		baseContainerCmd: baseContainerCmd{
			bin:  bin,
			envs: envs,
		},
		useLegacyEngine: config.UseLegacyEngine,
	}
}

// build overrides baseContainerCmd.build to add --progress=plain for BuildKit.
func (c *dockerCmd) build(in *buildInput, core *buildInputCore, hints *buildInputHints) error {
	if hints == nil {
		hints = newBuildInputHints(nil)
	}

	// Pull down the required images, and tag them properly.
	var froms []string
	for from := range core.Froms {
		froms = append(froms, from)
	}
	sort.Strings(froms)

	for _, from := range froms {
		src, ok := in.froms[from]
		if !ok {
			return fmt.Errorf("missing base image source for %q", from)
		}
		if src.local != "" { // local image, already ready.
			continue
		}
		if err := c.pull(src.src, src.name); err != nil {
			return fmt.Errorf("pull %s(%s): %w", src.name, src.src, err)
		}
	}
	// TODO(aslonnie): maybe recheck all the IDs of the from images?

	// Build the image.
	var args []string
	args = append(args, "build")
	if !c.useLegacyEngine {
		args = append(args, "--progress=plain")
	}
	args = append(args, "-f", core.Dockerfile)

	for _, t := range in.tagList() {
		args = append(args, "-t", t)
	}

	buildArgs := make(map[string]string)
	for k, v := range hints.BuildArgs {
		buildArgs[k] = v
	}
	// non-hint args can overwrite hint args
	for k, v := range core.BuildArgs {
		buildArgs[k] = v
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

	// read context from stdin
	args = append(args, "-")

	log.Printf("%s %s", c.bin, strings.Join(args, " "))

	buildCmd := c.cmd(args...)
	if in.context != nil {
		buildCmd.Stdin = newWriterToReader(in.context)
	}

	return buildCmd.Run()
}
