package raycicmd

import (
	"fmt"
)

const rawGitHubURL = "https://raw.githubusercontent.com/"
const runWandaURL = rawGitHubURL +
	"ray-project/rayci/$${RAYCI_BRANCH:-stable}/run_wanda.sh"
const defaultBuilderType = "builder"

var wandaCommands = []string{
	fmt.Sprintf(`curl -sfL "%s" > /tmp/run_wanda.sh`, runWandaURL),
	`/bin/bash /tmp/run_wanda.sh -rayci`,
}

type wandaStep struct {
	name         string
	file         string
	buildID      string
	label        string
	instanceType string

	dependsOn any

	envs     map[string]string
	ciConfig *config

	matrix any
}

func (s *wandaStep) buildkiteStep() map[string]any {
	instanceType := s.instanceType
	if instanceType == "" {
		instanceType = defaultBuilderType
	}
	agent := builderAgent(s.ciConfig, instanceType)

	envs := make(map[string]string)
	for k, v := range s.envs {
		envs[k] = v
	}
	envs["RAYCI_WANDA_NAME"] = s.name
	envs["RAYCI_WANDA_FILE"] = s.file

	label := s.label
	if label == "" {
		label = "wanda: " + s.name
	}

	bkStep := map[string]any{
		"label":    label,
		"key":      s.name,
		"commands": wandaCommands,
		"env":      envs,
		"retry":    defaultBuilderRetry,

		"timeout_in_minutes": defaultTimeoutInMinutes,
	}

	if s.dependsOn != nil {
		bkStep["depends_on"] = s.dependsOn
	}
	if agent != "" {
		bkStep["agents"] = newBkAgents(agent)
	}
	if p := s.ciConfig.BuilderPriority; p != 0 {
		bkStep["priority"] = p
	}
	if s.matrix != nil {
		bkStep["matrix"] = s.matrix
	}
	return bkStep
}
