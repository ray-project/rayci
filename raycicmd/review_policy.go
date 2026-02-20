package raycicmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

const reviewPolicyFile = "review-policy.yaml"

type reviewPolicy struct {
	Review *reviewConfig `yaml:"review"`
}

type reviewConfig struct {
	MaxAdditions int      `yaml:"max_additions"`
	MaxDeletions int      `yaml:"max_deletions"`
	Ignore       []string `yaml:"ignore"`
}

// loadReviewPolicy reads a review-policy.yaml file from the given
// directory and returns the parsed policy. Returns nil if the file
// does not exist.
func loadReviewPolicy(dir string) (*reviewPolicy, error) {
	p := filepath.Join(dir, reviewPolicyFile)
	bs, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read review policy: %w", err)
	}

	policy := new(reviewPolicy)
	if err := yaml.Unmarshal(bs, policy); err != nil {
		return nil, fmt.Errorf("unmarshal review policy: %w", err)
	}
	return policy, nil
}

// makePolicyGroup evaluates the review policy against the given change
// lister and returns a Buildkite pipeline group with a soft-failing
// step when thresholds are exceeded. Returns nil when the policy is
// nil, thresholds are zero, or the diff is within limits.
func makePolicyGroup(
	policy *reviewPolicy,
	lister ChangeLister,
	runnerQueue string,
) *bkPipelineGroup {
	if policy == nil || policy.Review == nil {
		return nil
	}
	cfg := policy.Review
	if cfg.MaxAdditions <= 0 && cfg.MaxDeletions <= 0 {
		return nil
	}

	stats, err := lister.CountChangedLines(cfg.Ignore)
	if err != nil {
		log.Printf("policy: count changed lines: %v", err)
		return nil
	}

	var exceeded []string
	if cfg.MaxAdditions > 0 && stats.Added > cfg.MaxAdditions {
		exceeded = append(exceeded, fmt.Sprintf(
			"additions (%d) exceed threshold (%d)", stats.Added, cfg.MaxAdditions,
		))
	}
	if cfg.MaxDeletions > 0 && stats.Deleted > cfg.MaxDeletions {
		exceeded = append(exceeded, fmt.Sprintf(
			"deletions (%d) exceed threshold (%d)", stats.Deleted, cfg.MaxDeletions,
		))
	}

	if len(exceeded) == 0 {
		return nil
	}

	var commands []string
	commands = append(commands, fmt.Sprintf(
		"echo 'PR diff stats: +%d -%d lines'", stats.Added, stats.Deleted,
	))
	for _, msg := range exceeded {
		commands = append(commands, fmt.Sprintf("echo 'WARNING: %s'", msg))
	}

	var annotationLines []string
	annotationLines = append(annotationLines,
		fmt.Sprintf("PR diff stats: **+%d** / **-%d** lines", stats.Added, stats.Deleted),
	)
	for _, msg := range exceeded {
		annotationLines = append(annotationLines, "- "+msg)
	}
	annotation := strings.Join(annotationLines, "\n")
	commands = append(commands, fmt.Sprintf(
		"buildkite-agent annotate %q --style warning --context review-size",
		annotation,
	))
	commands = append(commands, "exit 1")

	step := map[string]any{
		"label":     ":bar_chart: Policy",
		"key":       "policy-review-size",
		"command":   commands,
		"soft_fail": true,
	}

	if runnerQueue != "" {
		step["agents"] = newBkAgents(runnerQueue)
	}

	return &bkPipelineGroup{
		Group: "Policy",
		Steps: []any{step},
	}
}
