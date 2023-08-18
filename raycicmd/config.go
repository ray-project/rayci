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

	CITemp      string `yaml:"ci_temp"`
	CIWorkRepo  string `yaml:"ci_work_repo"`
	ForgePrefix string `yaml:"forge_prefix"`

	BuilderQueues map[string]string `yaml:"builder_queues"`
	RunnerQueues  map[string]string `yaml:"runner_queues"`

	// Priority for builder command steps.
	BuilderPriority int `yaml:"builder_priority"`

	// Default priority for runner command steps.
	RunnerPriority int `yaml:"runner_priority"`

	// BuildkiteDir is the directory of buildkite pipeline files.
	BuildkiteDirs []string `yaml:"buildkite_dirs"`

	// ForgeDir is the directory of forge Dockerfile files.
	ForgeDirs []string `yaml:"forge_dirs"`

	// Env is the environment variables to set for runner steps.
	Env map[string]string `yaml:"env"`
}

func builderAgent(config *config) string {
	if config.BuilderQueues != nil {
		if q, ok := config.BuilderQueues["builder"]; ok {
			return q
		}
	}
	return ""
}

func localDefaultConfig(envs Envs) *config {
	return &config{
		CITemp: filepath.Join(getEnv(envs, "HOME"), ".cache/rayci"),
	}
}

// builtin ray buildkite pipeline IDs.
const (
	// v1 pipelines
	rayBranchPipeline = "0183465b-c6fb-479b-8577-4cfd743b545d"
	rayPRPipeline     = "0183465f-a222-467a-b122-3b9ea3e68094"

	// v2 pipelines
	rayV2PostmergePipeline = "0189e759-8c96-4302-b6b5-b4274406bf89"
	rayV2PremergePipeline  = "0189942e-0876-4b8f-80a4-617f988ec59b"

	// dev only
	rayDevPipeline = "5b097a97-ad35-4443-9552-f5c413ead11c"

	// pipeline for this repo
	rayCIPipeline = "01890992-4f1a-4cc4-b99d-f2da360eb3ab"
)

const (
	rayCIECR = "029272617770.dkr.ecr.us-west-2.amazonaws.com"

	rayBazelBuildCache = "https://bazel-cache-dev.s3.us-west-2.amazonaws.com"

	defaultForgePrefix = "cr.ray.io/rayproject/"
)

var defaultForgeDirs = []string{".buildkite/forge", "ci/forge", "ci/v2/forge"}

var branchPipelineConfig = &config{
	name: "ray-branch",

	ArtifactsBucket: "ray-ci-artifact-branch-public",

	CITemp:      "s3://ray-ci-artifact-branch-public/ci-temp/",
	CIWorkRepo:  rayCIECR + "/rayci_temp_branch",
	ForgePrefix: defaultForgePrefix,

	BuilderQueues: map[string]string{
		"builder":       "builder_queue_branch",
		"builder-arm64": "builder_queue_arm64_branch",
	},

	RunnerQueues: map[string]string{
		"default":   "runner_queue_small_branch",
		"small":     "runner_queue_small_branch",
		"medium":    "runner_queue_medium_branch",
		"large":     "runner_queue_branch",
		"gpu":       "gpu_runner_queue_branch",
		"gpu-large": "gpu_large_runner_queue_branch",

		"medium-arm64": "runner_queue_arm64_medium_branch",
	},

	ForgeDirs: defaultForgeDirs,

	Env: map[string]string{
		"BUILDKITE_BAZEL_CACHE_URL": rayBazelBuildCache,
	},
}

var prPipelineConfig = &config{
	name: "ray-pr",

	ArtifactsBucket: "ray-ci-artifact-pr-public",

	CITemp:      "s3://ray-ci-artifact-pr-public/ci-temp/",
	CIWorkRepo:  rayCIECR + "/rayci_temp_pr",
	ForgePrefix: defaultForgePrefix,

	BuilderQueues: map[string]string{
		"builder":       "builder_queue_pr",
		"builder-arm64": "builder_queue_arm64_pr",
	},

	RunnerQueues: map[string]string{
		"default":   "runner_queue_small_pr",
		"small":     "runner_queue_small_pr",
		"medium":    "runner_queue_medium_pr",
		"large":     "runner_queue_pr",
		"gpu":       "gpu_runner_queue_pr",
		"gpu-large": "gpu_large_runner_queue_pr",

		"medium-arm64": "runner_queue_arm64_medium_pr",
	},

	BuilderPriority: 1,
	RunnerPriority:  1,

	ForgeDirs: defaultForgeDirs,

	Env: map[string]string{
		"BUILDKITE_BAZEL_CACHE_URL": rayBazelBuildCache,
		"BUILDKITE_CACHE_READONLY":  "true",
	},
}

func ciDefaultConfig(envs Envs) *config {
	pipelineID := getEnv(envs, "BUILDKITE_PIPELINE_ID")
	switch pipelineID {
	case rayBranchPipeline, rayV2PostmergePipeline, rayCIPipeline:
		return branchPipelineConfig
	case rayPRPipeline, rayV2PremergePipeline, rayDevPipeline:
		return prPipelineConfig
	}

	// By default, assume it is less privileged.
	return prPipelineConfig
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
