package wanda

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
)

var supportedPlatforms = map[string]map[string]struct{}{
	"linux":   {"amd64": {}, "arm64": {}},
	"darwin":  {"arm64": {}},
	"windows": {"amd64": {}},
}

func checkPlatformSupport() error {
	archs, ok := supportedPlatforms[runtime.GOOS]
	if !ok {
		return fmt.Errorf("unsupported host OS: %s", runtime.GOOS)
	}
	if _, ok := archs[runtime.GOARCH]; !ok {
		return fmt.Errorf("unsupported host architecture: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return nil
}

// SupportedPlatformsList returns a formatted list of supported platforms.
func SupportedPlatformsList() string {
	var platforms []string
	for os, archs := range supportedPlatforms {
		for arch := range archs {
			platforms = append(platforms, os+"/"+arch)
		}
	}
	sort.Strings(platforms)
	return strings.Join(platforms, ", ")
}

// targetOS returns the OS to use for container image resolution.
// On macOS, containers are always linux; otherwise use the host OS.
func targetOS() string {
	if runtime.GOOS == "darwin" {
		return "linux"
	}
	return runtime.GOOS
}

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
