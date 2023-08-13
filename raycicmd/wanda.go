package raycicmd

import (
	"fmt"
)

const rawGitHubURL = "https://raw.githubusercontent.com/"
const runWandaURL = rawGitHubURL + "ray-project/rayci/lonnie-x/run_wanda.sh"

var wandaCommands = []string{
	fmt.Sprintf(`curl -sfL "%s" > /tmp/run_wanda.sh`, runWandaURL),
	`RAYCI_BRANCH=lonnie-x /bin/bash /tmp/run_wanda.sh -rayci`,
}

type wandaStep struct {
	name    string
	file    string
	buildID string

	dependsOn any

	envs     map[string]string
	ciConfig *config
}

func (s *wandaStep) buildkiteStep() map[string]any {
	agent := builderAgent(s.ciConfig)

	envs := make(map[string]string)
	for k, v := range s.envs {
		envs[k] = v
	}
	envs["RAYCI_WANDA_NAME"] = s.name
	envs["RAYCI_WANDA_FILE"] = s.file

	bkStep := map[string]any{
		"label":    "wanda: " + s.name,
		"key":      s.name,
		"commands": wandaCommands,
		"env":      envs,
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
	return bkStep
}
