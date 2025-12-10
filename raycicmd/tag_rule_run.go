package raycicmd

import (
	"fmt"
	"log"
	"os"
	"slices"
	"sort"
	"strings"
)

func loadTagRuleSet(configPaths []string) (*TagRuleSet, error) {
	combinedSet := &TagRuleSet{
		tagDefs: make(map[string]struct{}),
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
			combinedSet.tagDefs[tagDef] = struct{}{}
		}
		combinedSet.rules = append(combinedSet.rules, cfg.Rules...)
	}
	return combinedSet, nil
}

func isPullRequest(env Envs) bool {
	return getEnv(env, "BUILDKITE_PULL_REQUEST") != "false" && getEnv(env, "BUILDKITE_PULL_REQUEST") != ""
}

func needRunAllTags(env Envs) (bool, string) {
	if getEnv(env, "RAYCI_RUN_ALL_TESTS") == "1" {
		return true, "RAYCI_RUN_ALL_TESTS is set, running all tags"
	}

	if getEnv(env, "BUILDKITE_BRANCH") == "master" {
		return true, "BUILDKITE_BRANCH is master, running all tags"
	}

	if strings.HasPrefix(getEnv(env, "BUILDKITE_BRANCH"), "releases/") {
		return true, "BUILDKITE_BRANCH starts with releases/, running all tags"
	}

	if !isPullRequest(env) {
		return true, "Not a PR build... skipping config parsing and running all tags"
	}

	return false, "No special conditions met, running tags based on config files"
}

func sortAndDeduplicateTags(tags []string) []string {
	if len(tags) < 2 {
		return tags
	}
	sort.Strings(tags)
	return slices.Compact(tags)
}

// defaultTags are always included in all PR builds.
var defaultTags = []string{"always", "lint"}

// fallbackTags are used when a changed file doesn't match any known rule.
var fallbackTags = strings.Fields(
	"ml tune train data serve core_cpp cpp java python doc " +
		"linux_wheels macos_wheels dashboard tools release_tests",
)

func tagsForChangedFiles(ruleSet *TagRuleSet, files []string) []string {
	tagSet := make(map[string]struct{})
	for _, tag := range defaultTags {
		tagSet[tag] = struct{}{}
	}

	hasUnhandledFiles := false
	for _, file := range files {
		if matchTags, matched := ruleSet.MatchTags(file); matched {
			for _, tag := range matchTags {
				tagSet[tag] = struct{}{}
			}
		} else {
			log.Printf("Unhandled source code change: %s\n", file)
			hasUnhandledFiles = true
		}
	}

	if hasUnhandledFiles {
		for _, tag := range fallbackTags {
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

func RunTagAnalysis(
	configPaths []string,
	env Envs,
	git GitClient,
) ([]string, error) {
	if getEnv(env, "BUILDKITE") != "true" {
		return nil, fmt.Errorf("BUILDKITE environment variable is not set")
	}

	needRunAllTags, reason := needRunAllTags(env)
	if needRunAllTags {
		log.Printf("Running all tags: %s\n", reason)
		return []string{"*"}, nil
	}

	// If the config file does not exist, run all tags. This is the equivalent
	// of the default behavior in the original Python code in cases that the
	// binary ci/pipeline/test_conditional_testing.py does not exist.
	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			log.Printf("config file %s does not exist, running all tags: %s", configPath, err)
			return []string{"*"}, nil
		}
	}

	baseBranch := getEnv(env, "BUILDKITE_PULL_REQUEST_BASE_BRANCH")
	commit := getEnv(env, "BUILDKITE_COMMIT")
	if baseBranch == "" || commit == "" {
		return nil, fmt.Errorf(
			"BUILDKITE_PULL_REQUEST_BASE_BRANCH and BUILDKITE_COMMIT are required for PR builds",
		)
	}

	ruleSet, err := loadTagRuleSet(configPaths)
	if err != nil {
		return nil, err
	}

	commitRange := fmt.Sprintf("origin/%s...%s", baseBranch, commit)
	changedFiles, err := git.ListChangedFiles(baseBranch, commitRange)
	if err != nil {
		return nil, fmt.Errorf("list changed files: %w", err)
	}

	log.Printf("commit range: %s\n", commitRange)
	log.Printf("changedFiles: %v\n", changedFiles)

	tags := tagsForChangedFiles(ruleSet, changedFiles)
	log.Printf("tags: %s\n", strings.Join(tags, " "))

	return tags, nil
}
