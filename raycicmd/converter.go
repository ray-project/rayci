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

// matrixExpansionContext tracks matrix steps during expansion for later
// dependency resolution. When a step depends on a matrix step (e.g., "ray-build"),
// we need to know what keys it expanded into (e.g., ["ray-build-python310", ...]).
type matrixExpansionContext struct {
	stepKeyToConfig   map[string]*matrixConfig
	stepKeyToExpanded map[string][]string
}

func expandMatrixInGroups(gs []*pipelineGroup) ([]*pipelineGroup, *matrixExpansionContext, error) {
	ctx := &matrixExpansionContext{
		stepKeyToConfig:   make(map[string]*matrixConfig),
		stepKeyToExpanded: make(map[string][]string),
	}
	var result []*pipelineGroup

	for _, g := range gs {
		newGroup := &pipelineGroup{
			filename:      g.filename,
			sortKey:       g.sortKey,
			Group:         g.Group,
			Key:           g.Key,
			Tags:          g.Tags,
			SortKey:       g.SortKey,
			DependsOn:     g.DependsOn,
			DefaultJobEnv: g.DefaultJobEnv,
		}

		for _, step := range g.Steps {
			matrixDef, hasMatrix := step["matrix"]
			if !hasMatrix {
				// No matrix, keep step as-is
				newGroup.Steps = append(newGroup.Steps, step)
				continue
			}

			baseKey := stepKey(step)
			if baseKey == "" {
				// No key - pass through to Buildkite for native matrix handling
				newGroup.Steps = append(newGroup.Steps, step)
				continue
			}

			// Parse matrix configuration
			cfg, err := parseMatrixConfig(matrixDef)
			if err != nil {
				return nil, nil, fmt.Errorf("parse matrix in step %q: %w", stepKey(step), err)
			}

			// Validate label has placeholder
			if label, ok := step["label"].(string); ok {
				if !hasMatrixPlaceholder(label) {
					return nil, nil, fmt.Errorf("matrix step %q: label must contain {{matrix...}} placeholder", baseKey)
				}
			}

			// Register for selector expansion
			ctx.stepKeyToConfig[baseKey] = cfg

			instances := cfg.expand()
			if len(instances) == 0 {
				return nil, nil, fmt.Errorf("matrix step %q: no instances after expansion", baseKey)
			}

			var expandedKeysList []string
			for _, inst := range instances {
				expandedStep := inst.substituteValues(step).(map[string]any)

				expandedKey := inst.generateKey(baseKey, cfg)
				if _, hasName := expandedStep["name"]; hasName {
					expandedStep["name"] = expandedKey
				} else {
					expandedStep["key"] = expandedKey
				}
				delete(expandedStep, "matrix")

				originalTags := stepTags(step)
				matrixTags := inst.generateTags()
				allTags := append([]string{}, originalTags...)
				allTags = append(allTags, matrixTags...)
				if len(allTags) > 0 {
					expandedStep["tags"] = allTags
				}

				expandedKeysList = append(expandedKeysList, expandedKey)
				newGroup.Steps = append(newGroup.Steps, expandedStep)
			}

			ctx.stepKeyToExpanded[baseKey] = expandedKeysList
		}

		result = append(result, newGroup)
	}

	return result, ctx, nil
}

// expandDependsOnSelectors processes depends_on to expand matrix selectors.
func expandDependsOnSelectors(dependsOn any, ctx *matrixExpansionContext) ([]string, error) {
	selectors, err := parseMatrixDependsOn(dependsOn)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, sel := range selectors {
		if sel.Matrix == nil {
			// Simple key reference - check if it's a matrix step
			if expanded, ok := ctx.stepKeyToExpanded[sel.Key]; ok {
				// Matrix step: expand to all expanded keys
				result = append(result, expanded...)
			} else {
				// Non-matrix step: use key as-is
				result = append(result, sel.Key)
			}
		} else {
			matches, err := sel.expand(ctx.stepKeyToConfig)
			if err != nil {
				return nil, err
			}
			result = append(result, matches...)
		}
	}

	return result, nil
}

func (c *converter) convertGroups(gs []*pipelineGroup, filter *stepFilter) (
	[]*bkPipelineGroup, error,
) {
	expandedGroups, matrixCtx, err := expandMatrixInGroups(gs)
	if err != nil {
		return nil, fmt.Errorf("expand matrix: %w", err)
	}

	set := newStepNodeSet()
	var groupNodes []*stepNode

	for i, g := range expandedGroups {
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
				// Expand matrix selectors in depends_on
				expandedDeps, err := expandDependsOnSelectors(dependsOn, matrixCtx)
				if err != nil {
					return nil, fmt.Errorf("expand depends_on for step %q: %w", step.key, err)
				}

				// Update the step source with expanded deps for Buildkite output
				if len(expandedDeps) == 1 {
					step.src["depends_on"] = expandedDeps[0]
				} else {
					step.src["depends_on"] = expandedDeps
				}

				for _, dep := range expandedDeps {
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
		groupAllAccepted := filter.accept(g)

		// This will be flipped if any step in the group is accepted.
		// This will stay false if the group is empty or all steps are rejected.
		groupAnyAccepted := false

		for _, step := range g.subSteps {
			if groupRejected || filter.reject(step) {
				// when the group is rejected, all steps are rejected.
				rejects[step.id] = struct{}{}
			} else if filter.accept(step) || groupAllAccepted {
				hits[step.id] = struct{}{}
				groupAnyAccepted = true
			}
		}

		// If any step in the group is accepted, the group is accepted.
		// and this will accept all dependencies of this group later on.
		if groupAnyAccepted {
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
