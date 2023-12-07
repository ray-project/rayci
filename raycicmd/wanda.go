package raycicmd

import (
	"fmt"
	"os"
)

const rawGitHubURL = "https://raw.githubusercontent.com/"
const defaultBuilderType = "builder"

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

func getWandaCommands() []string {
	rayciBranch := os.Getenv("RAYCI_BRANCH")
	if rayciBranch == "" {
		rayciBranch = "stable"
	}
	runWandaURL := rawGitHubURL + "ray-project/rayci/" + rayciBranch + "/run_wanda.sh"

	return []string{
		fmt.Sprintf(`bash -c "curl -sfL %s > /tmp/run_wanda.sh"`, runWandaURL),
		`bash /tmp/run_wanda.sh -rayci`,
	}
}

func (s *wandaStep) buildkiteStep() map[string]any {
	instanceType := s.instanceType
	if instanceType == "" {
		instanceType = defaultBuilderType
	}
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
		"commands": getWandaCommands(),
		"env":      envs,
		"retry":    defaultBuilderRetry,

		"timeout_in_minutes": defaultTimeoutInMinutes,
	}

	if s.dependsOn != nil {
		bkStep["depends_on"] = s.dependsOn
	}

	agentQueue := builderAgent(s.ciConfig, instanceType)
	if agentQueue == skipQueue {
		bkStep["skip"] = true
	} else if agentQueue != "" {
		bkStep["agents"] = newBkAgents(agentQueue)
	}

	if p := s.ciConfig.BuilderPriority; p != 0 {
		bkStep["priority"] = p
	}
	if s.matrix != nil {
		bkStep["matrix"] = s.matrix
	}
	return bkStep
}
