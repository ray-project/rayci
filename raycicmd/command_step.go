package raycicmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type commandConverter struct {
	config *config
	info   *buildInfo

	envMap map[string]string
}

func newCommandConverter(
	config *config, info *buildInfo, envMap map[string]string,
) *commandConverter {
	return &commandConverter{
		config: config,
		info:   info,
		envMap: envMap,
	}
}

func (c *commandConverter) mapAgent(instanceType string) (string, error) {
	if instanceType == "" {
		instanceType = "default"
	}
	if q, ok := c.config.RunnerQueues[instanceType]; ok {
		return q, nil
	}
	return "", fmt.Errorf("unknown instance type %q", instanceType)
}

func (c *commandConverter) jobEnvImage(name string) string {
	if name == "" {
		name = "forge"
	}

	return fmt.Sprintf("%s:%s-%s", c.config.CIWorkRepo, c.info.buildID, name)
}

const (
	dockerPlugin = "docker#v5.8.0"

	macosSandboxPlugin = "ray-project/macos-sandbox#v1.0.7"
	macosJobEnv        = "MACOS"
	macosDenyFileRead  = "/usr/local/etc/buildkite-agent/buildkite-agent.cfg"

	windowsJobEnv = "WINDOWS"
)

func (c *commandConverter) match(step map[string]any) bool {
	// This converter is used as a default converter.
	// All steps that are not matching other steps will be treated as a
	// command step. Therefore, it matches everything.
	return true
}

func (c *commandConverter) convert(id string, step map[string]any) (
	map[string]any, error,
) {
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
	// We treat nil and empty allowConcurrencyGroupPrefixes differently. A nil value
	// means that we don't have any restrictions on the concurrency group. An empty
	// value means that we don't allow any concurrency group.
	if group, ok := stringInMap(step, "concurrency_group"); ok {
		if prefixes := c.config.AllowConcurrencyGroupPrefixes; prefixes != nil {
			if !stringHasPrefix(group, prefixes) {
				return nil, fmt.Errorf("concurrency group %q is not allowed", group)
			}
		}
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

	parallelism, ok := step["parallelism"]
	if ok && c.config.MaxParallelism > 0 {
		maxParallelism := c.config.MaxParallelism
		parallelism, err := strconv.Atoi(fmt.Sprintf("%v", parallelism))
		if err != nil {
			return nil, fmt.Errorf("convert parallelism: %w", err)
		}
		if parallelism > maxParallelism {
			result["parallelism"] = maxParallelism
		}
	}

	envMap := copyEnvMap(c.envMap)
	if id != "" {
		envMap["RAYCI_STEP_ID"] = id
	}
	result["env"] = envMap

	envKeys := make(map[string]struct{})
	for k := range envMap {
		envKeys[k] = struct{}{}
	}
	for _, k := range c.config.HookEnvKeys {
		envKeys[k] = struct{}{}
	}
	for _, k := range c.config.BuildEnvKeys {
		envKeys[k] = struct{}{}
	}
	var envKeyList []string
	for k := range envKeys {
		envKeyList = append(envKeyList, k)
	}
	sort.Strings(envKeyList)

	jobEnv, _ := stringInMap(step, "job_env")
	dockerPluginConfig := &stepDockerPluginConfig{extraEnvs: envKeyList}
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
	switch jobEnv {
	case windowsJobEnv: // a special job env for windows
		result["plugins"] = []any{map[string]any{
			dockerPlugin: makeRayWindowsDockerPlugin(dockerPluginConfig),
		}}
		result["artifact_paths"] = windowsArtifactPaths
	case macosJobEnv: // a special job env for macos
		result["plugins"] = []any{map[string]any{
			macosSandboxPlugin: map[string]string{
				"deny-file-read": macosDenyFileRead,
			},
		}}
	default:
		// default Linux Job env.
		jobEnvImage := c.jobEnvImage(jobEnv)
		result["plugins"] = []any{map[string]any{
			dockerPlugin: makeRayDockerPlugin(jobEnvImage, dockerPluginConfig),
		}}
		result["artifact_paths"] = defaultArtifactPaths
	}

	return result, nil
}
