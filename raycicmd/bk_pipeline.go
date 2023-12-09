package raycicmd

import (
	"fmt"
	"time"
)

type bkPipelineGroup struct {
	Group     string   `yaml:"group,omitempty"`
	Key       string   `yaml:"key,omitempty"`
	DependsOn []string `yaml:"depends_on,omitempty"`
	Steps     []any    `yaml:"steps,omitempty"`
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
	"BUILDKITE_HOOK_WORKING_DIR",
	"BUILDKITE_LABEL",
	"BUILDKITE_RETRY_COUNT",
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
}

type stepDockerPluginConfig struct {
	extraEnvs           []string
	mountBuildkiteAgent bool
	publishTCPPorts     []string
}

func dockerPluginEnvList(config *stepDockerPluginConfig) []string {
	envs := append([]string(nil), buildkiteEnvs...)
	if len(config.extraEnvs) > 0 {
		envs = append(envs, config.extraEnvs...)
	}
	return envs
}

func makeRayWindowsDockerPlugin(image string, config *stepDockerPluginConfig) map[string]any {
	envs := append([]string(nil), buildkiteEnvs...)
	if len(config.extraEnvs) > 0 {
		envs = append(envs, config.extraEnvs...)
	}

	m := map[string]any{
		"image":          image,
		"shell":          []string{"bash", "-c"},
		"shm-size":       "2.5gb",
		"mount-checkout": true,
		"environment":    envs,
		"volumes": []string{
			`\\.\pipe\docker_engine:\\.\pipe\docker_engine`,
		},
	}

	return m
}

func makeRayDockerPlugin(
	image string, config *stepDockerPluginConfig,
) map[string]any {
	envs := dockerPluginEnvList(config)

	m := map[string]any{
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

	if config.mountBuildkiteAgent {
		m["mount_buildkite_agent"] = true
	}
	if len(config.publishTCPPorts) > 0 {
		var publish []string
		for _, p := range config.publishTCPPorts {
			publish = append(publish, fmt.Sprintf("127.0.0.1:%s:%s/tcp", p, p))
		}
		m["publish"] = publish
	}

	return m
}

var (
	defaultRayRetry = map[string]any{
		"manual": map[string]any{"permit_on_passed": true},
		"automatic": []any{
			map[string]any{"exit_status": 1, "limit": 1},
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
