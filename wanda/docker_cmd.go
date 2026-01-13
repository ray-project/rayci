package wanda

import (
	"fmt"
	"os"
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
	var extraFlags []string
	if !c.useLegacyEngine {
		extraFlags = append(extraFlags, "--progress=plain")
	}
	return c.doBuild(in, core, hints, extraFlags)
}
