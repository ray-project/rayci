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
		if !step.marked {
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
			stepNode := &stepNode{
				id:   fmt.Sprintf("g%d_s%d", i, j),
				key:  stepKey(step),
				tags: stepTags(step),
				src:  step,
			}
			set.add(stepNode)
			groupNode.subSteps = append(groupNode.subSteps, stepNode)
		}

		set.add(groupNode)
		groupNodes = append(groupNodes, groupNode)
	}

	// Build index and check if we have duplicated user keys.
	if err := set.buildIndex(); err != nil {
		return nil, fmt.Errorf("build index: %w", err)
	}

	// Apply tags filter.
	for _, groupNode := range groupNodes {
		if !filter.hit(groupNode) {
			continue // group is rejected
		}

		groupNode.marked = true
		for _, step := range groupNode.subSteps {
			if filter.hit(step) {
				step.marked = true
				groupNode.marked = true // include group if any step is included
			}
		}
	}

	// TODO(aslonnie): for any included node, also include its dependencies.

	var bkGroups []*bkPipelineGroup
	for _, groupNode := range groupNodes {
		if !groupNode.marked {
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
