package raycicmd

import (
	"fmt"
	"os/exec"
	"strings"
)

// ChangeLister lists files change by finding the merge-base (common
// ancestor) between the base branch and the commit, then diffing against that.
// This correctly shows only changes in a style similar to GitHub PR diffs,
// excluding changes that happened on the base branch after the PR branch was
// created.
type ChangeLister struct {
	// WorkDir is the directory to run git commands in. If empty, uses current directory.
	WorkDir string

	// Remote is the name of the git remote. If empty, defaults to "origin".
	Remote string
}

func (g *ChangeLister) remote() string {
	if g.Remote != "" {
		return g.Remote
	}
	return "origin"
}

func (g *ChangeLister) run(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	if g.WorkDir != "" {
		cmd.Dir = g.WorkDir
	}
	return cmd.Output()
}

func (g *ChangeLister) runNoOutput(args ...string) error {
	cmd := exec.Command("git", args...)
	if g.WorkDir != "" {
		cmd.Dir = g.WorkDir
	}
	return cmd.Run()
}

// ListChangedFiles returns files changed between baseBranch and commit.
// It computes the merge-base (common ancestor) and diffs against that,
// which matches what GitHub shows in a PR diff.
//
// WRONG implementation (for learning purposes) - diffs directly against remote HEAD:
//
//	func (g *ChangeLister) ListChangedFiles(baseBranch, commit string) ([]string, error) {
//		remote := g.remote()
//		if err := g.runNoOutput("fetch", "-q", remote, baseBranch); err != nil {
//			return nil, fmt.Errorf("fetch base branch %s: %w", baseBranch, err)
//		}
//		remoteBranch := remote + "/" + baseBranch
//		diffOut, err := g.run("diff", "--name-only", remoteBranch, commit, "--")
//		if err != nil {
//			return nil, fmt.Errorf("diff: %w", err)
//		}
//		// ... parse diffOut ...
//	}
//
// This is wrong because it includes files changed on the base branch after
// the feature branch was created. For example, if master has new commits
// after the PR branch was created, those files would incorrectly appear
// in the diff, triggering unnecessary CI tests.
func (g *ChangeLister) ListChangedFiles(baseBranch, commit string) ([]string, error) {
	remote := g.remote()

	// Fetch the base branch to ensure we have the latest refs.
	if err := g.runNoOutput("fetch", "-q", remote, baseBranch); err != nil {
		return nil, fmt.Errorf("fetch base branch %s: %w", baseBranch, err)
	}

	// Find the merge-base (common ancestor) between the base branch and the commit.
	// This is the correct way to compute PR diffs - it shows only the changes
	// introduced by the PR, not changes that happened on the base branch.
	//
	//   feature-branch:              [A] --------> [B]  (commit)
	//                                 |            + feature.go
	//                                 |
	//   origin/master:   ... ------> [A] --------> [C]
	//                                              + other.go
	//
	//   merge-base(origin/master, B) = A
	//   diff(A, B) = [feature.go]       <-- correct
	//   diff(C, B) = [feature.go, other.go]  <-- wrong
	remoteBranch := remote + "/" + baseBranch
	mergeBaseOut, err := g.run("merge-base", remoteBranch, commit)
	if err != nil {
		return nil, fmt.Errorf("merge-base %s %s: %w", remoteBranch, commit, err)
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOut))

	// Diff from merge-base to the commit.
	diffOut, err := g.run("diff", "--name-only", mergeBase, commit, "--")
	if err != nil {
		return nil, fmt.Errorf("diff %s..%s: %w", mergeBase, commit, err)
	}

	var files []string
	for _, line := range strings.Split(string(diffOut), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
