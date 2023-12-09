package raycicmd

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

const windowsJobEnv = "WINDOWS"
const windowsJobQueue = "windows"
const windowsBuildEnvImage = "rayproject/buildenv:windows"

type converter struct {
	config  *config
	buildID string

	ciTempForBuild string

	envMap map[string]string
}

func newConverter(config *config, info *buildInfo) *converter {
	c := &converter{
		config:  config,
		buildID: info.BuildID,

		ciTempForBuild: config.CITemp + info.BuildID + "/",
	}

	envMap := make(map[string]string)
	envMap["RAYCI_BUILD_ID"] = info.BuildID
	envMap["RAYCI_WORK_REPO"] = config.CIWorkRepo
	envMap["RAYCI_TEMP"] = c.ciTempForBuild
	if info.RayCIBranch != "" {
		envMap["RAYCI_BRANCH"] = info.RayCIBranch
	}
	if config.ForgePrefix != "" {
		envMap["RAYCI_FORGE_PREFIX"] = config.ForgePrefix
	}

	if c.config.ArtifactsBucket != "" && info.GitCommit != "" {
		dest := fmt.Sprintf(
			"s3://%s/%s",
			c.config.ArtifactsBucket,
			info.GitCommit,
		)
		envMap["BUILDKITE_ARTIFACT_UPLOAD_DESTINATION"] = dest
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

func (c *converter) convertWait(step map[string]any) (map[string]any, error) {
	// a wait step
	if err := checkStepKeys(step, waitStepAllowedKeys); err != nil {
		return nil, fmt.Errorf("check wait step keys: %w", err)
	}
	return cloneMapExcept(step, waitStepDropKeys), nil
}

func (c *converter) convertWanda(step map[string]any) (map[string]any, error) {
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
	label, _ := stringInMap(step, "label")
	instanceType, _ := stringInMap(step, "instance_type")

	var matrix any
	if m, ok := step["matrix"]; ok {
		matrix = m
	}

	envs := c.envMapCopy()
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
		name:         name,
		label:        label,
		file:         file,
		buildID:      c.buildID,
		envs:         envs,
		ciConfig:     c.config,
		matrix:       matrix,
		instanceType: instanceType,
	}
	if dependsOn, ok := step["depends_on"]; ok {
		s.dependsOn = dependsOn
	}

	return s.buildkiteStep(), nil
}

func (c *converter) convertRunner(step map[string]any) (map[string]any, error) {
	if err := checkStepKeys(step, commandStepAllowedKeys); err != nil {
		return nil, fmt.Errorf("check command step keys: %w", err)
	}

	queue, _ := stringInMapAnyKey(step, "queue", "instance_type")
	if queue == "" {
		queue = "default"
	}
	agentQueue, err := c.mapAgent(queue)
	if err != nil {
		return nil, fmt.Errorf("map agent: %w", err)
	}

	result := cloneMapExcept(step, commandStepDropKeys)

	if agentQueue != skipQueue { // queue type not supported, skip.
		result["agents"] = newBkAgents(agentQueue)
	} else {
		result["skip"] = true
	}

	result["retry"] = defaultRayRetry
	result["timeout_in_minutes"] = defaultTimeoutInMinutes

	priority, ok := step["priority"]
	if !ok {
		priority = c.config.RunnerPriority
	}
	if priority != 0 {
		result["priority"] = priority
	}

	envMap := c.envMapCopy()
	result["env"] = envMap

	envKeys := make(map[string]struct{})
	for k := range envMap {
		envKeys[k] = struct{}{}
	}
	for _, k := range c.config.HookEnvKeys {
		envKeys[k] = struct{}{}
	}
	var envKeyList []string
	for k := range envKeys {
		envKeyList = append(envKeyList, k)
	}
	sort.Strings(envKeyList)

	jobEnv, _ := stringInMap(step, "job_env")
	dockerPluginConfig := &stepDockerPluginConfig{
		extraEnvs: envKeyList,
	}
	if d := c.config.DockerPlugin; d != nil && d.AllowMountBuildkiteAgent {
		v, _ := boolInMap(step, "mount_buildkite_agent")
		dockerPluginConfig.mountBuildkiteAgent = v
	}
	publishPortsStr, _ := stringInMap(step, "docker_publish_tcp_ports")
	if publishPortsStr != "" {
		publishPorts := strings.Split(publishPortsStr, ",")
		if len(publishPorts) > 0 {
			dockerPluginConfig.publishTCPPorts = publishPorts
		}
	}

	if queue == windowsJobQueue {
		jobEnvImage := windowsBuildEnvImage
		if jobEnv != windowsJobEnv {
			jobEnvImage = c.jobEnvImage(jobEnv)
		}
		result["plugins"] = []any{map[string]any{
			dockerPlugin: makeRayWindowsDockerPlugin(jobEnvImage, dockerPluginConfig),
		}}
	} else {
		// default Linux Job env.
		jobEnvImage := c.jobEnvImage(jobEnv)
		result["plugins"] = []any{map[string]any{
			dockerPlugin: makeRayDockerPlugin(jobEnvImage, dockerPluginConfig),
		}}
		result["artifact_paths"] = defaultArtifactPaths
	}

	return result, nil
}

func (c *converter) convertPipelineStep(step map[string]any) (
	map[string]any, error,
) {
	if _, ok := step["wait"]; ok {
		return c.convertWait(step)
	}
	// special steps for building container images.
	if _, ok := step["wanda"]; ok {
		return c.convertWanda(step)
	}
	return c.convertRunner(step)
}

func (c *converter) convertPipelineGroup(g *pipelineGroup, filter *tagFilter) (
	*bkPipelineGroup, error,
) {
	bkGroup := &bkPipelineGroup{
		Group:     g.Group,
		Key:       g.Key,
		DependsOn: g.DependsOn,
	}

	for _, step := range g.Steps {
		// filter steps by tags
		if stepTags, ok := step["tags"]; ok {
			if !filter.hit(toStringList(stepTags)) {
				continue
			}
		}

		// convert step to buildkite step
		bkStep, err := c.convertPipelineStep(step)
		if err != nil {
			return nil, fmt.Errorf("convert pipeline step: %w", err)
		}
		bkGroup.Steps = append(bkGroup.Steps, bkStep)
	}

	return bkGroup, nil
}
