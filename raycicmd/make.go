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

func listCIYamlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			entries = nil
		} else {
			return nil, fmt.Errorf("read pipeline dir: %w", err)
		}
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isRayCIYaml(name) {
			continue
		}
		names = append(names, name)
	}

	return names, nil
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

func makePipeline(repoDir string, config *config, info *buildInfo) (
	*bkPipeline, error,
) {
	pl := new(bkPipeline)

	c := newConverter(config, info)

	// Build steps that build the forge images.

	forgeGroup, err := makeForgeGroup(repoDir, info, config, c.envMapCopy())
	if err != nil {
		return nil, fmt.Errorf("make forge group: %w", err)
	}
	if len(forgeGroup.Steps) > 0 {
		pl.Steps = append(pl.Steps, forgeGroup)
	}

	// Build steps for CI.

	bkDirs := config.BuildkiteDirs
	if len(bkDirs) == 0 {
		bkDirs = []string{".buildkite"}
	}

	for _, bkDir := range bkDirs {
		bkDir = filepath.Join(repoDir, bkDir) // extend to full path

		names, err := listCIYamlFiles(bkDir)
		if err != nil {
			return nil, fmt.Errorf("list pipeline files: %w", err)
		}

		// map each file into a group.
		for _, name := range names {
			file := filepath.Join(bkDir, name)

			g, err := parsePipelineFile(file)
			if err != nil {
				return nil, fmt.Errorf("parse pipeline file %s: %w", file, err)
			}

			bkGroup, err := c.convertPipelineGroup(g)
			if err != nil {
				return nil, fmt.Errorf(
					"convert pipeline group %s: %w", file, err,
				)
			}
			if len(bkGroup.Steps) == 0 {
				continue // skip empty groups
			}

			pl.Steps = append(pl.Steps, bkGroup)
		}
	}

	totalSteps := 0
	for _, group := range pl.Steps {
		totalSteps += len(group.Steps)
	}
	if totalSteps == 0 {
		q, ok := config.RunnerQueues["default"]
		if !ok {
			q = ""
		}
		return makeNoopBkPipeline(q), nil
	}

	return pl, nil
}
