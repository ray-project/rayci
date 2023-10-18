package raycicmd

import (
	"fmt"
)

func boolInMap(m map[string]any, key string) (bool, bool) {
	v, ok := m[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func stringInMap(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func stringInMapAnyKey(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if s, ok := stringInMap(m, k); ok {
			return s, true
		}
	}
	return "", false
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	res := make(map[string]any)
	for k, v := range m {
		res[k] = v
	}
	return res
}

func cloneMapExcept(m map[string]any, except []string) map[string]any {
	if m == nil {
		return nil
	}
	exceptMap := make(map[string]bool, len(except))
	for _, k := range except {
		exceptMap[k] = true
	}

	res := make(map[string]any)
	for k, v := range m {
		if !exceptMap[k] {
			res[k] = v
		}
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

func checkStepKeys(m map[string]any, allowed []string) error {
	allowedMap := make(map[string]bool, len(allowed))
	for _, k := range allowed {
		allowedMap[k] = true
	}
	for k := range m {
		if !allowedMap[k] {
			return fmt.Errorf("unsupported step key %q", k)
		}
	}
	return nil
}

func toStringList(v any) []string {
	switch v := v.(type) {
	case nil:
		return nil
	case []string:
		var list []string
		list = append(list, v...)
		return list
	case []any:
		var list []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				list = append(list, str)
			}
		}
		return list
	case string:
		return []string{v}
	default:
		return nil
	}
}
