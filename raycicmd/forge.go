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
	`DEST_IMG="$${RAYCI_WORK_REPO}:$${RAYCI_BUILD_ID}-$${RAYCI_FORGE_NAME}" ; ` +
	`tar --mtime="UTC 2020-01-01" -c -f - "$${RAYCI_FORGE_DOCKERFILE}" |` +
	` docker build --progress=plain -t "$${DEST_IMG}" ` +
	` -f "$${RAYCI_FORGE_DOCKERFILE}" - ; ` +
	`docker push "$${DEST_IMG}" '`

type forgeStep struct {
	name    string
	file    string
	buildID string

	envs     map[string]string
	ciConfig *config
}

func (s *forgeStep) buildkiteStep() map[string]any {
	agent := builderAgent(s.ciConfig)

	envs := make(map[string]string)
	for k, v := range s.envs {
		envs[k] = v
	}
	envs["RAYCI_FORGE_NAME"] = s.name
	envs["RAYCI_FORGE_DOCKERFILE"] = s.file

	bkStep := map[string]any{
		"label":    s.name,
		"key":      s.name,
		"commands": []string{forgeBuilderCommand},
		"env":      envs,
	}

	if agent != "" {
		bkStep["agents"] = newBkAgents(agent)
	}
	if p := s.ciConfig.BuilderPriority; p != 0 {
		bkStep["priority"] = p
	}

	return bkStep
}

func makeForgeGroup(
	repoDir, buildID string, config *config, envs map[string]string,
) (
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
			step := &forgeStep{
				name:     forgeName,
				file:     filePath,
				buildID:  buildID,
				envs:     envs,
				ciConfig: config,
			}
			g.Steps = append(g.Steps, step.buildkiteStep())
		}
	}

	return g, nil
}
