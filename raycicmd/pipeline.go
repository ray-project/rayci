package raycicmd

import (
	"bytes"
	"fmt"
	"log"
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

var noopPipeline = &bkPipeline{
	Steps: []*bkPipelineGroup{{
		Group: "noop",
		Steps: []any{map[string]any{
			"label":   "noop",
			"command": "echo 'no steps found in repo'",
		}},
	}},
}

type converter struct {
	config *config
}

func newConverter(config *config) *converter {
	return &converter{config: config}
}

func makePipeline(repoDir string, config *config) (*bkPipeline, error) {
	pipelineDir := filepath.Join(repoDir, ".buildkite")

	entries, err := os.ReadDir(pipelineDir)
	if err != nil {
		if os.IsNotExist(err) {
			return noopPipeline, nil
		}
		return nil, fmt.Errorf("read pipeline dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".rayci.yaml") ||
			strings.HasSuffix(name, ".rayci.yml") {
			files = append(files, filepath.Join(pipelineDir, name))
		}
	}

	if len(files) == 0 {
		return noopPipeline, nil
	}

	pl := new(bkPipeline)
	c := newConverter(config)
	for _, file := range files {
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
	if q, ok := c.config.AgentQueueMap[instanceType]; ok {
		return q, nil
	}
	return "", fmt.Errorf("unknown instance type %q", instanceType)
}

func checkStepKeys(m map[string]any, allowed []string) error {
	allowedMap := make(map[string]bool, len(allowed))
	for _, k := range allowed {
		allowedMap[k] = true
	}

	for k := range m {
		if !allowedMap[k] {
			return fmt.Errorf("unsupported step key %q", k)
		}
	}
	return nil
}

var (
	waitStepAllowedKeys    = []string{"wait", "continue_on_failure"}
	commandStepAllowedKeys = []string{
		"command", "commands",
		"label", "name", "key",
		"depends_on", "instance_type", "queue", "soft_fail",
		"matrix",
		"job_env",
	}
)

func stringInMap(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	res := make(map[string]any)
	for k, v := range m {
		res[k] = v
	}
	return res
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

	jobEnv := "forge" // default job env

	if v, ok := stringInMap(result, "job_env"); ok {
		delete(result, "job_env")
		jobEnv = v
	}

	switch jobEnv {
	case "ubuntu-focal": // builtin support
		jobEnv = "ubuntu:20.04"
	default:
		log.Printf("fake job env %q with ubuntu", jobEnv)
		jobEnv = "ubuntu:20.04" // TODO(aslonnie): map to ECR
	}

	result["agents"] = newBkAgents(q)
	result["retry"] = defaultRayRetry
	result["timeout_in_minutes"] = defaultTimeoutInMinutes
	result["artifact_paths"] = defaultArtifactPaths

	if !c.config.Dockerless {
		result["plugins"] = []any{
			makeRayDockerPlugin(jobEnv),
		}
	}

	return result, nil
}
