package raycicmd

import (
	"strings"
)

func forgeNameFromDockerfile(name string) (string, bool) {
	const prefix = "Dockerfile."

	if !strings.HasPrefix(name, prefix) {
		return "", false
	}
	name = strings.TrimPrefix(name, prefix)
	if name == "" {
		return "", false
	}
	return name, true
}
