package raycicmd

import (
	"regexp"
	"slices"
	"strings"
)

type TagRule struct {
	Tags     []string
	Lineno   int
	Dirs     []string
	Files    []string
	Patterns []*regexp.Regexp
}

// **AI-generated code.
// globToRegexp converts a glob pattern to an equivalent regex pattern.
// * matches any characters (including /)
// ? matches any single character
// Other special regex chars are escaped.
func globToRegexp(pattern string) *regexp.Regexp {
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
	return regexp.MustCompile(result.String())
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
	if slices.ContainsFunc(r.Patterns, func(re *regexp.Regexp) bool {
		return re.MatchString(changedFilePath)
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
