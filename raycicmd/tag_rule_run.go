package raycicmd

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

// mergedTagRuleConfig holds the result of merging multiple tag rule config files.
type mergedTagRuleConfig struct {
	// RuleSet contains the merged rules for matching files to tags.
	RuleSet *TagRuleSet
	// DefaultTags contains the union of all default tags across all configs.
	// These are tags from default rules (\default).
	DefaultTags []string
}

// loadAndMergeTagRuleConfigs loads and merges tag rule configurations from multiple files.
// Tag definitions and rules from all files are combined into a single TagRuleSet.
// Default tags (from \default rules) are unioned across all configs.
func loadAndMergeTagRuleConfigs(configPaths []string) (*mergedTagRuleConfig, error) {
	merged := &mergedTagRuleConfig{
		RuleSet: &TagRuleSet{
			tagDefs: make(map[string]struct{}),
		},
	}

	defaultTagSet := make(map[string]struct{})

	for _, configPath := range configPaths {
		ruleContent, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		cfg, err := ParseTagRuleConfig(string(ruleContent))
		if err != nil {
			return nil, err
		}

		for _, tagDef := range cfg.TagDefs {
			merged.RuleSet.tagDefs[tagDef] = struct{}{}
		}
		merged.RuleSet.rules = append(merged.RuleSet.rules, cfg.Rules...)
		merged.RuleSet.defaultRules = append(merged.RuleSet.defaultRules, cfg.DefaultRules...)

		// Collect default tags from default rules
		for _, rule := range cfg.DefaultRules {
			for _, tag := range rule.Tags {
				defaultTagSet[tag] = struct{}{}
			}
		}
	}

	// Convert default tag set to sorted slice
	merged.DefaultTags = make([]string, 0, len(defaultTagSet))
	for tag := range defaultTagSet {
		merged.DefaultTags = append(merged.DefaultTags, tag)
	}
	sort.Strings(merged.DefaultTags)

	if err := merged.RuleSet.ValidateRules(); err != nil {
		return nil, err
	}

	return merged, nil
}

// isPullRequest returns true if the current build is for a pull request.
// Buildkite sets BUILDKITE_PULL_REQUEST to "false" for non-PR builds,
// or to the PR number for PR builds.
func isPullRequest(env Envs) bool {
	pr := getEnv(env, "BUILDKITE_PULL_REQUEST")
	return pr != "false" && pr != ""
}

// needRunAllTags checks if all tags should be run regardless of changed files.
// Returns true with a reason string if any of these conditions are met:
//   - RAYCI_RUN_ALL_TESTS=1 is set
//   - Building on the master branch
//   - Building on a release branch (releases/*)
//   - Not a pull request build
func needRunAllTags(env Envs) (bool, string) {
	if getEnv(env, "RAYCI_RUN_ALL_TESTS") == "1" {
		return true, "RAYCI_RUN_ALL_TESTS is set"
	}

	if !isPullRequest(env) {
		return true, "not a PR build"
	}

	return false, ""
}

// tagsForChangedFiles determines which tags to run based on changed files.
// For each file, rules are evaluated in order:
//   - If a rule matches, its tags are added to the result
//   - If the rule has Fallthrough=true, continue to the next rule
//   - Otherwise, stop processing rules for this file
//
// If no non-fallthrough rule matches a file, default rules are applied.
// Fallthrough rules add tags but don't prevent default rule fallback.
func tagsForChangedFiles(ruleSet *TagRuleSet, files []string) []string {
	tagSet := make(map[string]struct{})

	for _, file := range files {
		terminatingRuleMatched := false
		for _, rule := range ruleSet.rules {
			if rule.Match(file) {
				for _, tag := range rule.Tags {
					tagSet[tag] = struct{}{}
				}
				if !rule.Fallthrough {
					terminatingRuleMatched = true
					break // stop processing rules for this file
				}
			}
		}
		// If no terminating rule matched, apply default rules from all configs
		if !terminatingRuleMatched {
			if len(ruleSet.defaultRules) > 0 {
				for _, rule := range ruleSet.defaultRules {
					for _, tag := range rule.Tags {
						tagSet[tag] = struct{}{}
					}
				}
			} else {
				log.Printf("unhandled file (no matching rule): %s", file)
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// RunTagAnalysis determines which test tags to run based on changed files.
//
// For PR builds, it analyzes changed files against tag rules and returns
// only the relevant tags. For non-PR builds (master, release branches, etc.),
// it returns ["*"] to run all tags.
//
// The function requires BUILDKITE=true and, for PR builds, also requires
// BUILDKITE_PULL_REQUEST_BASE_BRANCH and BUILDKITE_COMMIT to be set.
func RunTagAnalysis(
	configPaths []string,
	env Envs,
	lister ChangeLister,
) ([]string, error) {
	if getEnv(env, "BUILDKITE") != "true" {
		return nil, fmt.Errorf("BUILDKITE environment variable is not set")
	}

	runAll, reason := needRunAllTags(env)
	if runAll {
		log.Printf("running all tags: %s", reason)
		return []string{"*"}, nil
	}

	// If no config files exist, run all tags. This matches the original
	// Python behavior when ci/pipeline/test_conditional_testing.py was absent.
	// See: https://github.com/ray-project/rayci/blob/23e47c9b5502a3646f506cf5362c6d3507952bce/raycicmd/step_filter.go#L108-L109
	hasConfigFile := false
	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			hasConfigFile = true
			break
		}
		log.Printf("config file not found: %s", configPath)
	}
	if !hasConfigFile {
		log.Printf("no config files found, running all tags")
		return []string{"*"}, nil
	}

	baseBranch := getEnv(env, "BUILDKITE_PULL_REQUEST_BASE_BRANCH")
	commit := getEnv(env, "BUILDKITE_COMMIT")
	if baseBranch == "" || commit == "" {
		return nil, fmt.Errorf(
			"BUILDKITE_PULL_REQUEST_BASE_BRANCH and BUILDKITE_COMMIT are required for PR builds",
		)
	}

	merged, err := loadAndMergeTagRuleConfigs(configPaths)
	if err != nil {
		return nil, fmt.Errorf("load tag rules: %w", err)
	}

	changedFiles, err := lister.ListChangedFiles()
	if err != nil {
		return nil, fmt.Errorf("list changed files: %w", err)
	}

	log.Printf("base branch: %s, commit: %s", baseBranch, commit)
	log.Printf("changed files: %v", changedFiles)

	tags := tagsForChangedFiles(merged.RuleSet, changedFiles)
	log.Printf("selected tags: %s", strings.Join(tags, " "))

	return tags, nil
}
