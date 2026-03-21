package raycicmd

import (
	"fmt"
	"maps"
	"slices"
)

// expandArraySteps expands array steps into resolvedSteps and resolves
// depends_on references to point to the expanded keys.
func expandArraySteps(gs []*pipelineGroup) error {
	configs := make(map[string]*arrayConfig)
	// elems maps each expanded step key to its arrayElement,
	// used for implicit dimension matching in Pass 2.
	elems := make(map[string]*arrayElement)

	// Pass 1: expand array steps into resolvedSteps.
	for _, g := range gs {
		if err := g.buildResolvedSteps(configs, elems); err != nil {
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
			currentElem := elems[stepKey(rs.src)]
			resolved, err := resolveDependsOn(
				dependsOn, configs, currentElem,
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

	// Pass 3: resolve group-level DependsOn references that point to
	// array steps. Replace each array base key with the expanded keys.
	for _, g := range gs {
		if len(g.DependsOn) == 0 {
			continue
		}
		resolved := make([]string, 0, len(g.DependsOn))
		for _, dep := range g.DependsOn {
			cfg, isArray := configs[dep]
			if !isArray {
				resolved = append(resolved, dep)
				continue
			}
			for _, elem := range cfg.elements {
				resolved = append(resolved, elem.generateKey(dep))
			}
		}
		g.DependsOn = resolved
	}

	return nil
}

// buildResolvedSteps populates g.resolvedSteps from g.Steps, expanding
// array steps into multiple entries.
func (g *pipelineGroup) buildResolvedSteps(
	configs map[string]*arrayConfig,
	elems map[string]*arrayElement,
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
		cfg := configs[baseKey]
		for j, es := range expanded {
			result = append(result, &resolvedStep{src: es})
			elems[stepKey(es)] = cfg.elements[j]
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

	// Parse and apply adjustments if present.
	var adjustments []*arrayAdjustment
	if arrayMap, ok := arrayDef.(map[string]any); ok {
		if adjDef, ok := arrayMap["adjustments"]; ok {
			adjustments, err = parseArrayAdjustments(adjDef)
			if err != nil {
				return nil, fmt.Errorf(
					"step %q: %w", baseKey, err,
				)
			}
		}
	}

	elements, err = applyAdjustments(elements, adjustments, cfg)
	if err != nil {
		return nil, fmt.Errorf(
			"step %q: %w", baseKey, err,
		)
	}

	if _, exists := configs[baseKey]; exists {
		return nil, fmt.Errorf(
			"duplicate array step key %q", baseKey,
		)
	}

	cfg.elements = elements
	configs[baseKey] = cfg

	seenKeys := make(map[string]struct{}, len(elements))
	var result []map[string]any
	for _, elem := range elements {
		// Deep-copy step, replacing {{array.X}} placeholders with
		// this element's dimension values.
		expandedStep := elem.substituteValues(step).(map[string]any)
		// e.g. "build-step" + {python:3.11, cuda:12.1.1}
		//    → "build-step--cuda1211-python311"
		expandedKey := elem.generateKey(baseKey)

		if _, dup := seenKeys[expandedKey]; dup {
			return nil, fmt.Errorf(
				"array step %q: duplicate generated key %q "+
					"(values may collide after sanitization)",
				baseKey, expandedKey,
			)
		}
		seenKeys[expandedKey] = struct{}{}

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

// applyAdjustments processes adjustments in two passes: first appends
// additions, then filters out skipped elements.
func applyAdjustments(
	elements []*arrayElement,
	adjustments []*arrayAdjustment,
	cfg *arrayConfig,
) ([]*arrayElement, error) {
	// Pass 1: append additions.
	for _, adj := range adjustments {
		if adj.skip {
			continue
		}
		for dim := range cfg.dims {
			if _, ok := adj.with[dim]; !ok {
				return nil, fmt.Errorf(
					"addition adjustment must specify all "+
						"dimensions; missing %q", dim,
				)
			}
		}
		if hasElement(elements, adj.with) {
			return nil, fmt.Errorf(
				"addition adjustment with=%v duplicates "+
					"an existing element", adj.with,
			)
		}
		elements = append(elements, &arrayElement{
			values: maps.Clone(adj.with),
		})
	}

	// Pass 2: collect skip adjustments, validate, then remove.
	var skipAdjs []*arrayAdjustment
	for _, adj := range adjustments {
		if adj.skip {
			skipAdjs = append(skipAdjs, adj)
		}
	}
	for _, adj := range skipAdjs {
		if !hasElement(elements, adj.with) {
			return nil, fmt.Errorf(
				"skip adjustment with=%v matches no element",
				adj.with,
			)
		}
	}
	elements = slices.DeleteFunc(elements, func(e *arrayElement) bool {
		for _, adj := range skipAdjs {
			if elemMatchesWith(e, adj.with) {
				return true
			}
		}
		return false
	})

	return elements, nil
}

func hasElement(
	elements []*arrayElement, with map[string]string,
) bool {
	for _, elem := range elements {
		if elemMatchesWith(elem, with) {
			return true
		}
	}
	return false
}

func elemMatchesWith(elem *arrayElement, with map[string]string) bool {
	for dim, val := range with {
		if elem.values[dim] != val {
			return false
		}
	}
	return true
}
