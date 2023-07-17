package raycicmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

func isRayCIYaml(p string) bool {
	if strings.HasSuffix(p, ".rayci.yaml") {
		return true
	}
	if strings.HasSuffix(p, ".rayci.yml") {
		return true
	}
	return false
}

func makeForgeGroup(buildID string, config *config) (*bkPipelineGroup, error) {
	g := &bkPipelineGroup{
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
			forgeName, ok := forgeNameFromDockerfile(name)
			if !ok {
				continue
			}

			filePath := filepath.Join(dir, name)
			step := makeForgeStep(buildID, forgeName, filePath, config)
			g.Steps = append(g.Steps, step)
		}
	}

	return g, nil
}

func makePipeline(repoDir string, config *config, buildID string) (
	*bkPipeline, error,
) {
	pl := new(bkPipeline)

	// Build steps that build the forge images.

	forgeGroup, err := makeForgeGroup(buildID, config)
	if err != nil {
		return nil, fmt.Errorf("make forge group: %w", err)
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
		if len(bkGroup.Steps) == 0 {
			continue // skip empty groups
		}

		pl.Steps = append(pl.Steps, bkGroup)
	}

	totalSteps := 0
	for _, g := range pl.Steps {
		totalSteps += len(g.Steps)
	}
	if totalSteps == 0 {
		q, ok := config.RunnerQueues["default"]
		if !ok {
			return nil, fmt.Errorf("no default queue found in config")
		}
		return makeNoopBkPipeline(q), nil
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
