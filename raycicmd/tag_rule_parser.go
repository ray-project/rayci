package raycicmd

import (
	"fmt"
	"regexp"
	"strings"
)

// sanitizeLine removes the first # character and anything after it.
func sanitizeLine(line string) string {
	commentIndex := strings.Index(line, "#")
	if commentIndex != -1 {
		line = line[:commentIndex]
	}

	return strings.TrimSpace(line)
}

// pendingRule holds the accumulated state for a rule being parsed.
// This state is flushed to create a TagRule when a semicolon or EOF is encountered.
type pendingRule struct {
	// tags is the list of tags seen in the order they were parsed.
	tags []string
	// dirs is the list of directories seen in the order they were parsed.
	dirs []string
	// files is the list of files seen in the order they were parsed.
	files []string
	// patterns is the list of glob patterns (converted to regex patterns) seen
	// in the order they were parsed.
	patterns []*regexp.Regexp
}

// flush creates a TagRule from the pending state and resets it.
func (pr *pendingRule) flush(lineno int) *TagRule {
	rule := &TagRule{
		Tags:     pr.tags,
		Lineno:   lineno,
		Dirs:     pr.dirs,
		Files:    pr.files,
		Patterns: pr.patterns,
	}
	pr.tags = nil
	pr.dirs = nil
	pr.files = nil
	pr.patterns = nil

	return rule
}

// isEmpty returns true if the pending rule has no content.
func (pr *pendingRule) isEmpty() bool {
	return len(pr.tags) == 0 && len(pr.dirs) == 0 &&
		len(pr.files) == 0 && len(pr.patterns) == 0
}

// tagRuleParser holds the intermediate state while parsing a rule config content.
type tagRuleParser struct {
	// tagDefs is the list of tag definitions seen in the order they were parsed.
	tagDefs []string
	// rules is the list of TagRules seen in the order they were parsed.
	rules []*TagRule
	// pending holds the state for the rule currently being parsed.
	pending pendingRule
	// lineno is the line number of the current line being parsed.
	lineno int
}

func (p *tagRuleParser) handleTagDef(line string, tagDefsEnded bool) error {
	if tagDefsEnded {
		return fmt.Errorf(
			"tag must be declared at file start. Line %d: %s",
			p.lineno,
			line,
		)
	}
	fields := strings.Fields(strings.TrimPrefix(line, "!"))
	if len(fields) > 0 {
		p.tagDefs = append(p.tagDefs, fields...)
	}
	return nil
}

func (p *tagRuleParser) handleTags(line string) {
	fields := strings.Fields(strings.TrimPrefix(line, "@"))
	if len(fields) > 0 {
		p.pending.tags = append(p.pending.tags, fields...)
	}
}

func (p *tagRuleParser) flushRule(line string) error {
	if line != ";" {
		return fmt.Errorf(
			"unexpected tokens after semicolon on line %d: %s",
			p.lineno,
			line,
		)
	}

	// Always append a rule here, even if it's effectively empty,
	// to preserve the original behavior.
	p.rules = append(p.rules, p.pending.flush(p.lineno))
	return nil
}

func (p *tagRuleParser) handlePathOrPattern(line string) error {
	if strings.Contains(line, "*") || strings.Contains(line, "?") {
		re, err := globToRegexp(line)
		if err != nil {
			return fmt.Errorf(
				"invalid pattern on line %d: %q: %w",
				p.lineno,
				line,
				err,
			)
		}
		p.pending.patterns = append(p.pending.patterns, re)
		return nil
	}

	if strings.HasSuffix(line, "/") {
		// Store directory without trailing slash, consistent with matcher.
		p.pending.dirs = append(p.pending.dirs, line[:len(line)-1])
		return nil
	}

	p.pending.files = append(p.pending.files, line)
	return nil
}

func (p *tagRuleParser) flushFinalRule() {
	if p.pending.isEmpty() {
		return
	}
	p.rules = append(p.rules, p.pending.flush(p.lineno))
}

// Parse will parse the rule config content into a list of TagRules and a list
// of tag definitions.
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
func ParseTagRuleConfig(ruleContent string) ([]*TagRule, []string, error) {
	p := &tagRuleParser{}
	tagDefsEnded := false

	for i, rawLine := range strings.Split(ruleContent, "\n") {
		p.lineno = i + 1

		line := sanitizeLine(rawLine)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "!") {
			if err := p.handleTagDef(line, tagDefsEnded); err != nil {
				return nil, nil, err
			}
			continue
		}

		tagDefsEnded = true
		switch {
		case strings.HasPrefix(line, "@"):
			p.handleTags(line)
		case strings.HasPrefix(line, ";"):
			if err := p.flushRule(line); err != nil {
				return nil, nil, err
			}
		default:
			if err := p.handlePathOrPattern(line); err != nil {
				return nil, nil, err
			}
		}
	}

	p.flushFinalRule()
	return p.rules, p.tagDefs, nil
}
