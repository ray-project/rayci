package raycicmd

import (
	"fmt"
	"slices"
	"strings"
)

const (
	selectorLiteral  int = iota // no parens: pass through
	selectorImplicit            // ($): match on shared dimensions
	selectorMatchAll            // (*): all variants
	selectorFilter              // (key=val): explicit filter
)

// arraySelector represents a dependency selector with optional array filter.
type arraySelector struct {
	key    string              // step name (without parens)
	mode   int                 // how to resolve the dependency
	filter map[string][]string // only set when mode == selectorFilter
}

// parseSelector parses a depends_on string with optional paren syntax.
//
//	"ray-cpu-build"                       → literal
//	"ray-cpu-build($)"                    → implicit dimension matching
//	"ray-cpu-build(*)"                    → all variants
//	"ray-cpu-build(python=3.10, cuda=12)" → explicit filter
func parseSelector(s string) (*arraySelector, error) {
	open := strings.Index(s, "(")
	if open < 0 {
		return &arraySelector{key: s}, nil
	}
	if open == 0 {
		return nil, fmt.Errorf(
			"depends_on %q: step name before '(' is empty", s,
		)
	}
	if !strings.HasSuffix(s, ")") {
		return nil, fmt.Errorf(
			"depends_on %q: missing closing ')'", s,
		)
	}
	base := s[:open]
	content := s[open+1 : len(s)-1]

	switch content {
	case "$":
		return &arraySelector{key: base, mode: selectorImplicit}, nil
	case "*":
		return &arraySelector{key: base, mode: selectorMatchAll}, nil
	case "":
		return nil, fmt.Errorf(
			"depends_on %q: empty parentheses", s,
		)
	}

	filter, err := parseSelectorFilter(content)
	if err != nil {
		return nil, fmt.Errorf("depends_on %q: %w", s, err)
	}
	return &arraySelector{
		key: base, mode: selectorFilter, filter: filter,
	}, nil
}

// parseSelectorFilter parses "key=val, key=val" into a filter map.
// Trailing or double commas are rejected.
func parseSelectorFilter(content string) (map[string][]string, error) {
	filter := make(map[string][]string)
	pairs := strings.Split(content, ",")
	for i, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			return nil, fmt.Errorf(
				"empty filter entry at position %d", i+1,
			)
		}
		eq := strings.Index(pair, "=")
		if eq < 0 {
			return nil, fmt.Errorf(
				"invalid filter %q: expected key=value", pair,
			)
		}
		dim := strings.TrimSpace(pair[:eq])
		val := strings.TrimSpace(pair[eq+1:])
		if dim == "" {
			return nil, fmt.Errorf(
				"invalid filter %q: empty dimension name", pair,
			)
		}
		if val == "" {
			return nil, fmt.Errorf(
				"invalid filter %q: empty value", pair,
			)
		}
		if slices.Contains(filter[dim], val) {
			return nil, fmt.Errorf(
				"duplicate filter entry %s=%s", dim, val,
			)
		}
		filter[dim] = append(filter[dim], val)
	}
	return filter, nil
}

// parseArrayDependsOn parses a depends_on field into a list of selectors.
//
// Supported YAML formats:
//
//	depends_on: step-key                  # literal pass-through
//	depends_on: ray-build($)              # implicit dimension matching
//	depends_on: ray-build(*)              # all variants
//	depends_on: ray-build(python=3.11)    # explicit filter
//	depends_on: [step-a($), step-b]       # array with selectors
func parseArrayDependsOn(v any) ([]*arraySelector, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case string:
		sel, err := parseSelector(val)
		if err != nil {
			return nil, err
		}
		return []*arraySelector{sel}, nil
	case []string: // from Go callers constructing steps programmatically
		selectors := make([]*arraySelector, len(val))
		for i, s := range val {
			sel, err := parseSelector(s)
			if err != nil {
				return nil, fmt.Errorf("depends_on[%d]: %w", i, err)
			}
			selectors[i] = sel
		}
		return selectors, nil
	case []any:
		return parseArrayDependsOnList(val)
	default:
		return nil, fmt.Errorf(
			"depends_on must be string or array, got %T", v,
		)
	}
}

func parseArrayDependsOnList(arr []any) ([]*arraySelector, error) {
	var selectors []*arraySelector
	for i, item := range arr {
		switch val := item.(type) {
		case string:
			sel, err := parseSelector(val)
			if err != nil {
				return nil, fmt.Errorf("depends_on[%d]: %w", i, err)
			}
			selectors = append(selectors, sel)
		default:
			return nil, fmt.Errorf(
				"depends_on[%d]: expected string, got %T; "+
					"use selector syntax like step(key=val)",
				i, item,
			)
		}
	}
	return selectors, nil
}

