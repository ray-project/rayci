package raycicmd

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
		"RAYCI_DISABLE_TEST_DB",
	},
	HookEnvKeys: []string{"RAYCI_CHECKOUT_DIR"},

	SkipTags: []string{"disabled"},
}

func makePRPipelineConfig(name string) *config {
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

		BuildEnvKeys: []string{
			"RAYCI_DISABLE_TEST_DB",
			"RAYCI_MICROCHECK_RUN",
		},
		HookEnvKeys: []string{"RAYCI_CHECKOUT_DIR"},

		TagFilterCommand: []string{"./ci/ci_tags_from_change.sh"},

		SkipTags: []string{"disabled", "skip-on-premerge"},

		ConcurrencyGroupPrefixes: []string{},
	}
	return config
}

var prPipelineConfig = makePRPipelineConfig("ray-pr")

func ciDefaultConfig(envs Envs) *config {
	pipelineID := getEnv(envs, "BUILDKITE_PIPELINE_ID")
	switch pipelineID {
	case rayBranchPipeline, rayV2PostmergePipeline, rayCIPipeline:
		return branchPipelineConfig
	case rayPRPipeline, rayV2PremergePipeline, rayDevPipeline:
		return prPipelineConfig
	case rayV2MicrocheckPipeline:
		c := makePRPipelineConfig("ray-pr-microcheck")
		c.MaxParallelism = 1
		c.NotifyOwnerOnFailure = true
		c.SkipTags = append(c.SkipTags, "skip-on-microcheck")

		return c
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
