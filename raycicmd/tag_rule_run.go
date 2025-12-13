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
	// DefaultTags are always included, regardless of which files changed.
	DefaultTags map[string]struct{}
	// FallbackTags are added when any changed file doesn't match a known rule.
	FallbackTags map[string]struct{}
}

// loadAndMergeTagRuleConfigs loads and merges tag rule configurations from multiple files.
// Tag definitions and rules from all files are combined into a single TagRuleSet.
// Default and fallback tags are also merged.
func loadAndMergeTagRuleConfigs(configPaths []string) (*mergedTagRuleConfig, error) {
	merged := &mergedTagRuleConfig{
		RuleSet: &TagRuleSet{
			tagDefs: make(map[string]struct{}),
		},
		DefaultTags:  make(map[string]struct{}),
		FallbackTags: make(map[string]struct{}),
	}

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
		for _, tag := range cfg.DefaultTags {
			merged.DefaultTags[tag] = struct{}{}
		}
		for _, tag := range cfg.FallbackTags {
			merged.FallbackTags[tag] = struct{}{}
		}
		merged.RuleSet.rules = append(merged.RuleSet.rules, cfg.Rules...)
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
// It always includes the default tags, then adds tags matched by each file.
// If any file doesn't match a rule, fallback tags are added to ensure coverage.
func tagsForChangedFiles(
	ruleSet *TagRuleSet,
	defaultTags map[string]struct{},
	fallbackTags map[string]struct{},
	files []string,
) []string {
	tagSet := make(map[string]struct{})
	for tag := range defaultTags {
		tagSet[tag] = struct{}{}
	}

	hasUnmatchedFiles := false
	for _, file := range files {
		matchedTags, matched := ruleSet.MatchTags(file)
		if matched {
			for _, tag := range matchedTags {
				tagSet[tag] = struct{}{}
			}
		} else {
			log.Printf("unhandled file (no matching rule): %s", file)
			hasUnmatchedFiles = true
		}
	}

	if hasUnmatchedFiles {
		for tag := range fallbackTags {
			tagSet[tag] = struct{}{}
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

	tags := tagsForChangedFiles(merged.RuleSet, merged.DefaultTags, merged.FallbackTags, changedFiles)
	log.Printf("selected tags: %s", strings.Join(tags, " "))

	return tags, nil
}
