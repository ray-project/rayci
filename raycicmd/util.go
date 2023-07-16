package raycicmd

import (
	"fmt"
)

func stringInMap(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
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
