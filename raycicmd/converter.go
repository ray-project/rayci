package raycicmd

import (
	"fmt"
)

type converter struct {
	config *config
	info   *buildInfo
	envMap map[string]string

	stepConverters []stepConverter

	commandConverter *commandConverter
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
			"s3://%s/%s/%s",
			c.config.ArtifactsBucket,
			info.gitCommit,
			info.buildID,
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
	if c.config.AllowTriggerStep {
		c.stepConverters = append(c.stepConverters, triggerConverter)
	}
	c.commandConverter = newCommandConverter(config, info, envMap)

	return c
}

func (c *converter) convertStep(id string, step map[string]any) (
	map[string]any, error,
) {
	for _, stepConverter := range c.stepConverters {
		if stepConverter.match(step) {
			return stepConverter.convert(id, step)
		}
	}
	return c.commandConverter.convert(id, step)
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

	c.commandConverter.setDefaultJobEnv(g.DefaultJobEnv)

	for _, step := range n.subSteps {
		if !step.hit() {
			continue
		}

		// convert step to buildkite step
		bkStep, err := c.convertStep(step.id, step.src)
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

func mergeStringSlices(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	result := make([]string, 0, len(a)+len(b))
	result = append(result, a...)
	result = append(result, b...)
	return result
}

func (c *converter) convertGroups(gs []*pipelineGroup, filter *stepFilter) (
	[]*bkPipelineGroup, error,
) {
	set := newStepNodeSet()
	var groupNodes []*stepNode

	for i, g := range gs {
		groupNode := &stepNode{
			id:       fmt.Sprintf("g%d", i),
			key:      g.Key,
			srcGroup: g,
			tags:     g.Tags,
		}

		for j, step := range g.Steps {
			// Merge group tags with step-specific tags so that steps inherit
			// group-level tags for conditional pipeline selection.
			tags := mergeStringSlices(g.Tags, stepTags(step))

			stepNode := &stepNode{
				id:   fmt.Sprintf("g%d_s%d", i, j),
				key:  stepKey(step),
				tags: tags,
				src:  step,
			}
			set.add(stepNode)
			groupNode.subSteps = append(groupNode.subSteps, stepNode)
		}

		set.add(groupNode)
		groupNodes = append(groupNodes, groupNode)
	}

	if err := set.buildIndex(); err != nil {
		return nil, fmt.Errorf("build index: %w", err)
	}

	// Populate dependsOn.
	for _, groupNode := range groupNodes {
		// A group node is different from a step node.
		//
		// When a group is being depended on, all its steps are being depended
		// on. The gating is equivalent to wait step at the end of the group.
		//
		// When a group depends on a node, it means all its steps depend on the
		// node. The gating is equivalent to wait step at the beginning of the
		// group.
		groupDeps := make(map[string]struct{})
		for _, dep := range groupNode.srcGroup.DependsOn {
			if depNode, ok := set.byKey(dep); ok {
				groupDeps[depNode.id] = struct{}{}
			}
		}

		var lastBlockOrWait *stepNode
		for _, step := range groupNode.subSteps {
			// Track step dependencies.
			if dependsOn, ok := step.src["depends_on"]; ok {
				deps := toStringList(dependsOn)
				for _, dep := range deps {
					if depNode, ok := set.byKey(dep); ok {
						set.addDep(step.id, depNode.id)
					}
				}
			} else if lastBlockOrWait != nil {
				set.addDep(step.id, lastBlockOrWait.id)
			}

			// Add all group common deps.
			for dep := range groupDeps {
				set.addDep(step.id, dep)
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
		// If a group is rejected, everything in the group is rejected.
		// and when all steps are rejected, the group will be removed.
		// We still need to keep the group's structure to reject steps in other
		// groups that depend on steps in this group.
		groupRejected := filter.reject(g)

		// If a group is accept, then steps in this group is marked as included.
		// and all this groups dependencies will be included. Individual steps
		// in this group can still be rejected.
		groupAccepted := filter.accept(g)

		for _, step := range g.subSteps {
			if groupRejected || filter.reject(step) {
				// when the group is rejected, all steps are rejected.
				rejects[step.id] = struct{}{}
			} else if filter.accept(step) {
				hits[step.id] = struct{}{}
				groupAccepted = true
			}
		}

		if groupAccepted {
			hits[g.id] = struct{}{}
		}
	}
	set.markDeps(hits)
	set.rejectDeps(rejects)

	// Finalize the conversion.
	var bkGroups []*bkPipelineGroup
	for _, groupNode := range groupNodes {
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
