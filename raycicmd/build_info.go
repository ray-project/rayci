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
	BuildID     string
	RayCIBranch string
	GitCommit   string
	GitDiff     []string
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

func gitDiff(envs Envs) []string {
	base_branch := "master"
	if base_branch == "" {
		return []string{}
	}
	cmd := exec.Command(
		"git", 
		"diff", 
		"--name-only", 
		fmt.Sprintf("$(git merge-base origin/%s HEAD)..HEAD)", base_branch),
	)
	diffs, err := cmd.Output()
	if err != nil {
		log.Printf("Fail to resolve git diff: %v", err)
		return []string{}
	}
	log.Printf("git diff: %s", diffs)
	return strings.Split(string(bytes.TrimSpace(diffs)), "\n")
}
