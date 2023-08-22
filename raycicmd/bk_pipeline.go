package raycicmd

import (
	"time"
)

type bkPipelineGroup struct {
	Group string `yaml:"group,omitempty"`
	Key   string `yaml:"key,omitempty"`
	Steps []any  `yaml:"steps,omitempty"`
}

type bkPipeline struct {
	Steps []*bkPipelineGroup `yaml:"steps,omitempty"`
}

func newBkAgents(queue string) map[string]any {
	return map[string]any{"queue": queue}
}

func makeNoopBkPipeline(q string) *bkPipeline {
	step := map[string]any{"command": "echo no pipeline steps"}
	if q != "" {
		step["agents"] = newBkAgents(q)
	}

	return &bkPipeline{
		Steps: []*bkPipelineGroup{{
			Group: "noop",
			Steps: []any{step},
		}},
	}
}

var buildkiteEnvs = []string{
	"CI",
	"BUILDKITE",
	"BUILDKITE_BRANCH",
	"BUILDKITE_COMMIT",
	"BUILDKITE_LABEL",
	"BUILDKITE_PIPELINE_ID",
	"BUILDKITE_PIPELINE_SLUG",
	"BUILDKITE_BUILD_ID",
	"BUILDKITE_BUILD_NUMBER",
	"BUILDKITE_BUILD_URL",
	"BUILDKITE_JOB_ID",
	"BUILDKITE_PARALLEL_JOB",
	"BUILDKITE_PARALLEL_JOB_COUNT",
	"BUILDKITE_PULL_REQUEST",
	"BUILDKITE_MESSAGE",
	"RAYCI_CHECKOUT_DIR",
}

func makeRayDockerPlugin(image string, extraEnvs []string) map[string]any {
	envs := append([]string(nil), buildkiteEnvs...)
	envs = append(envs, extraEnvs...)

	return map[string]any{
		"image":         image,
		"shell":         []string{"/bin/bash", "-elic"},
		"workdir":       "/ray",
		"add-caps":      []string{"SYS_PTRACE", "SYS_ADMIN", "NET_ADMIN"},
		"security-opts": []string{"apparmor=unconfined"},

		"volumes": []string{
			"/var/run/docker.sock:/var/run/docker.sock",
			"/tmp/artifacts:/artifact-mount",
		},

		"environment": envs,
	}
}

var (
	defaultRayRetry = map[string]any{
		"manual": map[string]any{"permit_on_passed": true},
		"automatic": []any{
			map[string]any{"exit_status": -1, "limit": 3},
			map[string]any{"exit_status": 255, "limit": 3},
		},
	}
	defaultBuilderRetry = map[string]any{
		"automatic": map[string]any{"limit": 1},
	}

	defaultTimeoutInMinutes = int((5 * time.Hour).Minutes())

	defaultArtifactPaths = []string{"/tmp/artifacts/**/*"}
)
