package raycicmd

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// dimension is a named dimension in a matrix (e.g., "python", "cuda").
type dimension string

// anonymousDim is used for simple array matrices: matrix: ["a", "b", "c"]
const anonymousDim dimension = ""

// variant is a value within a dimension (e.g., "3.10", "12.1.1").
type variant string

// matrixConfig represents an expanded step's matrix definition.
type matrixConfig struct {
	Setup map[dimension][]variant
}

// matrixInstance represents a single expanded matrix combination.
type matrixInstance struct {
	Values map[dimension]variant
}

// matrixSelector represents a dependency selector with optional matrix filter.
type matrixSelector struct {
	Key    string                  // base step key
	Matrix map[dimension][]variant // partial dimension constraints (nil = all), each dimension can match multiple values
}

// parseMatrixConfig parses the matrix field from a step.
// Named dimensions matrix
//
//	matrix:
//	  setup:
//	    python: ["3.10", "3.11"]
//	    cuda: ["12.1.1", "12.8.1"]
//
// Simple array matrix (no dimension name)
//
//	matrix:
//	  - "darwin"
//	  - "Linux"
//	  - "Windows"
func parseMatrixConfig(v any) (*matrixConfig, error) {
	cfg := &matrixConfig{
		Setup: make(map[dimension][]variant),
	}

	switch val := v.(type) {
	case []any:
		// Simple array: matrix: ["3.10", "3.11"]
		variants, err := anySliceToVariantSlice(val)
		if err != nil {
			return nil, fmt.Errorf("invalid matrix array: %w", err)
		}
		cfg.Setup[anonymousDim] = variants

	case map[string]any:
		// Map with setup and optional adjustments
		if setup, ok := val["setup"]; ok {
			setupMap, ok := setup.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("matrix.setup must be a map")
			}
			for dim, vals := range setupMap {
				valsSlice, ok := vals.([]any)
				if !ok {
					return nil, fmt.Errorf("matrix.setup.%s must be an array", dim)
				}
				variants, err := anySliceToVariantSlice(valsSlice)
				if err != nil {
					return nil, fmt.Errorf("invalid values for dimension %s: %w", dim, err)
				}
				cfg.Setup[dimension(dim)] = variants
			}
		} else {
			return nil, fmt.Errorf("matrix map must have 'setup' key")
		}

		if _, ok := val["adjustments"]; ok {
			return nil, fmt.Errorf("matrix.adjustments is not supported")
		}

	default:
		return nil, fmt.Errorf("matrix must be an array or map, got %T", v)
	}

	return cfg, nil
}

func (cfg *matrixConfig) sortedDimensions() []dimension {
	dims := make([]dimension, 0, len(cfg.Setup))
	for dim := range cfg.Setup {
		dims = append(dims, dim)
	}
	sort.Slice(dims, func(i, j int) bool { return dims[i] < dims[j] })
	return dims
}

// expand generates all combinations from a matrix config.
func (cfg *matrixConfig) expand() []*matrixInstance {
	if len(cfg.Setup) == 0 {
		return nil
	}

	result := []*matrixInstance{{Values: make(map[dimension]variant)}}
	for _, dim := range cfg.sortedDimensions() {
		var expanded []*matrixInstance
		for _, inst := range result {
			for _, val := range cfg.Setup[dim] {
				newInst := &matrixInstance{Values: maps.Clone(inst.Values)}
				newInst.Values[dim] = val
				expanded = append(expanded, newInst)
			}
		}
		result = expanded
	}
	return result
}

// generateKey creates the expanded key for an instance.
// Format: {base-key}-{dim1}{val1}-{dim2}{val2} (dims sorted alphabetically)
func (inst *matrixInstance) generateKey(baseKey string, cfg *matrixConfig) string {
	parts := []string{baseKey}
	for _, dim := range cfg.sortedDimensions() {
		parts = append(parts, sanitizeKeyPart(string(dim)+string(inst.Values[dim])))
	}
	return strings.Join(parts, "-")
}

// sanitizeKeyPart removes invalid characters from a key part.
// Buildkite keys allow: alphanumeric, underscores, dashes, colons.
// We exclude dashes here since they're used as separators between parts.
func sanitizeKeyPart(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == ':' {
			return r
		}
		return -1 // drop character
	}, s)
}

// generateTags creates auto-generated tags for matrix values.
// These tags are used to tag the expanded steps and depends_on selectors.
// Format: {dim1}-{val1}, {dim2}-{val2} (dims sorted alphabetically)
func (inst *matrixInstance) generateTags() []string {
	var tags []string
	for dim, val := range inst.Values {
		if dim == anonymousDim {
			// Simple array: just use the value as tag (e.g., 3.10)
			tags = append(tags, string(val))
		} else {
			// Named dimension: use dimension-value format (e.g., python-3.11)
			tags = append(tags, string(dim)+"-"+string(val))
		}
	}
	sort.Strings(tags)
	return tags
}

// substituteValues replaces {{matrix.X}} placeholders in any value.
// It recursively traverses maps and slices, returning a new structure with
// all string values substituted. Non-string leaf values are returned as-is.
func (inst *matrixInstance) substituteValues(v any) any {
	switch val := v.(type) {
	case string:
		var replacerArgs []string
		for dim, dimVal := range inst.Values {
			if dim == anonymousDim {
				// Simple array: use {{matrix}}
				replacerArgs = append(replacerArgs, "{{matrix}}", string(dimVal))
			} else {
				// Named dimension: use {{matrix.dim}} (e.g., {{matrix.python}})
				replacerArgs = append(replacerArgs, "{{matrix."+string(dim)+"}}", string(dimVal))
			}
		}
		return strings.NewReplacer(replacerArgs...).Replace(val)

	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			result[k] = inst.substituteValues(v)
		}
		return result

	case []any:
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = inst.substituteValues(v)
		}
		return result

	default:
		return v
	}
}

