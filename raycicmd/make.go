package raycicmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func stripRayCIYamlSuffix(p string) string {
	if strings.HasSuffix(p, ".rayci.yaml") {
		return strings.TrimSuffix(p, ".rayci.yaml")
	}
	if strings.HasSuffix(p, ".rayci.yml") {
		return strings.TrimSuffix(p, ".rayci.yml")
	}
	return p
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

	g.filename = file
	if g.SortKey != "" {
		g.sortKey = g.SortKey
	} else {
		g.sortKey = stripRayCIYamlSuffix(filepath.Base(file))
	}

	return g, nil
}

func sortPipelineGroups(gs []*pipelineGroup) {
	sort.Slice(gs, func(i, j int) bool { return gs[i].less(gs[j]) })
}

func makePipeline(repoDir string, config *config, info *buildInfo) (
	*bkPipeline, error,
) {
	pl := new(bkPipeline)

	c := newConverter(config, info)

	// Build steps for CI.

	bkDirs := config.BuildkiteDirs
	if len(bkDirs) == 0 {
		bkDirs = []string{".buildkite"}
	}

	tagFilters, err := newTagFilter(config.SkipTags, config.TagFilterCommand)
	if err != nil {
		return nil, fmt.Errorf("run tag filter command: %w", err)
	}
	for _, bkDir := range bkDirs {
		bkDir = filepath.Join(repoDir, bkDir) // extend to full path

		names, err := listCIYamlFiles(bkDir)
		if err != nil {
			return nil, fmt.Errorf("list pipeline files: %w", err)
		}

		var groups []*pipelineGroup
		for _, name := range names {
			file := filepath.Join(bkDir, name)
			g, err := parsePipelineFile(file)
			if err != nil {
				return nil, fmt.Errorf("parse pipeline file %s: %w", file, err)
			}

			if !tagFilters.hit(g.Tags) {
				continue
			}

			groups = append(groups, g)
		}

		sortPipelineGroups(groups)

		// map each file into a group.
		steps, err := c.convertGroups(groups, tagFilters)
		if err != nil {
			return nil, fmt.Errorf("convert pipeline groups: %w", err)
		}
		pl.Steps = steps
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
