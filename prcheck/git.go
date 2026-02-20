package prcheck

import (
	"fmt"
	"os/exec"
	"strings"
)

type gitClient struct {
	workDir string
}

func (g *gitClient) command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workDir
	return cmd
}

// fetchRef fetches a branch ref from origin into a remote-tracking branch.
func (g *gitClient) fetchRef(ref string) error {
	refspec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", ref, ref)
	return g.command("fetch", "origin", refspec).Run()
}

// mergeBase returns the best common ancestor of two remote-tracking branches.
func (g *gitClient) mergeBase(base, head string) (string, error) {
	out, err := g.command("merge-base", "origin/"+base, "origin/"+head).Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// diffNumstat returns the raw --numstat output between two commits.
func (g *gitClient) diffNumstat(mergeBase, head string) ([]byte, error) {
	diffRange := mergeBase + ".." + head
	out, err := g.command(
		"diff", "--numstat", "--no-renames", diffRange, "--",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --numstat: %w", err)
	}
	return out, nil
}
