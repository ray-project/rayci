package raycicmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type buildInfo struct {
	buildID        string
	launcherBranch string
	gitCommit      string
	selects        []string
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

func parseSelect(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	fields := strings.Split(s, ",")
	var selects []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			return nil, fmt.Errorf("empty field in select")
		}
		selects = append(selects, f)
	}
	return selects, nil
}
