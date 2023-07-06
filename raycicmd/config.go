package raycicmd

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

type config struct {
	CITemp string `yaml:"ci_temp"`
}

func localDefaultConfig(envs Envs) *config {
	return &config{
		CITemp: filepath.Join(getEnv(envs, "HOME"), ".cache/rayci"),
	}
}

const (
	rayBranchPipeline = "0183465b-c6fb-479b-8577-4cfd743b545d"
)

func ciDefaultConfig(envs Envs) *config {
	pipelineID := getEnv(envs, "BUILDKITE_PIPELINE_ID")
	if pipelineID == rayBranchPipeline {
		return &config{
			CITemp: "s3://ray-ci-artifact-branch-public/ci-temp/",
		}
	}

	return &config{
		CITemp: "s3://ray-ci-artifact-pr-public/ci-temp/",
	}
}

func defaultConfig(envs Envs) *config {
	envCI := getEnv(envs, "CI")
	if envCI == "true" || envCI == "1" {
		return ciDefaultConfig(envs)
	}
	return localDefaultConfig(envs)
}

func loadConfig(f string, envs Envs) (*config, error) {
	if f == "" {
		return defaultConfig(envs), nil
	}

	bs, err := os.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	config := new(config)
	if err := yaml.Unmarshal(bs, config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return config, nil
}
