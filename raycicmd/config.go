package raycicmd

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

// skipQueue is the queue name for skipping a buildkite runner job.
const skipQueue = "~"

type dockerPluginConfig struct {
	// AllowMountDockerSocket sets if it is allowed for jobs to mount the
	// buildkite agent. This should only be set for pipelines where all builds
	// on the pipeline are trusted.
	AllowMountBuildkiteAgent bool `yaml:"allow_mount_buildkite_agent"`
}

type config struct {
	// name is a private field that is only used for internal testing.
	name string

	// ArtifactsBucket specifies the S3 bucket to save build artifacts.
	//
	// Optional but highly recommended.
	ArtifactsBucket string `yaml:"artifacts_bucket"`

	// CITemp specifies the location to save temp files that can be used accross
	// build jobs. Might not be usable on PR/premerge builds.
	//
	// Required.
	CITemp string `yaml:"ci_temp"`

	// CIWorkRepo is a container repository for saving wanda built images, which
	// can be used as environment images for build jobs.
	//
	// Required.
	CIWorkRepo string `yaml:"ci_work_repo"`

	// ForgePrefix is a prefix for wanda-built forge images. This serves as a
	// globally unique namespace for container image references, and ideally
	// should be set to a domain that does not exist on the public Internet,
	// to avoid the container builder from pulling public images when building.
	// The prefix will be auto appended recognized to wanda built images before
	// the wanda image name.
	//
	// Optional but highly recommended.
	ForgePrefix string `yaml:"forge_prefix"`

	// BuilderQueues is a mapping from job instance types to buildkite agent
	// queues for builders. If not set, the agent queue will be omitted, and
	// the default queue will be used.
	//
	// Optional but highly recommended.
	BuilderQueues map[string]string `yaml:"builder_queues"`

	// RunnerQueues is a mapping from job instance types to buildkite agent
	// queues for runners. If not set, the agent queue will be omitted, and the
	// default queue will be used. If it mapped to "~", then the job will be
	// marked with `skip: true`.
	//
	// Optional but highly recommended.
	RunnerQueues map[string]string `yaml:"runner_queues"`

	// BuilderPriority is the job priority for builder command steps.
	//
	// Optional.
	BuilderPriority int `yaml:"builder_priority"`

	// RunnerPriority is the job priority for runner command steps.
	//
	// Optional.
	RunnerPriority int `yaml:"runner_priority"`

	// BuildkiteDir is the directory of buildkite pipeline files.
	//
	// Optional. Default is `[]string{".buildkite"}` .
	BuildkiteDirs []string `yaml:"buildkite_dirs"`

	// Env is the environment variables to set for runner steps.
	//
	// Optional.
	Env map[string]string `yaml:"env"`

	// BuildEnvKeys is a list of environment variable keys to pass into
	// build jobs from the buildkite build.
	//
	// Optional.
	BuildEnvKeys []string `yaml:"build_env_keys"`

	// HookEnvKeys is the list of environment variable keys to pass into
	// build jobs from buildkite hooks.
	//
	// Optional.
	HookEnvKeys []string `yaml:"hook_env_keys"`

	// TagFilterCommand is a command invocation that populates the buildkite
	// step tags. When this command is specified, a buildkite step is skipped
	// if it is tagged by something that is not returned by this command.
	//
	// If the first arg (the binary) starts with "./", then it is treated as
	// a file path relative the repsitory root, and it will be checked if the
	// file exists. If the file does not exist, then the command is ignored
	// and all steps will be executed.
	//
	// Optional.
	TagFilterCommand []string `yaml:"tag_filter_command"`

	// SkipTags is the list of tags that will always be skipped.
	//
	// Optional.
	SkipTags []string `yaml:"skip_tags"`

	// AllowTriggerStep sets if it is allowed to have trigger steps in the
	// pipeline, default is false.
	//
	// Optional.
	AllowTriggerStep bool `yaml:"allow_trigger_step"`

	// AllowConcurrencyGroup is the list of concurrency group name prefixes that are
	// allowed for the pipeline. If not set or set to NULL, all concurrency group names
	// are allowed.
	//
	// Optional.
	AllowConcurrencyGroupPrefixes []string `yaml:"allow_concurrency_group_prefixes"`

	// MaxParallelism is the maximum number of parallel jobs that can be run in
	// the pipeline. If a bigger number of parallelism is requested, it will be
	// capped to this number.
	//
	// Optional.
	MaxParallelism int `yaml:"max_parallelism"`

	// NotifyOwnerOnFailure sets if the owner of the build should be notified
	// when a build fails.
	//
	// Optional.
	NotifyOwnerOnFailure bool `yaml:"notify_owner_on_failure"`

	// DockerPlugin contains additional docker plugin configs, to fine tune
	// the docker plugin's behavior.
	//
	// Optional.
	DockerPlugin *dockerPluginConfig `yaml:"docker_plugin"`
}

