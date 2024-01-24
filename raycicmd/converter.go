package raycicmd

import (
	"fmt"
	"sort"
	"strings"
)

const windowsJobEnv = "WINDOWS"
const macosJobEnv = "MACOS"
const macosDenyFileRead = "/usr/local/etc/buildkite-agent/buildkite-agent.cfg"

type converter struct {
	config *config
	info   *buildInfo
	envMap map[string]string

	stepConverters []stepConverter
}

func newConverter(config *config, info *buildInfo) *converter {
	c := &converter{
		config: config,
		info:   info,
	}

	envMap := make(map[string]string)
	envMap["RAYCI_BUILD_ID"] = info.buildID
	envMap["RAYCI_WORK_REPO"] = config.CIWorkRepo
	envMap["RAYCI_TEMP"] = config.CITemp + info.buildID + "/"
	if info.launcherBranch != "" {
		envMap["RAYCI_BRANCH"] = info.launcherBranch
	}
	if config.ForgePrefix != "" {
		envMap["RAYCI_FORGE_PREFIX"] = config.ForgePrefix
	}

	if c.config.ArtifactsBucket != "" && info.gitCommit != "" {
		dest := fmt.Sprintf(
			"s3://%s/%s",
			c.config.ArtifactsBucket,
			info.gitCommit,
		)
		envMap["BUILDKITE_ARTIFACT_UPLOAD_DESTINATION"] = dest
	}

	for k, v := range c.config.Env {
		envMap[k] = v
	}

	c.envMap = envMap

	c.stepConverters = []stepConverter{
		waitConverter,
		blockConverter,
		newWandaConverter(config, info, envMap),
	}

	return c
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

	return fmt.Sprintf("%s:%s-%s", c.config.CIWorkRepo, c.info.buildID, name)
}

const dockerPlugin = "docker#v5.8.0"
const macosSandboxPlugin = "ray-project/macos-sandbox#v1.0.7"

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

	envMap := copyEnvMap(c.envMap)
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
	dockerNetwork, _ := stringInMap(step, "docker_network")
	if dockerNetwork != "" {
		dockerPluginConfig.network = dockerNetwork
	}
	v, _ := boolInMap(step, "mount_windows_artifacts")
	dockerPluginConfig.mountWindowsArtifacts = v

	if jobEnv == windowsJobEnv { // a special job env for windows
		result["plugins"] = []any{map[string]any{
			dockerPlugin: makeRayWindowsDockerPlugin(dockerPluginConfig),
		}}
		if dockerPluginConfig.mountWindowsArtifacts {
			result["artifact_paths"] = windowsArtifactPaths
		}
	} else if jobEnv == macosJobEnv { // a special job env for macos
		result["plugins"] = []any{map[string]any{
			macosSandboxPlugin: map[string]string{
				"deny-file-read": macosDenyFileRead,
			},
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
	for _, stepConverter := range c.stepConverters {
		if stepConverter.match(step) {
			return stepConverter.convert(step)
		}
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
