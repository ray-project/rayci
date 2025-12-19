package wanda

import (
	"fmt"
	"os"
)

func podmanCmdEnvs() []string {
	var envs []string
	for _, k := range []string{
		"HOME",
		"USER",
		"PATH",
		"XDG_RUNTIME_DIR",
		"CONTAINERS_CONF",
		"REGISTRY_AUTH_FILE",
	} {
		if v, ok := os.LookupEnv(k); ok {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return envs
}

// podmanCmd wraps the podman CLI for building container images.
type podmanCmd struct {
	baseContainerCmd
}

// PodmanCmdConfig configures the podman command.
type PodmanCmdConfig struct {
	Bin string
}

// NewPodmanCmd creates a new podman container command.
func NewPodmanCmd(config *PodmanCmdConfig) ContainerCmd {
	bin := config.Bin
	if bin == "" {
		bin = "podman"
	}
	envs := podmanCmdEnvs()

	return &podmanCmd{
		baseContainerCmd: baseContainerCmd{
			bin:  bin,
			envs: envs,
		},
	}
}