// parseMatrixDependsOn parses a depends_on field into a list of selectors.
//
// Supported YAML formats:
//
//	depends_on: step-key              # single string
//	depends_on: [step-a, step-b]      # string array
//	depends_on:                       # selector with matrix filter
//	  - key: ray-build
//	    matrix:
//	      python: "3.11"
func parseMatrixDependsOn(v any) ([]*matrixSelector, error) {
	if v == nil {
		return nil, nil
	}

	switch val := v.(type) {
	case string:
		return []*matrixSelector{{Key: val}}, nil

	case []string:
		var selectors []*matrixSelector
		for _, key := range val {
			selectors = append(selectors, &matrixSelector{Key: key})
		}
		return selectors, nil

	case []any:
		var selectors []*matrixSelector
		for i, item := range val {
			switch itemVal := item.(type) {
			case string:
				selectors = append(selectors, &matrixSelector{Key: itemVal})

			case map[string]any:
				sel, err := parseMatrixSelectorMap(itemVal)
				if err != nil {
					return nil, fmt.Errorf("depends_on[%d]: %w", i, err)
				}
				selectors = append(selectors, sel)

			default:
				return nil, fmt.Errorf("depends_on[%d]: unexpected type %T", i, item)
			}
		}
		return selectors, nil

	default:
		return nil, fmt.Errorf("depends_on must be string or array, got %T", v)
	}
}

// parseMatrixSelectorMap parses a selector map into a matrixSelector.
//
// Example YAML:
//
//	key: ray-build
//	matrix:
//	  python: ["3.10", "3.11"]
//	  cuda: "12.1.1"
func parseMatrixSelectorMap(m map[string]any) (*matrixSelector, error) {
	sel := &matrixSelector{}

	key, ok := m["key"]
	if !ok {
		return nil, fmt.Errorf("selector missing 'key' field")
	}
	keyStr, ok := key.(string)
	if !ok {
		return nil, fmt.Errorf("selector 'key' must be a string")
	}
	sel.Key = keyStr

	if matrix, ok := m["matrix"]; ok {
		matrixMap, ok := matrix.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("selector 'matrix' must be a map")
		}
		sel.Matrix = make(map[dimension][]variant)
		for k, v := range matrixMap {
			dim := dimension(k)
			switch val := v.(type) {
			case string:
				sel.Matrix[dim] = []variant{variant(val)}
			case []any:
				variants, err := anySliceToVariantSlice(val)
				if err != nil {
					return nil, fmt.Errorf("selector 'matrix.%s': %w", k, err)
				}
				sel.Matrix[dim] = variants
			case []string:
				variants := make([]variant, len(val))
				for i, s := range val {
					variants[i] = variant(s)
				}
				sel.Matrix[dim] = variants
			default:
				return nil, fmt.Errorf("selector 'matrix.%s' must be a string or array", k)
			}
		}
	}

	return sel, nil
}

// expand returns the expanded step keys that match this selector.
//
// For example, if ray-build expanded to [ray-build-python310, ray-build-python311]
// and the selector filters to python: "3.11", this returns [ray-build-python311].
func (sel *matrixSelector) expand(
	stepKeyToConfig map[string]*matrixConfig,
	stepKeyToExpanded map[string][]string,
) ([]string, error) {
	cfg, ok := stepKeyToConfig[sel.Key]
	if !ok {
		return []string{sel.Key}, nil
	}

	if sel.Matrix == nil {
		return []string{sel.Key}, nil
	}

	for dim := range sel.Matrix {
		if _, ok := cfg.Setup[dim]; !ok {
			return nil, fmt.Errorf("selector dimension %q not found in matrix for %q", dim, sel.Key)
		}
	}

	allExpanded := stepKeyToExpanded[sel.Key]
	instances := cfg.expand()

	if len(allExpanded) != len(instances) {
		return nil, fmt.Errorf(
			"internal error: expanded keys count (%d) != instances count (%d) for %q",
			len(allExpanded), len(instances), sel.Key,
		)
	}

	var matches []string
	for i, inst := range instances {
		if inst.matches(sel) {
			matches = append(matches, allExpanded[i])
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no matches for selector {key: %q, matrix: %v}", sel.Key, sel.Matrix)
	}

	return matches, nil
}

func (inst *matrixInstance) matches(sel *matrixSelector) bool {
	for dim, allowedVals := range sel.Matrix {
		if !slices.Contains(allowedVals, inst.Values[dim]) {
			return false
		}
	}
	return true
}

// hasMatrixPlaceholder checks if a string contains any {{matrix...}} placeholder.
func hasMatrixPlaceholder(s string) bool {
	return strings.Contains(s, "{{matrix")
}

// anySliceToVariantSlice converts []any to []variant.
func anySliceToVariantSlice(arr []any) ([]variant, error) {
	result := make([]variant, len(arr))
	for i, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("element %d is not a string: %T", i, v)
		}
		result[i] = variant(s)
	}
	return result, nil
}
