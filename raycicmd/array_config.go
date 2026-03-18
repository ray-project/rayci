package raycicmd

import (
	"fmt"
	"maps"
	"sort"
	"strings"
)

// arrayConfig represents a parsed array definition. After expansion,
// elements holds the final list (post-adjustments).
type arrayConfig struct {
	dims     map[string][]string // dimension name -> values
	elements []*arrayElement     // populated during expansion
}

// arrayElement is one combination from expand().
type arrayElement struct {
	values map[string]string // dimension name -> selected value
}

// parseArrayConfig parses the array field from a step.
// The "adjustments" key, if present, is reserved and not treated as a dimension.
//
//	array:
//	  python: ["3.10", "3.11"]
//	  cuda: ["12.1.1", "12.8.1"]
//	  adjustments:
//	    - with:
//	        python: "3.10"
//	        cuda: "12.1.1"
//	      skip: true
//	    - with:
//	        python: "3.12"
//	        cuda: "12.8.1"
func parseArrayConfig(v any) (*arrayConfig, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("array must be a map, got %T", v)
	}
	if len(m) == 0 {
		return nil, fmt.Errorf("array cannot be empty")
	}

	cfg := &arrayConfig{
		dims: make(map[string][]string, len(m)),
	}
	for dim, vals := range m {
		if dim == "adjustments" {
			continue // handled separately
		}
		valsSlice, ok := vals.([]any)
		if !ok {
			return nil, fmt.Errorf("array.%s must be an array, got %T", dim, vals)
		}
		values, err := toStringSlice(valsSlice)
		if err != nil {
			return nil, fmt.Errorf("array.%s: %w", dim, err)
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("array.%s cannot be empty", dim)
		}
		cfg.dims[dim] = values
	}
	if len(cfg.dims) == 0 {
		return nil, fmt.Errorf("array must have at least one dimension")
	}
	return cfg, nil
}

func toStringSlice(arr []any) ([]string, error) {
	result := make([]string, len(arr))
	for i, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("element %d must be a string, got %T", i, v)
		}
		result[i] = s
	}
	return result, nil
}

func toStringMap(m map[string]any) (map[string]string, error) {
	result := make(map[string]string, len(m))
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf(
				"value for key %q must be a string, got %T", k, v,
			)
		}
		result[k] = s
	}
	return result, nil
}

func (cfg *arrayConfig) sortedDimensions() []string {
	dims := make([]string, 0, len(cfg.dims))
	for dim := range cfg.dims {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	return dims
}

// expand returns the cartesian product of all dimensions.
func (cfg *arrayConfig) expand() []*arrayElement {
	if len(cfg.dims) == 0 {
		return nil
	}

	result := []*arrayElement{{values: make(map[string]string)}}
	for _, dim := range cfg.sortedDimensions() {
		var expanded []*arrayElement
		for _, elem := range result {
			for _, val := range cfg.dims[dim] {
				newElem := &arrayElement{values: maps.Clone(elem.values)}
				newElem.values[dim] = val
				expanded = append(expanded, newElem)
			}
		}
		result = expanded
	}
	return result
}

// generateKey returns {base}--{dim1}{val1}-{dim2}{val2} (dims sorted).
// Double-dash separates base key from array dimensions.
func (elem *arrayElement) generateKey(baseKey string) string {
	dims := make([]string, 0, len(elem.values))
	for dim := range elem.values {
		dims = append(dims, dim)
	}
	sort.Strings(dims)

	var parts []string
	for _, dim := range dims {
		parts = append(parts, sanitizeKeyPart(dim+elem.values[dim]))
	}
	return baseKey + "--" + strings.Join(parts, "-")
}

// sanitizeKeyPart keeps alphanumeric, underscores, colons.
// Dashes are excluded since they separate dimension parts.
func sanitizeKeyPart(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == ':' {
			return r
		}
		return -1
	}, s)
}

// substituteValues deep-copies v, replacing {{array.X}} placeholders.
func (elem *arrayElement) substituteValues(v any) any {
	switch val := v.(type) {
	case string: // label: "Build {{array.python}}"
		return elem.substituteString(val)
	case map[string]any: // env: {PYTHON: "{{array.python}}"}
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = elem.substituteValues(v)
		}
		return result
	case []any: // commands: ["echo {{array.python}}"]
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = elem.substituteValues(v)
		}
		return result
	default:
		return v
	}
}

func (elem *arrayElement) substituteString(s string) string {
	if !hasArrayPlaceholder(s) {
		return s
	}
	for dim, val := range elem.values {
		s = strings.ReplaceAll(s, "{{array."+dim+"}}", val)
	}
	return s
}

func hasArrayPlaceholder(s string) bool {
	return strings.Contains(s, "{{array.")
}

// arrayAdjustment represents a single adjustment to the array expansion.
type arrayAdjustment struct {
	with map[string]string
	skip bool
}

// parseArrayAdjustments parses the adjustments field from a step.
func parseArrayAdjustments(v any) ([]*arrayAdjustment, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf(
			"adjustments must be an array, got %T", v,
		)
	}

	seen := make(map[string]struct{})
	var result []*arrayAdjustment
	for i, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf(
				"adjustments[%d] must be a map, got %T", i, item,
			)
		}

		adj, err := parseAdjustment(m)
		if err != nil {
			return nil, fmt.Errorf("adjustments[%d]: %w", i, err)
		}

		key := marshalStringMap(adj.with)
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf(
				"adjustments[%d]: duplicate \"with\" %v",
				i, adj.with,
			)
		}
		seen[key] = struct{}{}

		result = append(result, adj)
	}
	return result, nil
}

// marshalStringMap returns a deterministic string for dedup of string maps.
func marshalStringMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, ",")
}

func parseAdjustment(m map[string]any) (*arrayAdjustment, error) {
	withVal, ok := m["with"]
	if !ok {
		return nil, fmt.Errorf("missing required \"with\" key")
	}
	withMap, ok := withVal.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("\"with\" must be a map, got %T", withVal)
	}
	if len(withMap) == 0 {
		return nil, fmt.Errorf("\"with\" cannot be empty")
	}
	with, err := toStringMap(withMap)
	if err != nil {
		return nil, fmt.Errorf("with: %w", err)
	}

	skip, _ := m["skip"].(bool)

	for k := range m {
		if k != "with" && k != "skip" {
			return nil, fmt.Errorf("unknown key %q", k)
		}
	}

	return &arrayAdjustment{with: with, skip: skip}, nil
}
