package raycicmd

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// TagRule defines a rule that maps changed file paths to tags for Buildkite
// pipeline steps. When a file path matches any of the specified directories,
// files, OR glob patterns, the associated tags are applied.
type TagRule struct {
	// Tags is a list of tags to apply when the rule matches.
	Tags []string
	// Lineno is the line number of the rule in the config file.
	Lineno int
	// Dirs is a list of directories to match.
	Dirs []string
	// Files is a list of files to match.
	Files []string
	// Patterns is a list of glob patterns (converted to regex patterns) to match.
	Patterns []*regexp.Regexp
	// Fallthrough means this rule's tags are always included, and matching
	// continues to find more specific rules. Used with \fallthrough directive.
	Fallthrough bool
	// Default means this rule matches any file. Used as a catch-all with \default
	// directive.
	Default bool
}

// globToRegexp converts a glob pattern to an equivalent regex pattern.
// * matches any characters (including /)
// ? matches any single character
// Other special regex chars are escaped.
func globToRegexp(pattern string) (*regexp.Regexp, error) {
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
	re, err := regexp.Compile(result.String())
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}
	return re, nil
}

// Match returns true if the given file path matches any of the rule's
// directories, files, or glob patterns. A rule with Default=true matches any file.
func (r *TagRule) Match(changedFilePath string) bool {
	if r.Default {
		return true
	}

	if slices.ContainsFunc(r.Dirs, func(dir string) bool {
		return changedFilePath == dir ||
			strings.HasPrefix(changedFilePath, dir+"/")
	}) {
		return true
	}

	if slices.Contains(r.Files, changedFilePath) {
		return true
	}

	// Py version depends on fnmatch.fnmatch, which is not available in std Go.
	// Instead, we treat the pattern as a regex.
	if slices.ContainsFunc(r.Patterns, func(re *regexp.Regexp) bool {
		return re.MatchString(changedFilePath)
	}) {
		return true
	}

	return false
}

// MatchTags returns the tags for the rule if the given file path matches any
// of the rule's directories, files, or glob patterns.
func (r *TagRule) MatchTags(changedFilePath string) ([]string, bool) {
	if r.Match(changedFilePath) {
		return r.Tags, true
	}
	return []string{}, false
}

// TagRuleSet is a set of TagRules, used to match tags for changed files.
type TagRuleSet struct {
	// tagDefs is the set of all defined tags.
	tagDefs map[string]struct{}
	// rules is a list of non-default TagRule instances in the order they were parsed.
	rules []*TagRule
	// defaultRules is a list of default (catch-all) TagRule instances.
	// These are used when no other rule matches.
	defaultRules []*TagRule
}

// ValidateRules validates that all tags used in the rules are defined.
func (s *TagRuleSet) ValidateRules() error {
	allRules := append(s.rules, s.defaultRules...)
	for _, rule := range allRules {
		if len(rule.Tags) == 0 {
			continue
		}
		for _, tag := range rule.Tags {
			if _, ok := s.tagDefs[tag]; !ok {
				return fmt.Errorf(
					"tag %s not declared, used in rule at line %d",
					tag,
					rule.Lineno,
				)
			}
		}
	}
	return nil
}

// MatchTags returns the accumulated tags for rules matching the given file path.
// For fallthrough rules, tags are accumulated and matching continues.
// For non-fallthrough rules, tags are added and matching stops.
// If no rule matches, tags from all default rules are returned.
func (s *TagRuleSet) MatchTags(changedFilePath string) ([]string, bool) {
	tags := []string{}
	matched := false

	for _, rule := range s.rules {
		if rule.Match(changedFilePath) {
			tags = append(tags, rule.Tags...)
			matched = true
			if !rule.Fallthrough {
				break // Stop on first non-fallthrough match
			}
		}
	}

	// If no rule matched, use default rules (but still return matched=false)
	if !matched && len(s.defaultRules) > 0 {
		for _, rule := range s.defaultRules {
			tags = append(tags, rule.Tags...)
		}
	}

	return tags, matched
}
