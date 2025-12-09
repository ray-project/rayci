package raycicmd

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitClient abstracts git operations for testability.
type GitClient interface {
	ListChangedFiles(baseBranch, commitRange string) ([]string, error)
}

// RealGitClient implements GitClient using actual git commands.
type RealGitClient struct {
	// WorkDir is the directory to run git commands in. If empty, uses current directory.
	WorkDir string
}

// ListChangedFiles returns files changed between baseBranch and the commit in commitRange.
// commitRange should be in the format "origin/branch...commit".
// If baseBranch is provided, it will be fetched first.
func (g *RealGitClient) ListChangedFiles(baseBranch, commitRange string) ([]string, error) {
	parts := strings.Split(commitRange, "...")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid commit range: %s", commitRange)
	}

	if baseBranch != "" {
		cmd := exec.Command("git", "fetch", "-q", "origin", baseBranch)
		if g.WorkDir != "" {
			cmd.Dir = g.WorkDir
		}
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("fetch base branch %s: %w", baseBranch, err)
		}
	}

	cmd := exec.Command("git", "diff", "--name-only", commitRange, "--")
	if g.WorkDir != "" {
		cmd.Dir = g.WorkDir
	}
	diffNames, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	var files []string
	for _, line := range strings.Split(string(diffNames), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
