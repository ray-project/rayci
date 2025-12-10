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
}

// **AI-generated code.
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
// directories, files, or glob patterns.
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
