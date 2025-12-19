package wanda

import (
	"fmt"
	"runtime"
	"strings"
)

// ContainerRuntime specifies which container runtime to use.
type ContainerRuntime int

const (
	// RuntimeDocker uses Docker as the container runtime.
	RuntimeDocker ContainerRuntime = iota
	// RuntimePodman uses Podman as the container runtime.
	RuntimePodman
)

// ForgeConfig is a configuration for a forge to build container images.
type ForgeConfig struct {
	WorkDir    string
	WorkRepo   string
	NamePrefix string
	BuildID    string
	Epoch      string

	RayCI   bool
	Rebuild bool

	ReadOnlyCache bool

	// ContainerRuntime specifies which container runtime to use.
	// Defaults to RuntimeDocker.
	ContainerRuntime ContainerRuntime

	// ContainerBin is the path to the container runtime binary.
	// If empty, uses the default binary name ("docker" or "podman").
	ContainerBin string
}

// newContainerCmd creates a ContainerCmd based on the config settings.
func (c *ForgeConfig) newContainerCmd() ContainerCmd {
	return NewDockerCmd(&DockerCmdConfig{
		Bin:             c.ContainerBin,
		UseLegacyEngine: runtime.GOOS == "windows",
	})
}

func (c *ForgeConfig) isRemote() bool { return c.WorkRepo != "" }

func (c *ForgeConfig) workRepo() string {
	if c.WorkRepo != "" {
		return c.WorkRepo
	}
	return "localhost:5000/rayci-work"
}

func (c *ForgeConfig) workTag(name string) string {
	workRepo := c.workRepo()
	if c.BuildID != "" {
		return fmt.Sprintf("%s:%s-%s", workRepo, c.BuildID, name)
	}
	return fmt.Sprintf("%s:%s", workRepo, name)
}

func (c *ForgeConfig) cacheTag(inputDigest string) string {
	if _, d, ok := strings.Cut(inputDigest, ":"); ok {
		inputDigest = d
	}
	return fmt.Sprintf("%s:z-%s", c.workRepo(), inputDigest)
}
