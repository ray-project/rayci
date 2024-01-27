package raycicmd

import (
	"fmt"
)

const windowsJobEnv = "WINDOWS"
const macosJobEnv = "MACOS"
const macosDenyFileRead = "/usr/local/etc/buildkite-agent/buildkite-agent.cfg"

type converter struct {
	config *config
	info   *buildInfo
	envMap map[string]string

	stepConverters []stepConverter

	defaultConverter stepConverter
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
	c.defaultConverter = newCommandConverter(config, info, envMap)

	return c
}

func (c *converter) convertStep(step map[string]any) (
	map[string]any, error,
) {
	for _, stepConverter := range c.stepConverters {
		if stepConverter.match(step) {
			return stepConverter.convert(step)
		}
	}
	return c.defaultConverter.convert(step)
}

func (c *converter) convertGroup(n *jobNode) (
	*bkPipelineGroup, error,
) {
	g := n.srcGroup

	bkGroup := &bkPipelineGroup{
		Group:     g.Group,
		Key:       g.Key,
		DependsOn: g.DependsOn,
	}

	for _, step := range n.steps {
		if !step.include {
			continue
		}

		// convert step to buildkite step
		bkStep, err := c.convertStep(step.srcStep)
		if err != nil {
			return nil, fmt.Errorf("convert pipeline step: %w", err)
		}
		bkGroup.Steps = append(bkGroup.Steps, bkStep)
	}

	return bkGroup, nil
}

func (c *converter) convertGroups(gs []*pipelineGroup, filter *tagFilter) (
	[]*bkPipelineGroup, error,
) {

	var groupNodes []*jobNode

	for i, g := range gs {
		node := &jobNode{
			id:       fmt.Sprintf("g%d", i),
			userKey:  g.Key,
			srcGroup: g,
			tags:     g.Tags,
		}

		for j, step := range g.Steps {
			k, ok := stringInMap(step, "name")
			if !ok {
				k, _ = stringInMap(step, "key")
			}

			var tags []string
			if v, ok := step["tags"]; ok {
				tags = toStringList(v)
			}

			node.steps = append(node.steps, &jobNode{
				id:      fmt.Sprintf("g%d_s%d", i, j),
				userKey: k,
				srcStep: step,
				tags:    tags,
			})
		}

		groupNodes = append(groupNodes, node)
	}

	// Build namedNodes, and check if we have duplicated user keys.
	nameNodes := make(map[string]*jobNode)
	for _, g := range groupNodes {
		if k := g.userKey; k != "" {
			if _, ok := nameNodes[k]; ok {
				return nil, fmt.Errorf("duplicate node key %q", k)
			}
			nameNodes[g.userKey] = g
		}

		for _, step := range g.steps {
			if k := step.userKey; k != "" {
				if _, ok := nameNodes[k]; ok {
					return nil, fmt.Errorf("duplicate node key %q", k)
				}
			}
		}
	}

	// Apply tags filter.
	for _, g := range groupNodes {
		if filter.hit(g.tags) {
			g.include = true
		}

		for _, step := range g.steps {
			if filter.hit(step.tags) {
				step.include = true
				g.include = true // include group if any step is included
			}
		}
	}

	// TODO(aslonnie): for any node that is not included, also include its
	// dependencies.

	var bkGroups []*bkPipelineGroup
	for _, g := range groupNodes {
		if !g.include {
			continue
		}

		bkGroup, err := c.convertGroup(g)
		if err != nil {
			return nil, err
		}
		if len(bkGroup.Steps) == 0 {
			continue // skip empty groups
		}
		bkGroups = append(bkGroups, bkGroup)
	}

	return bkGroups, nil
}
