package prcheck

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const usage = `prcheck - check PR diff size against thresholds

Fetches both branch refs, computes the merge-base, and counts
lines added/deleted. Exits non-zero if thresholds are exceeded.

Usage:
  prcheck -config <path> -base-ref <branch> -head-ref <branch>

Flags:
  -config     Path to prcheck-policy.yaml config file.
  -base-ref   Base branch name (e.g. main).
  -head-ref   Head branch name (e.g. feature-branch).

Exit codes:
  0  No thresholds defined, or within thresholds.
  1  Config not found, or thresholds exceeded.
`

// Main is the entry point for the prcheck CLI.
func Main(args []string) (int, error) {
	set := flag.NewFlagSet("prcheck", flag.ContinueOnError)
	set.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	var configPath string
	var baseRef string
	var headRef string

	set.StringVar(&configPath, "config", "", "path to size policy config")
	set.StringVar(&baseRef, "base-ref", "", "base branch ref (e.g. main)")
	set.StringVar(&headRef, "head-ref", "", "head branch ref (e.g. feature-branch)")

	if err := set.Parse(args[1:]); err != nil {
		return 1, err
	}

	if configPath == "" || baseRef == "" || headRef == "" {
		fmt.Fprint(os.Stderr, usage)
		return 1, nil
	}

	g := &gitClient{}
	return run(configPath, baseRef, headRef, g)
}

func run(configPath, baseRef, headRef string, g *gitClient) (int, error) {
	policy, err := loadConfig(configPath)
	if err != nil {
		return 1, fmt.Errorf("load config: %w", err)
	}
	if policy.Size == nil {
		return 0, nil
	}

	cfg := policy.Size
	if cfg.MaxAdditions <= 0 && cfg.MaxDeletions <= 0 {
		return 0, nil
	}

	if err := g.fetchRef(baseRef); err != nil {
		return 1, fmt.Errorf("fetch base: %w", err)
	}
	if err := g.fetchRef(headRef); err != nil {
		return 1, fmt.Errorf("fetch head: %w", err)
	}
	mergeBase, err := g.mergeBase(baseRef, headRef)
	if err != nil {
		return 1, fmt.Errorf("merge base: %w", err)
	}

	output, err := g.diffNumstat(mergeBase, "origin/"+headRef)
	if err != nil {
		return 1, fmt.Errorf("diff numstat: %w", err)
	}

	stats, err := parseDiffNumstat(output, cfg.Ignore)
	if err != nil {
		return 1, fmt.Errorf("parse diff: %w", err)
	}
	statsLine := fmt.Sprintf("+%d -%d", stats.linesAdded, stats.linesDeleted)
	fmt.Println(statsLine)

	exceeded := checkSize(cfg, stats)
	if len(exceeded) == 0 {
		return 0, nil
	}

	for _, msg := range exceeded {
		fmt.Println(msg)
	}

	writeJobSummary(cfg, stats)
	writeGitHubOutput(cfg, stats)

	return 1, nil
}

// writeGitHubOutput writes key=value pairs to $GITHUB_OUTPUT so
// subsequent workflow steps can reference them.
func writeGitHubOutput(cfg *sizeConfig, stats *diffStats) {
	var sb strings.Builder
	if cfg.MaxAdditions > 0 && stats.linesAdded > cfg.MaxAdditions {
		fmt.Fprintf(&sb, "additions=%d (max allowed: %d)\n", stats.linesAdded, cfg.MaxAdditions)
	}
	if cfg.MaxDeletions > 0 && stats.linesDeleted > cfg.MaxDeletions {
		fmt.Fprintf(&sb, "deletions=%d (max allowed: %d)\n", stats.linesDeleted, cfg.MaxDeletions)
	}
	appendGitHubFile("GITHUB_OUTPUT", sb.String())
}

// writeJobSummary appends a markdown summary to $GITHUB_STEP_SUMMARY
// when running inside GitHub Actions.
func writeJobSummary(cfg *sizeConfig, stats *diffStats) {
	var sb strings.Builder
	sb.WriteString("### PR Size Warning\n\n")
	if cfg.MaxAdditions > 0 {
		fmt.Fprintf(&sb, "- additions: max %d, changed %d\n", cfg.MaxAdditions, stats.linesAdded)
	}
	if cfg.MaxDeletions > 0 {
		fmt.Fprintf(&sb, "- deletions: max %d, changed %d\n", cfg.MaxDeletions, stats.linesDeleted)
	}
	appendGitHubFile("GITHUB_STEP_SUMMARY", sb.String())
}

func appendGitHubFile(envVar, content string) {
	path := os.Getenv(envVar)
	if path == "" || content == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(content)
}
