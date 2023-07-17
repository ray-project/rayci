package raycicmd

import (
	"strings"
)

// builtin builder command to build a forge container image.
const forgeBuilderCommand = `/bin/bash -euo pipefail -c ` +
	`'export DOCKER_BUILDKIT=1 ; ` +
	`DEST_IMG="$${RAYCI_TMP_REPO}:$${RAYCI_BUILD_ID}-$${RAYCI_FORGE_NAME}" ; ` +
	`tar --mtime="UTC 2020-01-01" -c -f - "$${RAYCI_FORGE_DOCKERFILE}" |` +
	` docker build --progress=plain -t "$${DEST_IMG}" ` +
	` -f "$${RAYCI_FORGE_DOCKERFILE}" - ; ` +
	`docker push "$${DEST_IMG}" '`

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

func makeForgeStep(buildID, name, file string, config *config) map[string]any {
	agent := ""
	if config.BuilderQueues != nil {
		if q, ok := config.BuilderQueues["builder"]; ok {
			agent = q
		}
	}

	bkStep := map[string]any{
		"label":    name,
		"key":      name,
		"commands": []string{forgeBuilderCommand},
		"env": map[string]string{
			"RAYCI_BUILD_ID":         buildID,
			"RAYCI_TMP_REPO":         config.CITempRepo,
			"RAYCI_FORGE_DOCKERFILE": file,
			"RAYCI_FORGE_NAME":       name,
		},
	}
	if agent != "" {
		bkStep["agents"] = newBkAgents(agent)
	}

	return bkStep
}
