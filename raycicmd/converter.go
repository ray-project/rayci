package raycicmd

import (
	"fmt"
	"sort"
)

type converter struct {
	config  *config
	buildID string

	ciTempForBuild string

	envMap map[string]string
}

func newConverter(config *config, buildID string) *converter {
	c := &converter{
		config:  config,
		buildID: buildID,

		ciTempForBuild: config.CITemp + buildID + "/",
	}

	envMap := make(map[string]string)
	envMap["RAYCI_BUILD_ID"] = buildID
	envMap["RAYCI_WORK_REPO"] = config.CIWorkRepo
	envMap["RAYCI_TEMP"] = c.ciTempForBuild
	if config.ForgePrefix != "" {
		envMap["RAYCI_FORGE_PREFIX"] = config.ForgePrefix
	}
	for k, v := range c.config.Env {
		envMap[k] = v
	}

	c.envMap = envMap

	return c
}

func (c *converter) envMapCopy() map[string]string {
	m := make(map[string]string)
	for k, v := range c.envMap {
		m[k] = v
	}
	return m
}

func (c *converter) mapAgent(instanceType string) (string, error) {
	if instanceType == "" {
		instanceType = "default"
	}
	if q, ok := c.config.RunnerQueues[instanceType]; ok {
		return q, nil
	}
	return "", fmt.Errorf("unknown instance type %q", instanceType)
}

func (c *converter) jobEnvImage(name string) string {
	if name == "" {
		name = "forge"
	}

	return fmt.Sprintf("%s:%s-%s", c.config.CIWorkRepo, c.buildID, name)
}

const dockerPlugin = "docker#v5.8.0"

func (c *converter) convertPipelineStep(step map[string]any) (
	map[string]any, error,
) {
	if _, ok := step["wait"]; ok {
		// a wait step
		if err := checkStepKeys(step, waitStepAllowedKeys); err != nil {
			return nil, fmt.Errorf("check wait step keys: %w", err)
		}
		return cloneMap(step), nil
	}
	// special steps for building container images.
	if _, ok := step["wanda"]; ok {
		// a wanda step
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

		s := &wandaStep{
			name:     name,
			file:     file,
			buildID:  c.buildID,
			envs:     c.envMap,
			ciConfig: c.config,
		}
		if dependsOn, ok := step["depends_on"]; ok {
			s.dependsOn = dependsOn
		}

		return s.buildkiteStep(), nil
	}

	// a normal command step
	if err := checkStepKeys(step, commandStepAllowedKeys); err != nil {
		return nil, fmt.Errorf("check command step keys: %w", err)
	}

	queue, _ := stringInMapAnyKey(step, "queue", "instance_type")
	agentQueue, err := c.mapAgent(queue)
	if err != nil {
		return nil, fmt.Errorf("map agent: %w", err)
	}

	result := cloneMapExcept(step, commandStepDropKeys)

	result["agents"] = newBkAgents(agentQueue)
	result["retry"] = defaultRayRetry
	result["timeout_in_minutes"] = defaultTimeoutInMinutes
	result["artifact_paths"] = defaultArtifactPaths

	jobEnv, _ := stringInMap(step, "job_env")
	jobEnvImage := c.jobEnvImage(jobEnv)

	priority, ok := step["priority"]
	if !ok {
		priority = c.config.RunnerPriority
	}
	if priority != 0 {
		result["priority"] = priority
	}

	envMap := make(map[string]string)
	envMap["RAYCI_BUILD_ID"] = c.buildID
	envMap["RAYCI_TEMP"] = c.ciTempForBuild
	for k, v := range c.config.Env {
		envMap[k] = v
	}

	result["env"] = c.envMapCopy()

	var envKeys []string
	for k := range envMap {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	result["plugins"] = []any{
		map[string]any{
			dockerPlugin: makeRayDockerPlugin(jobEnvImage, envKeys),
		},
	}

	return result, nil
}

func (c *converter) convertPipelineGroup(g *pipelineGroup) (
	*bkPipelineGroup, error,
) {
	bkGroup := &bkPipelineGroup{
		Group: g.Group,
		Key:   g.Key,
	}

	for _, step := range g.Steps {
		bkStep, err := c.convertPipelineStep(step)
		if err != nil {
			return nil, fmt.Errorf("convert pipeline step: %w", err)
		}
		bkGroup.Steps = append(bkGroup.Steps, bkStep)
	}

	return bkGroup, nil
}
