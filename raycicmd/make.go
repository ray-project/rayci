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

var ciYamlSuffixes = []string{
	".rayci.yaml", ".rayci.yml",
	".ci.yaml", ".ci.yml",
}

func isRayCIYaml(p string) bool {
	for _, suffix := range ciYamlSuffixes {
		if strings.HasSuffix(p, suffix) {
			return true
		}
	}
	return false
}

func stripRayCIYamlSuffix(p string) string {
	for _, suffix := range ciYamlSuffixes {
		if strings.HasSuffix(p, suffix) {
			return strings.TrimSuffix(p, suffix)
		}
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
	sort.Slice(gs, func(i, j int) bool { return gs[i].lessThan(gs[j]) })
}

type pipelineContext struct {
	repoDir string
	config  *config
	info    *buildInfo
	envs    Envs
	changeLister ChangeLister
}

func makePipeline(ctx *pipelineContext) (
	*bkPipeline, error,
) {
	pl := new(bkPipeline)

	c := newConverter(ctx.config, ctx.info)

	filter, err := newStepFilter(
		ctx.config.SkipTags,
		ctx.info.selects,
		ctx.config.TagFilterCommand,
		ctx.config.TagRuleFiles,
		ctx.envs,
		ctx.changeLister,
	)
	if err != nil {
		return nil, fmt.Errorf("run tag filter command: %w", err)
	}

	filter.noTagMeansAlways = ctx.config.NoTagMeansAlways

	// Build steps for CI.
	bkDirs := ctx.config.BuildkiteDirs
	if len(bkDirs) == 0 {
		bkDirs = []string{".buildkite"}
	}

	var groups []*pipelineGroup
	for _, bkDir := range bkDirs {
		bkDir = filepath.Join(ctx.repoDir, bkDir) // extend to full path

		names, err := listCIYamlFiles(bkDir)
		if err != nil {
			return nil, fmt.Errorf("list pipeline files: %w", err)
		}

		for _, name := range names {
			file := filepath.Join(bkDir, name)
			g, err := parsePipelineFile(file)
			if err != nil {
				return nil, fmt.Errorf("parse pipeline file %s: %w", file, err)
			}

			groups = append(groups, g)
		}
	}
	sortPipelineGroups(groups)

	// map each file into a group.
	steps, err := c.convertGroups(groups, filter)
	if err != nil {
		return nil, fmt.Errorf("convert pipeline groups: %w", err)
	}
	pl.Steps = steps

	if pl.totalSteps() == 0 {
		q, ok := ctx.config.RunnerQueues["default"]
		if !ok {
			q = ""
		}
		return makeNoopBkPipeline(q), nil
	}

	if ctx.config.NotifyOwnerOnFailure {
		if email := ctx.info.buildAuthorEmail; email != "" {
			pl.Notify = append(pl.Notify, makeBuildFailureBkNotify(email))
		}
	}

	return pl, nil
}
