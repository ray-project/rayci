package raycicmd

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("load ray branch CI config", func(t *testing.T) {
		envs := newEnvsMap(map[string]string{
			"CI":                    "true",
			"BUILDKITE_PIPELINE_ID": rayBranchPipeline,
		})

		config := defaultConfig(envs)
		if want := "ray-branch"; config.name != want {
			t.Errorf("config got %q, want %q", config.name, want)
		}
		envHasKey := false
		for _, value := range config.BuildEnvKeys {
			if value == "RAYCI_BISECT_TEST_TARGET" {
				envHasKey = true
			}
		}
		if !envHasKey {
			t.Errorf("config does not have key RAYCI_BISECT_TEST_TARGET")
		}
	})

	t.Run("load ray microcheck CI config", func(t *testing.T) {
		envs := newEnvsMap(map[string]string{
			"CI":                    "true",
			"BUILDKITE_PIPELINE_ID": rayV2MicrocheckPipeline,
		})

		config := defaultConfig(envs)
		if want := "ray-pr-microcheck"; config.name != want {
			t.Errorf("config got %q, want %q", config.name, want)
		}
		val, ok := config.Env["BUILDKITE_BAZEL_CACHE_URL"]
		if !ok || val != rayBazelBuildCache {
			t.Errorf(
				"config.Env.BUILDKITE_BAZEL_CACHE_URL got %q, want %q",
				val, rayBazelBuildCache,
			)
		}
	})

	t.Run("load ray PR CI config", func(t *testing.T) {
		envs := newEnvsMap(map[string]string{
			"CI":                    "true",
			"BUILDKITE_PIPELINE_ID": rayPRPipeline,
		})

		config := defaultConfig(envs)
		if want := "ray-pr"; config.name != want {
			t.Errorf("config got %q, want %q", config.name, want)
		}
		val, ok := config.Env["RAYCI_MICROCHECK_RUN"]
		if ok {
			t.Errorf("config.Env.RAYCI_MICROCHECK_RUN got %q, want missing", val)
		}
	})

	t.Run("load local config", func(t *testing.T) {
		envs := newEnvsMap(map[string]string{
			"HOME": "/opt/fakehome",
		})

		config := defaultConfig(envs)
		const ciTemp = "/opt/fakehome/.cache/rayci"
		if config.CITemp != ciTemp {
			t.Errorf("config got %q, want %q", config.CITemp, ciTemp)
		}
	})
}
