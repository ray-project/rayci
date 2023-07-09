package raycicmd

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

type config struct {
	name string

	ArtifactsBucket string `yaml:"artifacts_bucket"`

	CITemp string `yaml:"ci_temp"`

	AgentQueueMap map[string]string `yaml:"agent_queue_map"`

	Dockerless bool `yaml:"dockerless"`
}

func localDefaultConfig(envs Envs) *config {
	return &config{
		CITemp: filepath.Join(getEnv(envs, "HOME"), ".cache/rayci"),
	}
}

// builtin ray buildkite pipeline IDs.
const (
	rayBranchPipeline = "0183465b-c6fb-479b-8577-4cfd743b545d"

	rayPRPipeline  = "0183465f-a222-467a-b122-3b9ea3e68094"
	rayDevPipeline = "5b097a97-ad35-4443-9552-f5c413ead11c"
)

func ciDefaultConfig(envs Envs) *config {
	pipelineID := getEnv(envs, "BUILDKITE_PIPELINE_ID")
	if pipelineID == rayBranchPipeline {
		return &config{
			name: "ray-branch",

			ArtifactsBucket: "ray-ci-artifact-branch-public",
			CITemp:          "s3://ray-ci-artifact-branch-public/ci-temp/",

			AgentQueueMap: map[string]string{
				"default":   "runner_queue_branch",
				"small":     "runner_queue_small_branch",
				"medium":    "runner_queue_medium_branch",
				"large":     "runner_queue_branch",
				"gpu":       "gpu_runner_queue_branch",
				"gpu-large": "gpu_large_runner_queue_branch",
			},
		}
	}

	return &config{
		name:            "ray-pr",
		ArtifactsBucket: "ray-ci-artifact-pr-public",
		CITemp:          "s3://ray-ci-artifact-pr-public/ci-temp/",

		AgentQueueMap: map[string]string{
			"default":   "runner_queue_pr",
			"small":     "runner_queue_small_pr",
			"medium":    "runner_queue_medium_pr",
			"large":     "runner_queue_pr",
			"gpu":       "gpu_runner_queue_pr",
			"gpu-large": "gpu_large_runner_queue_pr",
		},
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
