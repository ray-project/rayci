package raycicmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"
)

type pipelineGroup struct {
	Group string `yaml:"group"`
	Key   string `yaml:"key"`

	Steps []map[string]any `yaml:"steps"`
}

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

func isRayCIYaml(p string) bool {
	if strings.HasSuffix(p, ".rayci.yaml") {
		return true
	}
	if strings.HasSuffix(p, ".rayci.yml") {
		return true
	}
	return false
}

func makePipeline(repoDir string, config *config, buildID string) (
	*bkPipeline, error,
) {
	pl := new(bkPipeline)

	// Build steps that build the forge images.
	forgeGroup := &bkPipelineGroup{
		Group: "forge",
		Key:   "all-forges",
	}

	// add forge container building steps
	for _, dir := range config.ForgeDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read forge dir %s: %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			filePath := filepath.Join(dir, name)
			forgeName, ok := forgeNameFromDockerfile(name)
			if !ok {
				continue
			}

			agent := ""
			if config.BuilderQueues != nil {
				if q, ok := config.BuilderQueues["builder"]; ok {
					agent = q
				}
			}

			bkStep := map[string]any{
				"label":    forgeName,
				"key":      forgeName,
				"commands": []string{forgeBuilderCommand},
				"env": map[string]string{
					"RAYCI_BUILD_ID":         buildID,
					"RAYCI_TMP_REPO":         config.CITempRepo,
					"RAYCI_FORGE_DOCKERFILE": filePath,
					"RAYCI_FORGE_NAME":       forgeName,
				},
			}
			if agent != "" {
				bkStep["agents"] = newBkAgents(agent)
			}

			forgeGroup.Steps = append(forgeGroup.Steps, bkStep)
		}
	}

	if len(forgeGroup.Steps) > 0 {
		pl.Steps = append(pl.Steps, forgeGroup)
	}

	// Build steps for CI.

	bkDir := config.BuildkiteDir
	if bkDir == "" {
		bkDir = ".buildkite"
	}
	bkDir = filepath.Join(repoDir, bkDir)

	entries, err := os.ReadDir(bkDir)
	if err != nil {
		if os.IsNotExist(err) {
			entries = nil
		} else {
			return nil, fmt.Errorf("read pipeline dir: %w", err)
		}
	}

	c := newConverter(config, buildID)

	// add rayci buildkite pipelines
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isRayCIYaml(name) {
			continue
		}
		file := filepath.Join(bkDir, name)

		g, err := parsePipelineFile(file)
		if err != nil {
			return nil, fmt.Errorf("parse pipeline file %s: %w", file, err)
		}

		bkGroup, err := c.convertPipelineGroup(g)
		if err != nil {
			return nil, fmt.Errorf("convert pipeline group %s: %w", file, err)
		}
		pl.Steps = append(pl.Steps, bkGroup)
	}

	return pl, nil
}

func parsePipelineFile(file string) (*pipelineGroup, error) {
	bs, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read pipeline file: %w", err)
	}

	g := new(pipelineGroup)
	dec := yaml.NewDecoder(bytes.NewReader(bs))
	dec.KnownFields(true)
	if err := dec.Decode(g); err != nil {
		return nil, fmt.Errorf("unmarshal pipeline file: %w", err)
	}

	return g, nil
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

var (
	defaultTimeoutInMinutes = int((5 * time.Hour).Minutes())
	defaultArtifactPaths    = []string{"tmp/artifacts/**/*"}
)

func (c *converter) mapAgent(instanceType string) (string, error) {
	if instanceType == "" {
		instanceType = "default"
	}
	if q, ok := c.config.RunnerQueues[instanceType]; ok {
		return q, nil
	}
	return "", fmt.Errorf("unknown instance type %q", instanceType)
}

var (
	waitStepAllowedKeys    = []string{"wait", "continue_on_failure"}
	commandStepAllowedKeys = []string{
		"command", "commands",
		"label", "name", "key", "depends_on", "soft_fail", "matrix",
		"instance_type", "queue", "job_env",
	}
)

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

	queue, ok := stringInMap(step, "queue")
	if !ok {
		instanceType, ok := stringInMap(step, "instance_type")
		if ok {
			queue = instanceType
		}
	}

	q, err := c.mapAgent(queue)
	if err != nil {
		return nil, fmt.Errorf("map agent: %w", err)
	}

	result := cloneMap(step)
	for _, k := range []string{"instance_type", "queue"} {
		delete(result, k)
	}

	jobEnv := "forge" // default job env
	if v, ok := stringInMap(result, "job_env"); ok {
		delete(result, "job_env")
		jobEnv = v
	}
	jobEnv = fmt.Sprintf("%s:%s-%s", c.config.CITempRepo, c.buildID, jobEnv)

	result["agents"] = newBkAgents(q)
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
				"docker#v5.8.0": makeRayDockerPlugin(jobEnv, envs),
			},
		}
	}

	return result, nil
}
