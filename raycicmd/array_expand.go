package raycicmd

import (
	"fmt"
)

// expandArraySteps expands array steps into resolvedSteps.
func expandArraySteps(gs []*pipelineGroup) error {
	configs := make(map[string]*arrayConfig)

	for _, g := range gs {
		if err := g.buildResolvedSteps(configs); err != nil {
			return fmt.Errorf("expand arrays: %w", err)
		}
	}

	return nil
}

// buildResolvedSteps populates g.resolvedSteps from g.Steps, expanding
// array steps into multiple entries.
func (g *pipelineGroup) buildResolvedSteps(
	configs map[string]*arrayConfig,
) error {
	var result []*resolvedStep

	for _, step := range g.Steps {
		_, hasMatrix := step["matrix"]
		arrayDef, hasArray := step["array"]
		if hasMatrix && hasArray {
			return fmt.Errorf(
				"step %q has both \"matrix\" and \"array\"; use only one",
				stepKey(step),
			)
		}
		if !hasArray {
			result = append(result, &resolvedStep{src: step})
			continue
		}

		baseKey := stepKey(step)
		if baseKey == "" {
			return fmt.Errorf(
				"step with array must have a key or name",
			)
		}

		expanded, err := expandSingleArrayStep(
			step, baseKey, arrayDef, configs,
		)
		if err != nil {
			return err
		}
		for _, es := range expanded {
			result = append(result, &resolvedStep{src: es})
		}
	}

	g.resolvedSteps = result
	return nil
}

// expandSingleArrayStep expands one array step into multiple concrete steps.
func expandSingleArrayStep(
	step map[string]any,
	baseKey string,
	arrayDef any,
	configs map[string]*arrayConfig,
) ([]map[string]any, error) {
	cfg, err := parseArrayConfig(arrayDef)
	if err != nil {
		return nil, fmt.Errorf(
			"parse array in step %q: %w", baseKey, err,
		)
	}

	if label, ok := step["label"].(string); ok {
		if !hasArrayPlaceholder(label) {
			return nil, fmt.Errorf(
				"array step %q: label must contain "+
					"{{array...}} placeholder", baseKey,
			)
		}
	}

	elements := cfg.expand()
	if len(elements) == 0 {
		return nil, fmt.Errorf(
			"array step %q: no elements after expansion", baseKey,
		)
	}

	configs[baseKey] = cfg

	var result []map[string]any
	for _, elem := range elements {
		// Deep-copy step, replacing {{array.X}} placeholders with
		// this element's dimension values.
		expandedStep := elem.substituteValues(step).(map[string]any)
		// e.g. "build-step" + {python:3.11, cuda:12.1.1}
		//    → "build-step--cuda1211-python311"
		expandedKey := elem.generateKey(baseKey)

		if _, hasName := expandedStep["name"]; hasName {
			expandedStep["name"] = expandedKey
		} else {
			expandedStep["key"] = expandedKey
		}
		delete(expandedStep, "array")

		result = append(result, expandedStep)
	}

	return result, nil
}
