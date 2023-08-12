package raycicmd

import (
	"fmt"
	"os"
	"path/filepath"
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

// builtin builder command to build a forge container image.
const forgeBuilderCommand = `/bin/bash -euo pipefail -c ` +
	`'export DOCKER_BUILDKIT=1 ; ` +
	`DEST_IMG="$${RAYCI_TMP_REPO}:$${RAYCI_BUILD_ID}-$${RAYCI_FORGE_NAME}" ; ` +
	`tar --mtime="UTC 2020-01-01" -c -f - "$${RAYCI_FORGE_DOCKERFILE}" |` +
	` docker build --progress=plain -t "$${DEST_IMG}" ` +
	` -f "$${RAYCI_FORGE_DOCKERFILE}" - ; ` +
	`docker push "$${DEST_IMG}" '`

const rawGitHubURL = "https://raw.githubusercontent.com/"
const runWandaURL = rawGitHubURL + "ray-project/rayci/master/run_wanda.sh"

var wandaCommands = []string{
	fmt.Sprintf(`curl -sfL "%s" > /tmp/run_wanda.sh`, runWandaURL),
	`RAYCI_BRANCH=lonnie-x /bin/bash /tmp/run_wanda.sh -rayci`,
}

func builderAgent(config *config) string {
	if config.BuilderQueues != nil {
		if q, ok := config.BuilderQueues["builder"]; ok {
			return q
		}
	}
	return ""
}

func makeWandaStep(
	buildID, name, file string, deps []string, config *config,
) map[string]any {
	agent := builderAgent(config)

	bkStep := map[string]any{
		"label":    "wanda: " + name,
		"key":      name,
		"commands": wandaCommands,
		"env": map[string]string{
			"RAYCI_BUILD_ID":   buildID,
			"RAYCI_TMP_REPO":   config.CITempRepo,
			"RAYCI_WANDA_NAME": name,
			"RAYCI_WANDA_FILE": file,
		},
	}

	if len(deps) != 0 {
		bkStep["depends_on"] = deps
	}
	if agent != "" {
		bkStep["agents"] = newBkAgents(agent)
	}
	if config.BuilderPriority != 0 {
		bkStep["priority"] = config.BuilderPriority
	}
	return bkStep
}

func makeForgeStep(buildID, name, file string, config *config) map[string]any {
	agent := builderAgent(config)

	bkStep := map[string]any{
		"label":    name,
		"key":      name,
		"commands": []string{forgeBuilderCommand},
		"env": map[string]string{
			"RAYCI_BUILD_ID":         buildID,
			"RAYCI_TMP_REPO":         config.CITempRepo,
			"RAYCI_FORGE_NAME":       name,
			"RAYCI_FORGE_DOCKERFILE": file,
		},
	}

	if agent != "" {
		bkStep["agents"] = newBkAgents(agent)
	}
	if config.BuilderPriority != 0 {
		bkStep["priority"] = config.BuilderPriority
	}

	return bkStep
}

func makeForgeGroup(repoDir, buildID string, config *config) (
	*bkPipelineGroup, error,
) {
	g := &bkPipelineGroup{
		Group: "forge",
		Key:   "all-forges",
	}

	// add forge container building steps
	for _, dir := range config.ForgeDirs {
		entries, err := os.ReadDir(filepath.Join(repoDir, dir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read forge dir %s: %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			forgeName, ok := forgeNameFromDockerfile(name)
			if !ok {
				continue
			}

			filePath := filepath.Join(dir, name)
			step := makeForgeStep(buildID, forgeName, filePath, config)
			g.Steps = append(g.Steps, step)
		}
	}

	return g, nil
}
