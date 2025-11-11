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
	buildID          string
	buildAuthorEmail string
	launcherBranch   string
	gitCommit        string
	selects          []string
}

func makeBuildID(envs Envs, debug bool) (string, error) {
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

	// In debug mode, generate a dummy build ID
	if debug {
		return "debug000", nil
	}

	return "", fmt.Errorf("no build id found")
}

func gitCommit(envs Envs, debug bool) string {
	commit := getEnv(envs, "BUILDKITE_COMMIT")
	if commit == "" || commit == "HEAD" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		bs, err := cmd.Output()
		if err != nil {
			if debug {
				// In debug mode, use a dummy commit hash if git fails
				log.Printf("Using dummy commit hash in debug mode (git command failed: %v)", err)
				return "0000000000000000000000000000000000000000"
			}
			log.Printf("Fail to resolve HEAD commit: %v", err)
			commit = ""
		} else {
			commit = string(bytes.TrimSpace(bs))
		}
	}
	return commit
}