// resolveDependsOn resolves a depends_on field to concrete step keys.
//
// Selector semantics:
//   - No parens (literal): pass through if non-array; error if array
//   - ($): implicit dimension matching via overlapping dims
//   - (*): all variants
//   - (key=val): explicit filter
func resolveDependsOn(
	dependsOn any,
	configs map[string]*arrayConfig,
	currentElem *arrayElement,
) ([]string, error) {
	selectors, err := parseArrayDependsOn(dependsOn)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, sel := range selectors {
		cfg, isArray := configs[sel.key]

		if !isArray {
			if sel.mode != selectorLiteral {
				return nil, fmt.Errorf(
					"cannot use array selector on non-array step %q",
					sel.key,
				)
			}
			result = append(result, sel.key)
			continue
		}

		origMode := sel.mode

		switch sel.mode {
		case selectorLiteral:
			return nil, fmt.Errorf(
				"plain depends_on %q targets an array step; "+
					"use ($), (*), or (key=val) suffix",
				sel.key,
			)

		case selectorImplicit:
			if currentElem == nil {
				return nil, fmt.Errorf(
					"($) on %q can only be used from an array "+
						"step; use %s(*) for non-array steps",
					sel.key, sel.key,
				)
			}
			f := implicitDimFilter(currentElem, cfg)
			if f == nil {
				return nil, fmt.Errorf(
					"($) on %q: no overlapping dimensions; "+
						"use %s(*) to depend on all variants",
					sel.key, sel.key,
				)
			}
			sel = &arraySelector{
				key: sel.key, mode: selectorFilter, filter: f,
			}

		case selectorMatchAll, selectorFilter:
			// use sel as-is
		}

		matches, err := resolveArraySelector(sel, cfg)
		if err != nil {
			if origMode == selectorImplicit {
				return nil, fmt.Errorf(
					"implicit dimension matching on %q: %w; "+
						"use %s(*) to depend on all variants",
					sel.key, err, sel.key,
				)
			}
			return nil, err
		}
		result = append(result, matches...)
	}

	return result, nil
}

// implicitDimFilter builds a selector filter from the overlapping
// dimensions between the current element and the target config.
// Returns nil if there is no overlap.
func implicitDimFilter(
	current *arrayElement, target *arrayConfig,
) map[string][]string {
	var filter map[string][]string
	for dim := range target.dims {
		val, ok := current.values[dim]
		if !ok {
			continue
		}
		if filter == nil {
			filter = make(map[string][]string)
		}
		filter[dim] = []string{val}
	}
	return filter
}

// resolveArraySelector resolves a selector against an array config
// to concrete step keys, using the final element list (post-adjustments).
func resolveArraySelector(sel *arraySelector, cfg *arrayConfig) ([]string, error) {
	if sel.mode == selectorMatchAll {
		matches := make([]string, len(cfg.elements))
		for i, elem := range cfg.elements {
			matches[i] = elem.generateKey(sel.key)
		}
		return matches, nil
	}

	for dim := range sel.filter {
		if _, ok := cfg.dims[dim]; !ok {
			return nil, fmt.Errorf(
				"selector dimension %q not found in array for %q (valid: %v)",
				dim, sel.key, cfg.sortedDimensions(),
			)
		}
	}

	var matches []string
	for _, elem := range cfg.elements {
		if elem.matchesSelector(sel) {
			matches = append(matches, elem.generateKey(sel.key))
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf(
			"selector on %q with filter %v matches no elements",
			sel.key, sel.filter,
		)
	}

	return matches, nil
}

// resolveGroupDependsOn resolves group-level depends_on entries.
//
// Unlike step-level resolveDependsOn, groups are not array elements
// themselves, so:
//   - Literal references to array steps expand to all variants (same as *).
//   - ($) is rejected because there is no current element for dimension
//     matching.
//   - (*) and (key=val) work the same as step-level.
func resolveGroupDependsOn(
	deps []string, configs map[string]*arrayConfig,
) ([]string, error) {
	result := make([]string, 0, len(deps))
	for _, dep := range deps {
		sel, err := parseSelector(dep)
		if err != nil {
			return nil, err
		}

		cfg, isArray := configs[sel.key]

		if !isArray {
			if sel.mode != selectorLiteral {
				return nil, fmt.Errorf(
					"cannot use array selector on non-array step %q",
					sel.key,
				)
			}
			result = append(result, sel.key)
			continue
		}

		switch sel.mode {
		case selectorLiteral:
			sel = &arraySelector{
				key: sel.key, mode: selectorMatchAll,
			}
		case selectorImplicit:
			return nil, fmt.Errorf(
				"($) on %q cannot be used in group depends_on; "+
					"use %s(*) or an explicit filter",
				sel.key, sel.key,
			)
		case selectorMatchAll, selectorFilter:
			// use as-is
		}

		matches, err := resolveArraySelector(sel, cfg)
		if err != nil {
			return nil, err
		}
		result = append(result, matches...)
	}
	return result, nil
}

func (elem *arrayElement) matchesSelector(sel *arraySelector) bool {
	for dim, allowedVals := range sel.filter {
		if !slices.Contains(allowedVals, elem.values[dim]) {
			return false
		}
	}
	return true
}