func builderAgent(config *config, instanceType string) string {
	if config.BuilderQueues != nil {
		if q, ok := config.BuilderQueues[instanceType]; ok {
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
	rayV2PostmergePipeline  = "0189e759-8c96-4302-b6b5-b4274406bf89"
	rayV2PremergePipeline   = "0189942e-0876-4b8f-80a4-617f988ec59b"
	rayV2MicrocheckPipeline = "018f4f1e-1b73-4906-9802-92422e3badaa"

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

var branchPipelineConfig = &config{
	name: "ray-branch",

	ArtifactsBucket: "ray-ci-artifact-branch-public",

	CITemp:      "s3://ray-ci-artifact-branch-public/ci-temp/",
	CIWorkRepo:  rayCIECR + "/rayproject/citemp",
	ForgePrefix: defaultForgePrefix,

	BuilderQueues: map[string]string{
		"builder":         "builder_queue_branch",
		"builder-arm64":   "builder_queue_arm64_branch",
		"builder-windows": "builder_queue_windows_branch",
	},

	RunnerQueues: map[string]string{
		"default":        "runner_queue_small_branch",
		"small":          "runner_queue_small_branch",
		"medium":         "runner_queue_medium_branch",
		"large":          "runner_queue_branch",
		"gpu":            "gpu_runner_queue_branch",
		"gpu-large":      "gpu_large_runner_queue_branch",
		"trainium":       "trainium_runner_queue_branch",
		"windows":        "windows_queue_branch",
		"macos":          "macos-branch",
		"macos-arm64":    "macos-branch-arm64",
		"medium-arm64":   "runner_queue_arm64_medium_branch",
		"release":        "release_queue_small",
		"release-medium": "release_queue_medium",
	},

	Env: map[string]string{
		"BUILDKITE_BAZEL_CACHE_URL": rayBazelBuildCache,
	},

	BuildEnvKeys: []string{
		"RAYCI_SCHEDULE",
		"RAYCI_BISECT_TEST_TARGET",
	},
	HookEnvKeys: []string{"RAYCI_CHECKOUT_DIR"},

	SkipTags: []string{"disabled"},
}

func prPipelineConfig(
	name string,
	extraEnv map[string]string,
	maxParralelism int,
) *config {
	config := &config{
		name: name,

		ArtifactsBucket: "ray-ci-artifact-pr-public",

		CITemp:      "s3://ray-ci-artifact-pr-public/ci-temp/",
		CIWorkRepo:  rayCIECR + "/rayproject/citemp",
		ForgePrefix: defaultForgePrefix,

		BuilderQueues: map[string]string{
			"builder":         "builder_queue_pr",
			"builder-arm64":   "builder_queue_arm64_pr",
			"builder-windows": "builder_queue_windows_pr",
		},

		RunnerQueues: map[string]string{
			"default":     "runner_queue_small_pr",
			"small":       "runner_queue_small_pr",
			"medium":      "runner_queue_medium_pr",
			"large":       "runner_queue_pr",
			"gpu":         "gpu_runner_queue_pr",
			"gpu-large":   "gpu_large_runner_queue_pr",
			"trainium":    "trainium_runner_queue_pr",
			"windows":     "windows_queue_pr",
			"macos":       "macos",
			"macos-arm64": "macos-pr-arm64",

			"medium-arm64": "runner_queue_arm64_medium_pr",
		},

		BuilderPriority: 1,
		RunnerPriority:  1,

		Env: map[string]string{
			"BUILDKITE_BAZEL_CACHE_URL": rayBazelBuildCache,
			"BUILDKITE_CACHE_READONLY":  "true",
		},

		HookEnvKeys: []string{"RAYCI_CHECKOUT_DIR"},

		TagFilterCommand: []string{"./ci/ci_tags_from_change.sh"},

		SkipTags: []string{"disabled", "skip-on-premerge"},

		AllowConcurrencyGroupPrefixes: []string{},
	}
	for k, v := range extraEnv {
		config.Env[k] = v
	}
	if maxParralelism > 0 {
		config.MaxParallelism = maxParralelism
	}
	return config
}

func ciDefaultConfig(envs Envs) *config {
	pipelineID := getEnv(envs, "BUILDKITE_PIPELINE_ID")
	switch pipelineID {
	case rayBranchPipeline, rayV2PostmergePipeline, rayCIPipeline:
		return branchPipelineConfig
	case rayPRPipeline, rayV2PremergePipeline, rayDevPipeline:
		return prPipelineConfig("ray-pr", nil, -1)
	case rayV2MicrocheckPipeline:
		c := prPipelineConfig(
			"ray-pr-microcheck",
			map[string]string{"RAYCI_MICROCHECK_RUN": "1"},
			1,
		)
		c.NotifyOwnerOnFailure = true
		c.SkipTags = append(c.SkipTags, "skip-on-microcheck")

		return c
	}

	// By default, assume it is less privileged.
	return prPipelineConfig("ray-pr", nil, -1)
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
