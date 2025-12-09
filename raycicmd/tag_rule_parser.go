package raycicmd

import (
	"fmt"
	"regexp"
	"strings"
)

func sanitizeLine(line string) string {
	// Remove first # character and anything after it.
	commentIndex := strings.Index(line, "#")
	if commentIndex != -1 {
		line = line[:commentIndex]
	}

	return strings.TrimSpace(line)
}

// TagRuleParser holds the state while parsing the ruleContent.
type TagRuleParser struct {
	tagDefs      []string
	rules        []*TagRule
	tagDefsEnded bool

	tags     []string
	dirs     []string
	files    []string
	patterns []*regexp.Regexp

	lineno int
}

// Parse will parse the rule config content into a list of TagRules
//
// ruleContent is a string with the following format:
//
//	# Comment content, after '#', will be ignored.
//	# Empty lines will be ignored too.
//
//	! tag1 tag2 tag3    # Tag declarations, only allowed at file start
//	dir/                # Directory to match
//	file                # File to match
//	dir/*.py            # Pattern to match, using glob pattern
//	@ tag1 tag2         # Tags to emit for a rule. A rule without tags is a skipping rule.
//	;                   # Semicolon to separate rules
//
// Rules are evaluated in order, and the first matched rule will be used.
func (p *TagRuleParser) Parse(ruleContent string) error {
	for i, rawLine := range strings.Split(ruleContent, "\n") {
		p.lineno = i + 1

		// Keep the same sanitization behavior as before.
		line := sanitizeLine(rawLine)
		if line == "" {
			continue
		}

		// Tag definitions must come first, before any other non-empty lines.
		if strings.HasPrefix(line, "!") {
			if err := p.handleTagDef(line); err != nil {
				return err
			}
			continue
		}

		// We have encountered a non-tag-def, non-empty line.
		p.tagDefsEnded = true

		switch {
		case strings.HasPrefix(line, "@"):
			p.handleTags(line)
		case strings.HasPrefix(line, ";"):
			if err := p.flushRule(line); err != nil {
				return err
			}
		default:
			if err := p.handlePathOrPattern(line); err != nil {
				return err
			}
		}
	}

	p.flushFinalRule()
	return nil
}

func (p *TagRuleParser) handleTagDef(line string) error {
	if p.tagDefsEnded {
		return fmt.Errorf("tag must be declared at file start. Line %d: %s", p.lineno, line)
	}
	fields := strings.Fields(strings.TrimPrefix(line, "!"))
	if len(fields) > 0 {
		p.tagDefs = append(p.tagDefs, fields...)
	}
	return nil
}

func (p *TagRuleParser) handleTags(line string) {
	fields := strings.Fields(strings.TrimPrefix(line, "@"))
	if len(fields) > 0 {
		p.tags = append(p.tags, fields...)
	}
}

func (p *TagRuleParser) flushRule(line string) error {
	if line != ";" {
		return fmt.Errorf("unexpected tokens after semicolon on line %d: %s", p.lineno, line)
	}

	// Always append a rule here, even if it's effectively empty,
	// to preserve the original behavior.
	p.rules = append(p.rules, &TagRule{
		Tags:     p.tags,
		Lineno:   p.lineno,
		Dirs:     p.dirs,
		Files:    p.files,
		Patterns: p.patterns,
	})

	// Reset per-rule state.
	p.tags = nil
	p.dirs = nil
	p.files = nil
	p.patterns = nil

	return nil
}

func (p *TagRuleParser) handlePathOrPattern(line string) error {
	if strings.Contains(line, "*") || strings.Contains(line, "?") {
		re, err := globToRegexp(line)
		if err != nil {
			return fmt.Errorf("invalid pattern on line %d: %q: %w", p.lineno, line, err)
		}
		p.patterns = append(p.patterns, re)
		return nil
	}

	if strings.HasSuffix(line, "/") {
		// Store directory without trailing slash, consistent with matcher.
		p.dirs = append(p.dirs, line[:len(line)-1])
		return nil
	}

	p.files = append(p.files, line)
	return nil
}

func (p *TagRuleParser) flushFinalRule() {
	if len(p.tags) == 0 && len(p.dirs) == 0 && len(p.files) == 0 && len(p.patterns) == 0 {
		return
	}

	p.rules = append(p.rules, &TagRule{
		Tags:     p.tags,
		Lineno:   p.lineno,
		Dirs:     p.dirs,
		Files:    p.files,
		Patterns: p.patterns,
	})
}
