package raycicmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func makeBuildID(envs Envs) (string, error) {
	buildID := getEnv(envs, "RAYCI_BUILD_ID")
	if buildID != "" {
		return buildID, nil
	}

	buildID = getEnv(envs, "BUILDKITE_BUILD_ID")
	if buildID != "" {
		h := sha256.Sum256([]byte(buildID))
		prefix := hex.EncodeToString(h[:])[:8]
		return prefix, nil
	}

	return "", fmt.Errorf("no build id found")
}
