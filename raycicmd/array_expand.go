package raycicmd

import (
	"fmt"
	"slices"
)

// expandArraySteps expands array steps into resolvedSteps and resolves
// depends_on references to point to the expanded keys.
func expandArraySteps(gs []*pipelineGroup) error {
	configs := make(map[string]*arrayConfig)

	// Pass 1: expand array steps into resolvedSteps.
	for _, g := range gs {
		if err := g.buildResolvedSteps(configs); err != nil {
			return fmt.Errorf("expand arrays: %w", err)
		}
	}

	// Pass 2: resolve depends_on references that point to array steps.
	for _, g := range gs {
		for _, rs := range g.resolvedSteps {
			dependsOn, ok := rs.src["depends_on"]
			if !ok {
				continue
			}
			resolved, err := resolveDependsOn(
				dependsOn, configs,
			)
			if err != nil {
				return fmt.Errorf(
					"resolve depends_on for step %q: %w",
					stepKey(rs.src), err,
				)
			}
			rs.resolvedDependsOn = resolved
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

// arraySelector represents a dependency selector with optional array filter.
type arraySelector struct {
	key    string              // step identifier: "key" for command steps, "name" for wanda steps
	filter map[string][]string // dimension -> allowed values (nil = all instances)
}

// parseArrayDependsOn parses a depends_on field into a list of selectors.
//
// Supported YAML formats:
//
//	depends_on: step-key              # single string
//	depends_on: [step-a, step-b]      # string array
//	depends_on:                       # selector with array filter
//	  - ray-build:
//	      python: "3.11"
func parseArrayDependsOn(v any) ([]*arraySelector, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case string:
		return []*arraySelector{{key: val}}, nil
	case []string:
		selectors := make([]*arraySelector, len(val))
		for i, key := range val {
			selectors[i] = &arraySelector{key: key}
		}
		return selectors, nil
	case []any:
		return parseArrayDependsOnList(val)
	default:
		return nil, fmt.Errorf("depends_on must be string or array, got %T", v)
	}
}

func parseArrayDependsOnList(arr []any) ([]*arraySelector, error) {
	var selectors []*arraySelector
	for i, item := range arr {
		switch val := item.(type) {
		case string:
			selectors = append(selectors, &arraySelector{key: val})
		case map[string]any:
			sel, err := parseArraySelectorMap(val)
			if err != nil {
				return nil, fmt.Errorf("depends_on[%d]: %w", i, err)
			}
			selectors = append(selectors, sel)
		default:
			return nil, fmt.Errorf("depends_on[%d]: expected string or map, got %T", i, item)
		}
	}
	return selectors, nil
}

func parseArraySelectorMap(m map[string]any) (*arraySelector, error) {
	if len(m) != 1 {
		return nil, fmt.Errorf("selector must have exactly one key (the step name), got %d", len(m))
	}
	var stepName string
	var filterVal any
	for k, v := range m {
		stepName = k
		filterVal = v
	}

	sel := &arraySelector{key: stepName}

	if filterVal != nil {
		filterMap, ok := filterVal.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("selector %q filter must be a map, got %T", stepName, filterVal)
		}
		parsed, err := parseSelectorFilter(filterMap)
		if err != nil {
			return nil, fmt.Errorf("in selector for %q: %w", stepName, err)
		}
		sel.filter = parsed
	}

	return sel, nil
}

func parseSelectorFilter(m map[string]any) (map[string][]string, error) {
	result := make(map[string][]string, len(m))
	for dim, v := range m {
		switch val := v.(type) {
		case string:
			result[dim] = []string{val}
		case []any:
			values, err := toStringSlice(val)
			if err != nil {
				return nil, fmt.Errorf("selector %q: %w", dim, err)
			}
			result[dim] = values
		default:
			return nil, fmt.Errorf("selector %q must be a string or array, got %T", dim, v)
		}
	}
	return result, nil
}

// resolveDependsOn resolves a depends_on field to concrete step keys.
// Plain references to array steps fan out to all instances.
// Selector references resolve to only matching instances.
func resolveDependsOn(dependsOn any, configs map[string]*arrayConfig) ([]string, error) {
	selectors, err := parseArrayDependsOn(dependsOn)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, sel := range selectors {
		cfg, isArray := configs[sel.key]
		if !isArray {
			if sel.filter != nil {
				return nil, fmt.Errorf(
					"cannot use array selector on non-array step %q",
					sel.key,
				)
			}
			result = append(result, sel.key)
			continue
		}
		matches, err := resolveArraySelector(sel, cfg)
		if err != nil {
			return nil, err
		}
		result = append(result, matches...)
	}

	return result, nil
}

// resolveArraySelector resolves a selector against an array config to concrete step keys.
func resolveArraySelector(sel *arraySelector, cfg *arrayConfig) ([]string, error) {
	for dim := range sel.filter {
		if _, ok := cfg.dims[dim]; !ok {
			return nil, fmt.Errorf(
				"selector dimension %q not found in array for %q (valid: %v)",
				dim, sel.key, cfg.sortedDimensions(),
			)
		}
	}

	elements := cfg.expand()
	var matches []string
	for _, elem := range elements {
		if elem.matchesSelector(sel) {
			matches = append(matches, elem.generateKey(sel.key))
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no matches for selector {key: %q, array: %v}", sel.key, sel.filter)
	}

	return matches, nil
}

func (elem *arrayElement) matchesSelector(sel *arraySelector) bool {
	for dim, allowedVals := range sel.filter {
		if !slices.Contains(allowedVals, elem.values[dim]) {
			return false
		}
	}
	return true
}
