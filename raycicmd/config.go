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

	// WorkDir is the working directory for the docker plugin to use.
	WorkDir string `yaml:"work_dir"`

	// AddCaps is the list of capabilities to add to the docker container.
	AddCaps []string `yaml:"add_caps"`
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
	// Optional. Soon to be deprecated in favor of TagRuleFiles.
	TagFilterCommand []string `yaml:"tag_filter_command"`

	// TagRuleFiles is a list of files that contain tag rules.
	// When any rule files are specified, a buildkite step is skipped if it is
	// tagged by something that is not returned by this command.
	//
	// Optional.
	TagRuleFiles []string `yaml:"tag_rule_files"`

	// SkipTags is the list of tags that will always be skipped.
	//
	// Optional.
	SkipTags []string `yaml:"skip_tags"`

	// AllowTriggerStep sets if it is allowed to have trigger steps in the
	// pipeline, default is false.
	//
	// Optional.
	AllowTriggerStep bool `yaml:"allow_trigger_step"`

	// ConcurrencyGroupPrefixes is the list of concurrency group name prefixes
	// that are allowed for the pipeline. If not set or set to NULL, all
	// concurrency group names are allowed.
	//
	// Note that yaml key name is "allow_concurrency_group_prefixes".
	//
	// Optional.
	ConcurrencyGroupPrefixes []string `yaml:"allow_concurrency_group_prefixes"`

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

	// NoTagMeansAlways sets if a step without any tags should be treated as
	// a step with a magic "always" tag, that will always be picked during the
	// conditional testing tag picking phase.
	NoTagMeansAlways bool `yaml:"no_tag_means_always"`
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

func loadConfigFromFile(f string) (*config, error) {
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
