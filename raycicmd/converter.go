package raycicmd

import (
	"fmt"
)

type converter struct {
	config  *config
	buildID string

	ciTempForBuild string
}

func newConverter(config *config, buildID string) *converter {
	return &converter{
		config:  config,
		buildID: buildID,

		ciTempForBuild: config.CITemp + buildID + "/",
	}
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

	return fmt.Sprintf("%s:%s-%s", c.config.CITempRepo, c.buildID, name)
}

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

	// a normal command step
	if err := checkStepKeys(step, commandStepAllowedKeys); err != nil {
		return nil, fmt.Errorf("check command step keys: %w", err)
	}

	queue, _ := stringInMapAnyKey(step, "queue", "instance_type")
	agentQueue, err := c.mapAgent(queue)
	if err != nil {
		return nil, fmt.Errorf("map agent: %w", err)
	}

	jobEnv, _ := stringInMap(step, "job_env")
	jobEnvImage := c.jobEnvImage(jobEnv)

	result := cloneMapExcept(step, commandStepDropKeys)

	result["agents"] = newBkAgents(agentQueue)
	result["retry"] = defaultRayRetry
	result["timeout_in_minutes"] = defaultTimeoutInMinutes
	result["artifact_paths"] = defaultArtifactPaths

	if !c.config.Dockerless {
		envs := []string{
			"RAYCI_BUILD_ID=" + c.buildID,
			"RAYCI_TEMP=" + c.ciTempForBuild,
		}
		result["plugins"] = []any{
			map[string]any{
				"docker#v5.8.0": makeRayDockerPlugin(jobEnvImage, envs),
			},
		}
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
