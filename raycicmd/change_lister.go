package raycicmd

import (
	"fmt"
	"os/exec"
	"strings"
)

// ChangeLister lists changed files between two directory states.
type ChangeLister interface {
	ListChangedFiles() ([]string, error)
}

// GitChangeLister lists files changed by finding the merge-base (common ancestor)
// between the base branch and the commit, then diffing against that. This
// correctly shows only changes in a style similar to GitHub PR diffs, excluding
// changes that happened on the base branch after the PR branch was created.
type GitChangeLister struct {
	// WorkDir is the directory to run git commands in. If empty, uses current
	// directory.
	WorkDir string

	// Remote is the name of the git remote. If empty, defaults to "origin".
	Remote string

	// BaseBranch is the base branch to diff against (e.g., "main" or "master").
	BaseBranch string

	// Commit is the commit to diff from the base branch.
	Commit string
}

func (g *GitChangeLister) remote() string {
	if g.Remote != "" {
		return g.Remote
	}
	return "origin"
}

func (g *GitChangeLister) run(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	if g.WorkDir != "" {
		cmd.Dir = g.WorkDir
	}
	return cmd.Output()
}

func (g *GitChangeLister) runNoOutput(args ...string) error {
	cmd := exec.Command("git", args...)
	if g.WorkDir != "" {
		cmd.Dir = g.WorkDir
	}
	return cmd.Run()
}

// ListChangedFiles returns files changed between BaseBranch and Commit.
//
// It uses the merge-base of (remote/BaseBranch, Commit) so it only includes
// changes introduced on the feature branch, matching GitHub PR diffs and
// excluding new commits on the base branch.
func (g *GitChangeLister) ListChangedFiles() ([]string, error) {
	remote := g.remote()

	// Ensure we have the latest base branch refs.
	if err := g.runNoOutput("fetch", "-q", remote, g.BaseBranch); err != nil {
		return nil, fmt.Errorf("git fetch %s %s: %w", remote, g.BaseBranch, err)
	}

	// Find the merge-base (common ancestor) between the base branch and the commit.
	// This is the correct way to compute PR diffs - it shows only the changes
	// introduced by the PR, not changes that happened on the base branch.
	//
	//   feature-branch:              [A] --------> [B]  (Commit)
	//                                 |            + feature.go
	//                                 |
	//   remote/base:      ... ------> [A] --------> [C]
	//                                              + other.go
	//
	//   merge-base(remote/base, B) = A
	//   diff(A, B) = [feature.go]           <-- correct
	//   diff(C, B) = [feature.go, other.go] <-- wrong
	remoteBranch := remote + "/" + g.BaseBranch
	mergeBaseOut, err := g.run("merge-base", remoteBranch, g.Commit)
	if err != nil {
		return nil, fmt.Errorf("git merge-base %s %s: %w", remoteBranch, g.Commit, err)
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOut))

	// Diff only the PR changes: merge-base..Commit.
	diffRange := mergeBase + ".." + g.Commit
	diffOut, err := g.run("diff", "--name-only", diffRange, "--")
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", diffRange, err)
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(diffOut)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}
