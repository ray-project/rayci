package raycicmd

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
)

type TagRule struct {
	Tags     []string
	Lineno   int
	Dirs     []string
	Files    []string
	Patterns []string
}

// globToRegex converts a glob pattern to a regex pattern.
// * matches any characters (including /)
// ? matches any single character
// Other special regex chars are escaped.
func globToRegex(pattern string) string {
	var result strings.Builder
	result.WriteString("^")
	for _, ch := range pattern {
		switch ch {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.', '+', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
			result.WriteRune('\\')
			result.WriteRune(ch)
		default:
			result.WriteRune(ch)
		}
	}
	result.WriteString("$")
	return result.String()
}

func (r *TagRule) Match(changedFilePath string) bool {
	if slices.ContainsFunc(r.Dirs, func(dir string) bool {
		return changedFilePath == dir || strings.HasPrefix(changedFilePath, dir+"/")
	}) {
		return true
	}

	if slices.Contains(r.Files, changedFilePath) {
		return true
	}

	// Py version depends on fnmatch.fnmatch, which is not available in std Go.
	// Instead, we treat the pattern as a regex.
	if slices.ContainsFunc(r.Patterns, func(pattern string) bool {
		re, err := regexp.Compile(globToRegex(pattern))
		return err == nil && re.MatchString(changedFilePath)
	}) {
		return true
	}

	return false
}

func (r *TagRule) MatchTags(changedFilePath string) ([]string, bool) {
	if r.Match(changedFilePath) {
		return r.Tags, true
	}
	return []string{}, false
}

func sanitizeLine(line string) string {
	// Remove first # character and anything after it.
	commentIndex := strings.Index(line, "#")
	if commentIndex != -1 {
		line = line[:commentIndex]
	}

	return strings.TrimSpace(line)
}

// Parse the rule config content into a list ot TagRule's.
//
//	The rule content is a string with the following format:
//	```
//	# Comment content, after '#', will be ignored.
//	# Empty lines will be ignored too.
//
//	dir/  # Directory to match
//	file  # File to match
//	dir/*.py  # Pattern to match, using glob pattern, matches dir/a.py dir/dir/b.py or dir/.py
//	@ tag1 tag2 tag3 # Tags to emit for a rule. A rule without tags is a skipping rule.
//	;  # Semicolon to separate rules
//	```
//	Rules are evaluated in order, and the first matched rule will be used.
func parseRulesText(ruleContent string) ([]string, []*TagRule, error) {
	rules := []*TagRule{}
	tagDefs := []string{}
	tagDefsEnded := false

	tags := []string{}
	dirs := []string{}
	files := []string{}
	patterns := []string{}

	trackedLineno := 0
	for lineno, line := range strings.Split(ruleContent, "\n") {
		trackedLineno = lineno + 1
		line := sanitizeLine(strings.TrimSpace(line))

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "!") {
			if tagDefsEnded {
				return nil, nil, fmt.Errorf("tag must be declared at file start. Line %d: %s", trackedLineno, line)
			}

			tagDefs = append(tagDefs, strings.Fields(strings.TrimPrefix(line, "!"))...)
			continue
		}
		tagDefsEnded = true

		if strings.HasPrefix(line, "@") {
			tags = append(tags, strings.Fields(strings.TrimPrefix(line, "@"))...)
			continue
		}

		if strings.HasPrefix(line, ";") {
			if line != ";" {
				panic(fmt.Sprintf("Unexpected tokens after semicolon on line %d: %s", trackedLineno, line))
			}

			// Dump current rule, and reset the state.
			rules = append(rules, &TagRule{Tags: tags, Lineno: trackedLineno, Dirs: dirs, Files: files, Patterns: patterns})
			tags = []string{}
			dirs = []string{}
			files = []string{}
			patterns = []string{}

			continue
		}

		if strings.Contains(line, "*") {
			patterns = append(patterns, line)
			continue
		}

		if strings.HasSuffix(line, "/") {
			dirs = append(dirs, line[:len(line)-1])
			continue
		}

		files = append(files, line)
	}

	// Final flush if any remaining rules
	if len(tags) > 0 || len(dirs) > 0 || len(files) > 0 || len(patterns) > 0 {
		rules = append(rules, &TagRule{Tags: tags, Lineno: trackedLineno, Dirs: dirs, Files: files, Patterns: patterns})
	}

	return tagDefs, rules, nil
}

type TagRuleSet struct {
	tagDefs map[string]struct{}
	rules   []*TagRule
}

func NewTagRuleSet(ruleContent string) (*TagRuleSet, error) {
	set := &TagRuleSet{tagDefs: make(map[string]struct{}), rules: []*TagRule{}}

	if ruleContent == "" {
		return set, nil
	}

	if err := set.AddRules(ruleContent); err != nil {
		return nil, err
	}
	return set, nil
}

