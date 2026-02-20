package raycicmd

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ChangeLister lists changed files and computes diff stats.
type ChangeLister interface {
	ListChangedFiles() ([]string, error)
	CountChangedLines(ignore []string) (*diffStats, error)
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

	fetched bool
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

// fetch ensures the base branch refs are up to date. It only runs once
// per lister instance.
func (g *GitChangeLister) fetch() error {
	if g.fetched {
		return nil
	}
	remote := g.remote()
	if err := g.runNoOutput("fetch", "-q", remote, g.BaseBranch); err != nil {
		return fmt.Errorf("git fetch %s %s: %w", remote, g.BaseBranch, err)
	}
	g.fetched = true
	return nil
}

// diffRange returns the merge-base..Commit range string used for diffs.
func (g *GitChangeLister) diffRange() (string, error) {
	if err := g.fetch(); err != nil {
		return "", err
	}

	// Find the merge-base (common ancestor) between the base branch and the
	// commit. This is the correct way to compute PR diffs — it shows only
	// changes introduced by the PR, not changes on the base branch.
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
	remoteBranch := g.remote() + "/" + g.BaseBranch
	mergeBaseOut, err := g.run("merge-base", remoteBranch, g.Commit)
	if err != nil {
		return "", fmt.Errorf("git merge-base %s %s: %w", remoteBranch, g.Commit, err)
	}
	return strings.TrimSpace(string(mergeBaseOut)) + ".." + g.Commit, nil
}

// ListChangedFiles returns files changed between BaseBranch and Commit.
//
// It uses the merge-base of (remote/BaseBranch, Commit) so it only includes
// changes introduced on the feature branch, matching GitHub PR diffs and
// excluding new commits on the base branch.
func (g *GitChangeLister) ListChangedFiles() ([]string, error) {
	dr, err := g.diffRange()
	if err != nil {
		return nil, err
	}

	diffOut, err := g.run("diff", "--name-only", dr, "--")
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", dr, err)
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

type diffStats struct {
	Added   int
	Deleted int
}

// CountChangedLines returns the number of added and deleted lines between
// BaseBranch and Commit, excluding files under any of the given directory
// prefixes.
func (g *GitChangeLister) CountChangedLines(ignore []string) (*diffStats, error) {
	dr, err := g.diffRange()
	if err != nil {
		return nil, err
	}

	diffOut, err := g.run("diff", "--numstat", dr, "--")
	if err != nil {
		return nil, fmt.Errorf("git diff --numstat %s: %w", dr, err)
	}

	stats := new(diffStats)
	for _, line := range strings.Split(string(diffOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		// Binary files show as "-\t-\tfilename".
		if parts[0] == "-" || parts[1] == "-" {
			continue
		}

		filename := parts[2]
		skip := false
		for _, prefix := range ignore {
			if strings.HasPrefix(filename, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		added, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		deleted, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		stats.Added += added
		stats.Deleted += deleted
	}

	return stats, nil
}
