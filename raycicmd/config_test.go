package raycicmd

import (
	"testing"

	"os"
	"path/filepath"
)

func TestLoadConfig(t *testing.T) {
	t.Run("load ray branch CI config", func(t *testing.T) {
		envs := newEnvsMap(map[string]string{
			"CI":                    "true",
			"BUILDKITE_PIPELINE_ID": rayBranchPipeline,
		})

		config, err := loadConfig("", envs)
		if err != nil {
			t.Fatalf("load config: %v", err)
		}
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

	t.Run("load ray PR CI config", func(t *testing.T) {
		envs := newEnvsMap(map[string]string{
			"CI":                    "true",
			"BUILDKITE_PIPELINE_ID": rayPRPipeline,
		})

		config, err := loadConfig("", envs)
		if err != nil {
			t.Fatalf("load config: %v", err)
		}
		if want := "ray-pr"; config.name != want {
			t.Errorf("config got %q, want %q", config.name, want)
		}
	})

	t.Run("load config file", func(t *testing.T) {
		tmp := t.TempDir()
		const bs = `ci_temp: "s3://fake/ci-temp"`
		file := filepath.Join(tmp, "config.yaml")
		if err := os.WriteFile(file, []byte(bs), 0600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		config, err := loadConfig(file, &osEnvs{})
		if err != nil {
			t.Fatalf("load config: %v", err)
		}
		const ciTemp = "s3://fake/ci-temp"
		if config.CITemp != ciTemp {
			t.Errorf("config got %q, want %q", config.CITemp, ciTemp)
		}
	})

	t.Run("load local config", func(t *testing.T) {
		envs := newEnvsMap(map[string]string{
			"HOME": "/opt/fakehome",
		})

		config, err := loadConfig("", envs)
		if err != nil {
			t.Fatalf("load config: %v", err)
		}
		const ciTemp = "/opt/fakehome/.cache/rayci"
		if config.CITemp != ciTemp {
			t.Errorf("config got %q, want %q", config.CITemp, ciTemp)
		}
	})
}

func TestBuilderAgent(t *testing.T) {
	c := &config{
		BuilderQueues: map[string]string{
			"builder": "mybuilder",
			"other":   "otherbuilder",
		},
	}

	q := builderAgent(c, "builder")
	if q != "mybuilder" {
		t.Errorf("builder agent got %q, want `mybuilder`", q)
	}
	q = builderAgent(c, "other")
	if q != "otherbuilder" {
		t.Errorf("builder agent got %q, want `otherbuilder`", q)
	}
}
