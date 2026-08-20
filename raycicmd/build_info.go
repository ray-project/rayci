package raycicmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os/exec"

	"github.com/ray-project/rayci/wanda"
)

type buildInfo struct {
	buildID          string
	buildAuthorEmail string
	launcherBranch   string
	gitCommit        string
	selects          []string

	// cacheEpoch is resolved once here and handed to every wanda step, so
	// that a build straddling an epoch boundary does not have its steps
	// disagreeing about the cache key.
	cacheEpoch string
}

// makeCacheEpoch resolves the wanda cache epoch for a build. It calls into
// wanda rather than reimplementing the rule, so the pipeline and the builders
// can never drift apart on it.
func makeCacheEpoch(envs Envs) string {
	if v, ok := envs.Lookup("RAYCI_CACHE_EPOCH"); ok && v != "" {
		return v
	}
	return wanda.DefaultCacheEpoch()
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
