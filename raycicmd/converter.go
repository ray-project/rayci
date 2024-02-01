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

func (c *converter) convertGroup(n *stepNode) (
	*bkPipelineGroup, error,
) {
	g := n.srcGroup

	bkGroup := &bkPipelineGroup{
		Group:     g.Group,
		Key:       g.Key,
		DependsOn: g.DependsOn,
	}

	for _, step := range n.subSteps {
		if !step.hit() {
			continue
		}

		// convert step to buildkite step
		bkStep, err := c.convertStep(step.src)
		if err != nil {
			return nil, fmt.Errorf("convert pipeline step: %w", err)
		}
		bkGroup.Steps = append(bkGroup.Steps, bkStep)
	}

	return bkGroup, nil
}

func stepKey(step map[string]any) string {
	if k, ok := stringInMap(step, "name"); ok {
		return k
	}
	k, _ := stringInMap(step, "key")
	return k
}

func stepTags(step map[string]any) []string {
	if v, ok := step["tags"]; ok {
		return toStringList(v)
	}
	return nil
}

func (c *converter) convertGroups(gs []*pipelineGroup, filter *stepFilter) (
	[]*bkPipelineGroup, error,
) {
	nodes := newStepNodeSet()

	var groupNodes []*stepNode
	for i, g := range gs {
		node := &stepNode{
			id:       fmt.Sprintf("g%d", i),
			key:      g.Key,
			srcGroup: g,
			tags:     g.Tags,
		}

		for j, step := range g.Steps {
			stepNode := &stepNode{
				id:   fmt.Sprintf("g%d_s%d", i, j),
				src:  step,
				key:  stepKey(step),
				tags: stepTags(step),
			}
			node.subSteps = append(node.subSteps, stepNode)
			nodes.add(stepNode)
		}

		groupNodes = append(groupNodes, node)
		nodes.add(node)
	}

	// check if we have duplicated user keys.
	for _, groupNode := range groupNodes {
		if k := groupNode.key; k != "" {
			if err := nodes.addName(groupNode.id, k); err != nil {
				return nil, fmt.Errorf("add group %q: %w", k, err)
			}
		}
		for _, step := range groupNode.subSteps {
			if k := step.key; k != "" {
				if err := nodes.addName(step.id, k); err != nil {
					return nil, fmt.Errorf("add node %q: %w", k, err)
				}
			}
		}
	}

	// Populate dependsOn.
	for _, groupNode := range groupNodes {
		// A group node is different from a step node.
		//
		// When a group is being depended on, all its steps are being depended
		// on. The gating is equivalent to wait step at the end of the group.
		//
		// When a group depensd on a node, it means all its steps depend on the
		// node. The gating is equivalent to wait step at the beginning of the
		// group.
		commonDeps := make(map[string]struct{})
		for _, dep := range groupNode.srcGroup.DependsOn {
			if depNode, ok := nodes.byName(dep); ok {
				commonDeps[depNode.id] = struct{}{}
			}
		}

		var lastBlockOrWait *stepNode
		for _, step := range groupNode.subSteps {
			// Track step dependencies.
			if dependsOn, ok := step.src["depends_on"]; !ok {
				deps := toStringList(dependsOn)
				for _, dep := range deps {
					if depNode, ok := nodes.byName(dep); ok {
						nodes.addDep(step.id, depNode.id)
					}
				}
			} else if lastBlockOrWait != nil {
				nodes.addDep(step.id, lastBlockOrWait.id)
			}

			// Add all group common deps.
			for dep := range commonDeps {
				nodes.addDep(step.id, dep)
			}

			if isBlockOrWait(step.src) {
				lastBlockOrWait = step
			}
		}
	}

	// Apply tags filter.
	hits := make(map[string]struct{})
	rejects := make(map[string]struct{})
	for _, g := range groupNodes {
		rejectGroup := filter.reject(g)
		hitGroup := filter.hit(g)

		for _, step := range g.subSteps {
			if rejectGroup || filter.reject(step) {
				// when the group is rejected, all steps are rejected.
				rejects[step.id] = struct{}{}
			} else if filter.hit(step) {
				hits[step.id] = struct{}{}
				hitGroup = true
			}
		}

		if hitGroup {
			hits[g.id] = struct{}{}
		}
	}
	nodes.rejectDeps(rejects)
	nodes.markDeps(hits)

	// Finalize the conversion.
	var bkGroups []*bkPipelineGroup
	for _, groupNode := range groupNodes {
		if !groupNode.hit() {
			continue
		}

		bkGroup, err := c.convertGroup(groupNode)
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
