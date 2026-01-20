package wanda

import (
	"fmt"
	"os"
	"strings"
)

// stripComment removes comments from an envfile line.
// For KEY=value lines, only strips comments before '=' to preserve '#' in values.
func stripComment(line string) string {
	if idx := strings.Index(line, "="); idx >= 0 {
		key := line[:idx]
		value := line[idx+1:]
		if cidx := strings.Index(key, "#"); cidx >= 0 {
			key = key[:cidx]
		}
		return strings.TrimSpace(key) + "=" + value
	}

	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

// ParseEnvFile parses a .env file containing KEY=value pairs.
// Skips blank lines and comment lines (starting with #).
// Hash characters (#) in values are preserved (e.g., URLs, color codes).
// Values are returned as-is without variable expansion.
func ParseEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read envfile: %w", err)
	}

	env := make(map[string]string)
	for i, line := range strings.Split(string(data), "\n") {
		lineNum := i + 1

		line = stripComment(line)
		if line == "" {
			continue
		}

		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected KEY=value", lineNum)
		}

		key := strings.TrimSpace(k)
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNum)
		}

		env[key] = strings.TrimSpace(v)
	}

	return env, nil
}
