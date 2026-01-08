package raycicmd

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

// loadTagRuleConfigs loads tag rule configurations from multiple files.
// Each config file is loaded into its own TagRuleSet for independent evaluation.
func loadTagRuleConfigs(configPaths []string) ([]*TagRuleSet, error) {
	var ruleSets []*TagRuleSet

	for _, configPath := range configPaths {
		ruleContent, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		cfg, err := ParseTagRuleConfig(string(ruleContent))
		if err != nil {
			return nil, err
		}

		ruleSet := &TagRuleSet{
			tagDefs: make(map[string]struct{}),
			rules:   cfg.Rules,
		}
		for _, tagDef := range cfg.TagDefs {
			ruleSet.tagDefs[tagDef] = struct{}{}
		}

		if err := ruleSet.ValidateRules(); err != nil {
			return nil, fmt.Errorf("validate tag rule config %s: %w", configPath, err)
		}

		ruleSets = append(ruleSets, ruleSet)
	}

	return ruleSets, nil
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
// Each rule set is evaluated independently for each file. The first matching
// rule in each rule set determines the tags for that file from that config.
// Tags from all rule sets are unioned together.
func tagsForChangedFiles(ruleSets []*TagRuleSet, files []string) []string {
	tagSet := make(map[string]struct{})

	for _, file := range files {
		fileMatched := false
		for _, ruleSet := range ruleSets {
			tags, matched := ruleSet.MatchTags(file)
			// Include tags if either:
			// - A terminating rule matched (matched=true), or
			// - Default rules provided tags (matched=false but tags non-empty)
			if matched || len(tags) > 0 {
				fileMatched = true
				for _, tag := range tags {
					tagSet[tag] = struct{}{}
				}
			}
		}
		if !fileMatched {
			log.Printf("unhandled file (no matching rule): %s", file)
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

	ruleSets, err := loadTagRuleConfigs(configPaths)
	if err != nil {
		return nil, fmt.Errorf("load tag rules: %w", err)
	}

	changedFiles, err := lister.ListChangedFiles()
	if err != nil {
		return nil, fmt.Errorf("list changed files: %w", err)
	}

	log.Printf("base branch: %s, commit: %s", baseBranch, commit)
	log.Printf("changed files: %v", changedFiles)

	tags := tagsForChangedFiles(ruleSets, changedFiles)
	log.Printf("selected tags: %s", strings.Join(tags, " "))

	return tags, nil
}
