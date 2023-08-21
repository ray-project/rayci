package raycicmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os/exec"
)

type buildInfo struct {
	BuildID     string
	RayCIBranch string
	GitCommit   string
}

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

func gitCommit(envs Envs) string {
	commit := getEnv(envs, "BUILDKITE_COMMIT")
	if commit == "HEAD" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		bs, err := cmd.Output()
		if err != nil {
			log.Printf("Fail to resolve HEAD commit: %v", err)
			commit = ""
		} else {
			commit = string(bytes.TrimSpace(bs))
		}
	}
	return commit
}
