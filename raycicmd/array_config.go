package raycicmd

import (
	"fmt"
	"maps"
	"sort"
	"strings"
)

// arrayConfig represents a parsed array definition.
type arrayConfig struct {
	dims map[string][]string // dimension name -> values
}

// arrayInstance is one combination from expand().
type arrayInstance struct {
	values map[string]string // dimension name -> selected value
}

// parseArrayConfig parses the array field from a step.
//
//	array:
//	  python: ["3.10", "3.11"]
//	  cuda: ["12.1.1", "12.8.1"]
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

func (cfg *arrayConfig) sortedDimensions() []string {
	dims := make([]string, 0, len(cfg.dims))
	for dim := range cfg.dims {
		dims = append(dims, dim)
	}
	sort.Strings(dims)
	return dims
}

// expand returns the cartesian product of all dimensions.
func (cfg *arrayConfig) expand() []*arrayInstance {
	if len(cfg.dims) == 0 {
		return nil
	}

	result := []*arrayInstance{{values: make(map[string]string)}}
	for _, dim := range cfg.sortedDimensions() {
		var expanded []*arrayInstance
		for _, inst := range result {
			for _, val := range cfg.dims[dim] {
				newInst := &arrayInstance{values: maps.Clone(inst.values)}
				newInst.values[dim] = val
				expanded = append(expanded, newInst)
			}
		}
		result = expanded
	}
	return result
}

// generateKey returns {base}--{dim1}{val1}-{dim2}{val2} (dims sorted).
// Double-dash separates base key from array dimensions.
func (inst *arrayInstance) generateKey(baseKey string) string {
	dims := make([]string, 0, len(inst.values))
	for dim := range inst.values {
		dims = append(dims, dim)
	}
	sort.Strings(dims)

	var parts []string
	for _, dim := range dims {
		parts = append(parts, sanitizeKeyPart(dim+inst.values[dim]))
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
func (inst *arrayInstance) substituteValues(v any) any {
	switch val := v.(type) {
	case string: // label: "Build {{array.python}}"
		return inst.substituteString(val)
	case map[string]any: // env: {PYTHON: "{{array.python}}"}
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = inst.substituteValues(v)
		}
		return result
	case []any: // commands: ["echo {{array.python}}"]
		result := make([]any, len(val))
		for i, v := range val {
			result[i] = inst.substituteValues(v)
		}
		return result
	default:
		return v
	}
}

func (inst *arrayInstance) substituteString(s string) string {
	if !hasArrayPlaceholder(s) {
		return s
	}
	for dim, val := range inst.values {
		s = strings.ReplaceAll(s, "{{array."+dim+"}}", val)
	}
	return s
}

func hasArrayPlaceholder(s string) bool {
	return strings.Contains(s, "{{array.")
}
