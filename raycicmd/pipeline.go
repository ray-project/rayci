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

const (
	stepTypeCommand = "command" // Default and most common step type.
	stepTypeWait    = "wait"
)

type pipelineGroup struct {
	Group string          `yaml:"group"`
	Key   string          `yaml:"key"`
	Steps []*pipelineStep `yaml:"steps"`
}

type pipelineStep struct {
	// Marks the step's type, default is a command step.
	Type string `yaml:"type"`

	Label     string   `yaml:"label"`
	Key       string   `yaml:"key"`
	Commands  []string `yaml:"commands"`
	DependsOn []string `yaml:"depends_on"`
	If        string   `yaml:"if"`
	SoftFail  bool     `yaml:"soft_fail"`

	InstanceType string `yaml:"instance_type"`
	Queue        string `yaml:"queue"`

	JobEnv string `yaml:"job_env"` // Container to run in.

	// For wait step only
	// wait step also has an `if` and `depends_on` field.
	ContinueOnFailure bool `yaml:"continue_on_failure"`
}

var noopPipeline = &bkPipeline{
	Steps: []*bkPipelineGroup{{
		Group: "noop",
		Steps: []any{&bkCommandStep{
			Label:    "noop",
			Commands: []string{"echo 'no steps found in repo'"},
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
		if strings.HasSuffix(name, ".rayci.yml") {
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

func copyStrings(ss []string) []string {
	if ss == nil {
		return nil
	}
	res := make([]string, len(ss))
	copy(res, ss)
	return res
}

var defaultTimeoutInMinutes = int((5 * time.Hour).Minutes())

var defaultArtifactsPaths = []string{"tmp/artifacts/**/*"}

func (c *converter) mapAgent(instanceType string) (string, error) {
	if instanceType == "" {
		instanceType = "default"
	}
	if q, ok := c.config.AgentQueueMap[instanceType]; ok {
		return q, nil
	}
	return "", fmt.Errorf("unknown instance type %q", instanceType)
}

func (c *converter) convertPipelineStep(step *pipelineStep) (any, error) {
	switch step.Type {
	default:
		return nil, fmt.Errorf("unknown step type %q", step.Type)
	case stepTypeWait:
		return &bkWaitStep{
			If:                step.If,
			ContinueOnFailure: step.ContinueOnFailure,
		}, nil
	case "", stepTypeCommand:
		queue := step.Queue
		if queue == "" && step.InstanceType != "" {
			queue = step.InstanceType
		}

		q, err := c.mapAgent(queue)
		if err != nil {
			return nil, fmt.Errorf("map agent: %w", err)
		}

		jobEnv := "forge"
		if step.JobEnv != "" {
			jobEnv = step.JobEnv
		}

		cmd := &bkCommandStep{
			Key:       step.Key,
			Label:     step.Label,
			Commands:  copyStrings(step.Commands),
			DependsOn: copyStrings(step.DependsOn),
			SoftFail:  step.SoftFail,

			AritfactPaths:    defaultArtifactsPaths,
			TimeoutInMinutes: defaultTimeoutInMinutes,

			Agents: newBkAgents(q),

			Retry: defaultRayRetry,
		}

		if !c.config.Dockerless {
			cmd.Plugins = append(cmd.Plugins, makeRayDockerPlugin(jobEnv))
		}

		return cmd, nil
	}
}
