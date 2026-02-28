package raycilint

import (
	"flag"
	"fmt"
	"os"
)

var prsizeUsage = `rayci-lint prsize - check PR diff size against thresholds

Fetches both branch refs, computes the merge-base, and counts
lines added/deleted. Exits non-zero if thresholds are exceeded.

Usage:
  rayci-lint prsize -base-ref <branch> -head-ref <branch> [-config-value key=value]

Flags:
  -base-ref       Base branch name (e.g. main).
  -head-ref       Head branch name (e.g. feature-branch).
  -config-value   Override a config value. Supported keys:
` + overrideKeysHelp(prsizeConfig{}) + `

Exit codes:
  0  No thresholds defined, or within thresholds.
  1  Config not found, or thresholds exceeded.
`

func cmdPrsize(cfg *config, args []string) error {
	set := flag.NewFlagSet(
		"rayci-lint prsize", flag.ContinueOnError,
	)
	set.Usage = func() { fmt.Fprint(os.Stderr, prsizeUsage) }

	var configOverrides multiFlag

	baseRef := set.String("base-ref", "", "base branch ref (e.g. main)")
	headRef := set.String("head-ref", "", "head branch ref (e.g. feature-branch)")
	set.Var(&configOverrides, "config-value", "override a config value")

	if err := set.Parse(args); err != nil {
		return err
	}

	if *baseRef == "" || *headRef == "" {
		set.Usage()
		return fmt.Errorf(
			"both -base-ref and -head-ref are required",
		)
	}

	if err := applyOverrides(cfg.Prsize, configOverrides); err != nil {
		return err
	}

	g := &gitClient{}
	return runPrsize(cfg, *baseRef, *headRef, g)
}

func runPrsize(
	cfg *config,
	baseRef, headRef string,
	g *gitClient,
) error {
	ps := cfg.Prsize
	if ps.MaxAdditions <= 0 && ps.MaxDeletions <= 0 {
		return nil
	}

	if err := g.fetchRef(baseRef); err != nil {
		return fmt.Errorf("fetch base: %w", err)
	}
	if err := g.fetchRef(headRef); err != nil {
		return fmt.Errorf("fetch head: %w", err)
	}
	mergeBase, err := g.mergeBase(baseRef, headRef)
	if err != nil {
		return fmt.Errorf("merge base: %w", err)
	}

	output, err := g.diffNumstat(mergeBase, "origin/"+headRef)
	if err != nil {
		return fmt.Errorf("diff numstat: %w", err)
	}

	stats, err := parseDiffNumstat(output, ps.Ignore)
	if err != nil {
		return fmt.Errorf("parse diff: %w", err)
	}
	fmt.Printf("+%d -%d\n", stats.linesAdded, stats.linesDeleted)

	exceeded := checkSize(ps, stats)
	if len(exceeded) == 0 {
		return nil
	}

	for _, msg := range exceeded {
		fmt.Println(msg)
	}

	summaryPath := ps.GithubStepSummary
	if summaryPath == "" {
		summaryPath = os.Getenv("GITHUB_STEP_SUMMARY")
	}
	outputPath := ps.GithubOutput
	if outputPath == "" {
		outputPath = os.Getenv("GITHUB_OUTPUT")
	}
	writeGitHubStepSummary(summaryPath, ps, stats)
	writeGitHubOutput(outputPath, ps, stats)

	return fmt.Errorf("PR size thresholds exceeded")
}
