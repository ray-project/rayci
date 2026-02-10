package wanda

import (
	"fmt"
	"strings"
)

// ForgeConfig is a configuration for a forge to build container images.
type ForgeConfig struct {
	WorkDir        string
	DockerBin      string
	WorkRepo       string
	NamePrefix     string
	BuildID        string
	Epoch          string
	WandaSpecsFile string
	EnvFile        string
	ArtifactsDir   string

	RayCI   bool
	Rebuild bool

	ReadOnlyCache bool
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
