package raycicmd

import (
	"encoding/base64"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type commandConverter struct {
	config *config
	info   *buildInfo

	envMap map[string]string

	defaultJobEnv string
}

func newCommandConverter(
	config *config, info *buildInfo, envMap map[string]string,
) *commandConverter {
	return &commandConverter{
		config: config,
		info:   info,
		envMap: envMap,

		defaultJobEnv: "forge",
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

func (c *commandConverter) setDefaultJobEnv(name string) {
	if name == "" {
		name = "forge"
	}
	c.defaultJobEnv = name
}

func (c *commandConverter) jobEnvImage(name string) string {
	if name == "" {
		name = c.defaultJobEnv
	}

	// If the name contains a "/", treat it as a full image reference
	// (e.g., "rayproject/manylinux2014:1.0.0-jdk-x86_64").
	// Otherwise, treat it as a wanda image name and construct the full path.
	if strings.Contains(name, "/") {
		return name
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
	// We treat nil and empty allowConcurrencyGroupPrefixes differently.
	// A nil value means that we don't have any restrictions on the
	// concurrency cg. An empty value means that we don't allow any
	// concurrency cg.
	if cg, ok := stringInMap(step, "concurrency_group"); ok {
		if allow := c.config.ConcurrencyGroupPrefixes; allow != nil {
			if !stringHasPrefix(cg, allow) {
				return nil, fmt.Errorf(
					"concurrency group %q is not allowed", cg,
				)
			}
		}
	}

	timeoutInMinutes := defaultTimeoutInMinutes
	if v, ok := intInMap(step, "timeout_in_minutes"); ok {
		if v < 0 {
			return nil, fmt.Errorf("timeout_in_minutes cannot be negative")
		}
		if v > defaultTimeoutInMinutes {
			v = defaultTimeoutInMinutes
		}
		timeoutInMinutes = v
	}

	result := cloneMapExcept(step, commandStepDropKeys)
	if len(result) == 0 {
		return nil, fmt.Errorf(
			"step has only rayci-processed keys (missing command?): %v", step,
		)
	}

	if agentQueue != skipQueue { // queue type not supported, skip.
		result["agents"] = newBkAgents(agentQueue)
	} else {
		result["skip"] = true
	}

	result["retry"] = defaultRayRetry
	result["timeout_in_minutes"] = timeoutInMinutes

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
	if v, _ := boolInMap(step, "fetch_full_history"); v {
		envMap["RAYCI_FETCH_FULL_HISTORY"] = "1"
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

	jobEnv, _ := stringInMap(step, "job_env")

	awsRoles, err := stepAWSAssumeRoles(step)
	if err != nil {
		return nil, err
	}
	var awsEnvValues []string
	if len(awsRoles) > 0 {
		// The generated config uses a Linux path, and IMDS reachability
		// from these job envs is unverified; fail at generation time
		// rather than silently at runtime.
		if jobEnv == windowsJobEnv || jobEnv == macosJobEnv {
			return nil, fmt.Errorf(
				"aws_assume_role is not supported on job_env %q", jobEnv,
			)
		}
		// Chain env goes into the docker plugin environment, not step
		// env: step env applies to host-side job phases (hooks, artifact
		// upload), where the config file does not exist and pointing
		// AWS_CONFIG_FILE at it would change the host's credentials.
		// The container performs the chain's STS AssumeRole calls itself,
		// so it needs the host's region settings, and AWS_SDK_LOAD_CONFIG
		// makes aws-sdk-go v1 tools read the config file at all.
		envKeys["AWS_REGION"] = struct{}{}
		envKeys["AWS_DEFAULT_REGION"] = struct{}{}
		awsEnvValues = []string{
			"RAYCI_AWS_CONFIG_B64=" + base64.StdEncoding.EncodeToString(
				[]byte(awsChainConfig(
					awsRoles, awsSessionName(c.info.buildID),
				)),
			),
			"AWS_CONFIG_FILE=" + awsConfigFilePath,
			"AWS_SDK_LOAD_CONFIG=1",
		}
		if err := prependCommand(result, awsChainSetupCommand); err != nil {
			return nil, fmt.Errorf("set up aws_assume_role chain: %w", err)
		}
	}

	var envKeyList []string
	for k := range envKeys {
		envKeyList = append(envKeyList, k)
	}
	sort.Strings(envKeyList)
	envKeyList = append(envKeyList, awsEnvValues...)

	dockerPluginConfig := &stepDockerPluginConfig{extraEnvs: envKeyList}
	if d := c.config.DockerPlugin; d != nil {
		if d.AllowMountBuildkiteAgent {
			v, _ := boolInMap(step, "mount_buildkite_agent")
			dockerPluginConfig.mountBuildkiteAgent = v
		}
		if d.AllowMountSSHAgent {
			v, _ := boolInMap(step, "mount_ssh_agent")
			dockerPluginConfig.mountSSHAgent = v
		}
		if d.WorkDir != "" {
			dockerPluginConfig.workDir = d.WorkDir
		}
		dockerPluginConfig.addCaps = d.AddCaps
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
	// Non-default cases must stay in sync with the aws_assume_role
	// job_env guard above: a job env added here without extending the
	// guard would silently receive chain env it cannot use.
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

	// add step ID into label
	if id != "" {
		// Buildkite supports both "name" and "label".
		// Although "label" is the official key, "name" actually takes
		// precedence...  So to be consistency with buildkite, we do the same
		// here.

		label := result["name"]
		if label == nil {
			label = result["label"]
		}

		delete(result, "name")
		// "label" will be overwritten by the following code.

		if label == nil {
			label = fmt.Sprintf("[%s]", id)
		} else {
			label = fmt.Sprintf("%s [%s]", label, id)
		}
		result["label"] = label
	}

	return result, nil
}

// prependCommand inserts line before a step's existing command(s). Both
// "command" and "commands" are patched when a step carries both — Buildkite
// honors either key, and the setup line must precede whichever one runs.
// The docker plugin runs all command lines in a single shell invocation, so
// files written by line are visible to the rest.
func prependCommand(step map[string]any, line string) error {
	prepended := false
	for _, key := range []string{"commands", "command"} {
		v, ok := step[key]
		if !ok {
			continue
		}
		if v == nil {
			return fmt.Errorf("%s is empty", key)
		}
		cmds, err := scalarStrings(v)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		// A present-but-blank key would upload a job that runs only the
		// prepended setup line and exits green.
		if !slices.ContainsFunc(cmds, func(c string) bool {
			return strings.TrimSpace(c) != ""
		}) {
			return fmt.Errorf("%s is empty", key)
		}
		step[key] = append([]string{line}, cmds...)
		prepended = true
	}
	if !prepended {
		return fmt.Errorf("step has no command")
	}
	return nil
}
