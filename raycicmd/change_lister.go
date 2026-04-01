package raycicmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var (
	gitRefRe     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)
	commitHashRe = regexp.MustCompile(`^[0-9a-fA-F]{4,40}$`)
)

// ChangeLister lists changed files between two directory states.
type ChangeLister interface {
	ListChangedFiles() ([]string, error)
}

// gitChangeLister lists files changed by finding the merge-base (common ancestor)
// between the base branch and the commit, then diffing against that. This
// correctly shows only changes in a style similar to GitHub PR diffs, excluding
// changes that happened on the base branch after the PR branch was created.
type gitChangeLister struct {
	// WorkDir is the directory to run git commands in. If empty, uses current
	// directory.
	workDir string

	// Remote is the name of the git remote. If empty, defaults to "origin".
	remote string

	// BaseBranch is the base branch to diff against (e.g., "main" or "master").
	baseBranch string

	// Commit is the commit to diff from the base branch.
	commit string
}

func newGitChangeLister(
	workDir, remote, baseBranch, commit string,
) (*gitChangeLister, error) {
	if remote == "" {
		remote = "origin"
	}
	if !gitRefRe.MatchString(remote) {
		return nil, fmt.Errorf("invalid remote name: %q", remote)
	}
	if !gitRefRe.MatchString(baseBranch) {
		return nil, fmt.Errorf("invalid base branch name: %q", baseBranch)
	}
	if !commitHashRe.MatchString(commit) {
		return nil, fmt.Errorf("invalid commit hash: %q", commit)
	}
	return &gitChangeLister{
		workDir:    workDir,
		remote:     remote,
		baseBranch: baseBranch,
		commit:     commit,
	}, nil
}

func (g *gitChangeLister) run(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	if g.workDir != "" {
		cmd.Dir = g.workDir
	}
	return cmd.Output()
}

func (g *gitChangeLister) runNoOutput(args ...string) error {
	cmd := exec.Command("git", args...)
	if g.workDir != "" {
		cmd.Dir = g.workDir
	}
	return cmd.Run()
}

// ListChangedFiles returns files changed between BaseBranch and Commit.
//
// It uses the merge-base of (remote/BaseBranch, Commit) so it only includes
// changes introduced on the feature branch, matching GitHub PR diffs and
// excluding new commits on the base branch.
func (g *gitChangeLister) ListChangedFiles() ([]string, error) {
	remote := g.remote

	// Fetch the base branch with a refspec so the remote-tracking ref exists
	// for merge-base. Plain `git fetch origin <branch>` only updates FETCH_HEAD.
	remoteBranchRef := fmt.Sprintf("refs/remotes/%s/%s", remote, g.baseBranch)
	refspec := fmt.Sprintf("+%s:%s", g.baseBranch, remoteBranchRef)
	isShallowOut, err := g.run("rev-parse", "--is-shallow-repository")
	if err != nil {
		return nil, fmt.Errorf("git rev-parse --is-shallow-repository: %w", err)
	}
	if strings.TrimSpace(string(isShallowOut)) == "true" {
		if err := g.runNoOutput("fetch", "--unshallow", "-q", remote, refspec); err != nil {
			return nil, fmt.Errorf("git fetch --unshallow %s %s: %w", remote, refspec, err)
		}
	} else {
		if err := g.runNoOutput("fetch", "-q", remote, refspec); err != nil {
			return nil, fmt.Errorf("git fetch %s %s: %w", remote, refspec, err)
		}
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
	remoteBranch := remote + "/" + g.baseBranch
	mergeBaseOut, err := g.run("merge-base", remoteBranch, g.commit)
	if err != nil {
		return nil, fmt.Errorf("git merge-base %s %s: %w", remoteBranch, g.commit, err)
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOut))

	// Diff only the PR changes: merge-base..Commit.
	diffRange := mergeBase + ".." + g.commit
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
