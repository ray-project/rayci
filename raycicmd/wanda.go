package raycicmd

import (
	"fmt"
	"log"
	"path"
	"sort"
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

	launcherBranch string

	matrix any
}

func wandaCommands(br string) []string {
	if br == "" {
		br = "stable"
	}
	runWandaURLPath := path.Join("ray-project/rayci", br, "run_wanda.sh")
	runWandaURL := rawGitHubURL + runWandaURLPath

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
		"commands": wandaCommands(s.launcherBranch),
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

type wandaConverter struct {
	config *config
	info   *buildInfo
	envMap map[string]string
}

func newWandaConverter(
	config *config, info *buildInfo, envMap map[string]string,
) *wandaConverter {
	return &wandaConverter{
		config: config,
		info:   info,
		envMap: envMap,
	}
}

func (c *wandaConverter) match(step map[string]any) bool {
	_, ok := step["wanda"]
	return ok
}

type envEntry struct {
	k string
	v string
}

func parseStepEnvs(v any) ([]*envEntry, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not a map")
	}

	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var entries []*envEntry
	for _, k := range keys {
		str, ok := (m[k]).(string)
		if !ok {
			return nil, fmt.Errorf(
				"value of env %q is not a string", k,
			)
		}
		entries = append(entries, &envEntry{k: k, v: str})
	}
	return entries, nil
}

func (c *wandaConverter) convert(id string, step map[string]any) (
	map[string]any, error,
) {
	if err := checkStepKeys(step, wandaStepAllowedKeys); err != nil {
		return nil, fmt.Errorf("check wanda step keys: %w", err)
	}
	name, ok := stringInMap(step, "name")
	if !ok {
		return nil, fmt.Errorf("wanda step missing name")
	}
	file, ok := stringInMap(step, "wanda")
	if !ok {
		return nil, fmt.Errorf("wanda step file is not a string")
	}
	label, _ := stringInMap(step, "label")
	instanceType, _ := stringInMap(step, "instance_type")

	var matrix any
	if m, ok := step["matrix"]; ok {
		matrix = m
	}

	envs := copyEnvMap(c.envMap)
	if stepEnvs, ok := step["env"]; ok {
		entries, err := parseStepEnvs(stepEnvs)
		if err != nil {
			return nil, fmt.Errorf("parse wanda step envs: %w", err)
		}
		for _, entry := range entries {
			if _, ok := envs[entry.k]; ok {
				log.Printf("wanda step env %q ignored", entry.k)
			} else {
				envs[entry.k] = entry.v
			}
		}
	}

	s := &wandaStep{
		name:           name,
		label:          label,
		file:           file,
		buildID:        c.info.buildID,
		envs:           envs,
		ciConfig:       c.config,
		matrix:         matrix,
		instanceType:   instanceType,
		launcherBranch: c.info.launcherBranch,
	}
	if dependsOn, ok := step["depends_on"]; ok {
		s.dependsOn = dependsOn
	}

	return s.buildkiteStep(), nil
}