func (s *TagRuleSet) AddRules(ruleContent string) error {
	tagDefs, rules, err := parseRulesText(ruleContent)
	if err != nil {
		return err
	}
	for _, tagDef := range tagDefs {
		s.tagDefs[tagDef] = struct{}{}
	}
	s.rules = append(s.rules, rules...)

	return nil
}

func (s *TagRuleSet) ValidateRules() ([]string, error) {
	for _, rule := range s.rules {
		if len(rule.Tags) == 0 {
			continue
		}
		for _, tag := range rule.Tags {
			if _, ok := s.tagDefs[tag]; !ok {
				return []string{}, fmt.Errorf("tag %s not declared, used in rule at line %d", tag, rule.Lineno)
			}
		}
	}
	return []string{}, nil
}

func (s *TagRuleSet) MatchTags(changedFilePath string) ([]string, bool) {
	for _, rule := range s.rules {
		matchTags, matched := rule.MatchTags(changedFilePath)
		if matched {
			return matchTags, true
		}
	}
	return []string{}, false
}

type RunMainConfig struct {
	ConfigPaths []string
	Env         Envs
	Git         GitClient
}

func isPullRequest(env Envs) bool {
	return getEnv(env, "BUILDKITE_PULL_REQUEST") != "false"
}

// sortAndDeduplicateTags sorts tags and removes duplicates.
func sortAndDeduplicateTags(tags []string) []string {
	if len(tags) == 0 {
		return tags
	}
	sort.Strings(tags)
	result := tags[:1]
	for _, tag := range tags[1:] {
		if tag != result[len(result)-1] {
			result = append(result, tag)
		}
	}
	return result
}

func loadTagRuleSet(configPaths []string) (*TagRuleSet, error) {
	ruleSet, err := NewTagRuleSet("")
	if err != nil {
		return nil, fmt.Errorf("new tag rule set: %w", err)
	}

	for _, path := range configPaths {
		bs, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config file %s: %w", path, err)
		}
		if err := ruleSet.AddRules(string(bs)); err != nil {
			return nil, fmt.Errorf("parse rules from %s: %w", path, err)
		}
	}

	if _, err := ruleSet.ValidateRules(); err != nil {
		return nil, fmt.Errorf("validate rules: %w", err)
	}
	return ruleSet, nil
}

// defaultTags are always included in all PR builds.
var defaultTags = []string{"always", "lint"}

// fallbackTags are used when a changed file doesn't match any known rule.
var fallbackTags = strings.Fields(
	"ml tune train data serve core_cpp cpp java python doc " +
		"linux_wheels macos_wheels dashboard tools release_tests",
)

func tagsForChangedFiles(ruleSet *TagRuleSet, files []string) []string {
	tags := append([]string{}, defaultTags...)

	for _, file := range files {
		if matchTags, matched := ruleSet.MatchTags(file); matched {
			tags = append(tags, matchTags...)
		} else {
			log.Printf("Unhandled source code change: %s\n", file)
			tags = append(tags, fallbackTags...)
		}
	}

	return sortAndDeduplicateTags(tags)
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

func RunTagAnalysis(cfg *RunMainConfig) ([]string, error) {
	env := cfg.Env

	if getEnv(env, "BUILDKITE") != "true" {
		return nil, fmt.Errorf("BUILDKITE environment variable is not set")
	}

	needRunAllTags, reason := needRunAllTags(env)
	if needRunAllTags {
		log.Printf("Running all tags: %s\n", reason)
		return []string{"*"}, nil
	}

	baseBranch := getEnv(env, "BUILDKITE_PULL_REQUEST_BASE_BRANCH")
	commit := getEnv(env, "BUILDKITE_COMMIT")
	if baseBranch == "" || commit == "" {
		return nil, fmt.Errorf("BUILDKITE_PULL_REQUEST_BASE_BRANCH and BUILDKITE_COMMIT are required for PR builds")
	}

	ruleSet, err := loadTagRuleSet(cfg.ConfigPaths)
	if err != nil {
		return nil, err
	}

	commitRange := fmt.Sprintf("origin/%s...%s", baseBranch, commit)
	changedFiles, err := cfg.Git.ListChangedFiles(baseBranch, commitRange)
	if err != nil {
		return nil, fmt.Errorf("list changed files: %w", err)
	}

	log.Printf("commit range: %s\n", commitRange)
	log.Printf("changedFiles: %v\n", changedFiles)

	tags := tagsForChangedFiles(ruleSet, changedFiles)
	log.Printf("tags: %s\n", strings.Join(tags, " "))

	return tags, nil
}
