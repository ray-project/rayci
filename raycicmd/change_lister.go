package raycicmd

import (
	"fmt"
	"os/exec"
	"strings"
)

// ChangeLister lists files changed between git commits.
type ChangeLister struct {
	// WorkDir is the directory to run git commands in. If empty, uses current directory.
	WorkDir string
}

// ListChangedFiles returns files changed between baseBranch and the commit in commitRange.
// commitRange should be in the format "origin/branch...commit".
func (g *ChangeLister) ListChangedFiles(
	baseBranch, commitRange string,
) ([]string, error) {
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
