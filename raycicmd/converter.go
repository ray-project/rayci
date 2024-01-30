package raycicmd

import (
	"fmt"
)

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

func keyOfStep(step map[string]any) string {
	if k, ok := stringInMap(step, "name"); ok {
		return k
	}
	k, _ := stringInMap(step, "key")
	return k
}

func tagsOfStep(step map[string]any) []string {
	if v, ok := step["tags"]; ok {
		return toStringList(v)
	}
	return nil
}

func (c *converter) convertGroups(gs []*pipelineGroup, filter *tagFilter) (
	[]*bkPipelineGroup, error,
) {
	graph := newNodeGraph()

	var groupNodes []*jobNode
	for i, g := range gs {
		node := &jobNode{
			id:       fmt.Sprintf("g%d", i),
			userKey:  g.Key,
			srcGroup: g,
			tags:     g.Tags,
		}

		for j, step := range g.Steps {
			stepNode := &jobNode{
				id:      fmt.Sprintf("g%d_s%d", i, j),
				srcStep: step,
				userKey: keyOfStep(step),
				tags:    tagsOfStep(step),
			}
			node.steps = append(node.steps, stepNode)
			graph.add(stepNode)
		}

		groupNodes = append(groupNodes, node)
		graph.add(node)
	}

	// Build namedNodes, and check if we have duplicated user keys.
	for _, g := range groupNodes {
		if k := g.userKey; k != "" {
			if err := graph.addName(g.id, k); err != nil {
				return nil, fmt.Errorf("add group %q: %w", k, err)
			}
		}
		for _, step := range g.steps {
			if k := step.userKey; k != "" {
				if err := graph.addName(step.id, k); err != nil {
					return nil, fmt.Errorf("add node %q: %w", k, err)
				}
			}
		}
	}

	// Populate dependsOn.
	for _, g := range groupNodes {
		// A group node is different from a step node.
		//
		// When a group is being depended on, all its steps are being depended
		// on. The gating is equivalent to wait step at the end of the group.
		//
		// When a group depensd on a node, it means all its steps depend on the
		// node. The gating is equivalent to wait step at the beginning of the
		// group.
		commonDeps := make(map[string]struct{})
		for _, dep := range g.srcGroup.DependsOn {
			if depNode, ok := graph.byName(dep); ok {
				commonDeps[depNode.id] = struct{}{}
			}
		}

		var lastGate *jobNode
		for _, step := range g.steps {
			// Track step dependencies.
			if dependsOn, ok := step.srcStep["depends_on"]; !ok {
				deps := toStringList(dependsOn)
				for _, dep := range deps {
					if depNode, ok := graph.byName(dep); ok {
						step.addDependsOn(depNode.id)
					}
				}
			} else if lastGate != nil {
				step.addDependsOn(lastGate.id)
			}

			// Add all group common deps.
			for dep := range commonDeps {
				step.addDependsOn(dep)
			}

			if isBlockOrWait(step.srcStep) {
				lastGate = step
			}
		}
	}

	// Apply tags filter.
	hits := make(map[string]struct{})
	for _, g := range groupNodes {
		hitGroup := false
		if filter.hit(g.tags) {
			hitGroup = true
		}

		for _, step := range g.steps {
			if filter.hit(step.tags) {
				hits[step.id] = struct{}{}
				hitGroup = true
			}
		}

		if hitGroup {
			hits[g.id] = struct{}{}
		}
	}
	graph.markDeps(hits)

	// Finalize the conversion.
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
