package raycicmd

import (
	"fmt"
	"regexp"
	"strings"
)

// TagRuleConfig holds the parsed result of a tag rule configuration file.
type TagRuleConfig struct {
	// Rules is the list of tag rules in the order they were defined.
	// Rules are evaluated in order, and the first matched rule will be used.
	Rules []*TagRule

	// TagDefs is the list of declared tag names in the order they were defined.
	// These tags are declared with "!" at the start of the file.
	TagDefs []string
}

// ParseTagRuleConfig parses rule config content into a TagRuleConfig.
//
// ruleContent is a string with the following format:
//
//	# Comment content, after '#', will be ignored.
//	# Empty lines will be ignored too.
//
//	! tag1 tag2 tag3    # Tag declarations, only allowed at file start
//
//	# Rules section (tag declarations must end before rules begin)
//	dir/                # Directory to match
//	file                # File to match
//	dir/*.py            # Pattern to match, using glob pattern
//	*                   # Matches any file (catch-all)
//	\fallthrough        # Tags are always included, matching continues
//	@ tag1 tag2         # Tags to emit for a rule. A rule without tags is a skipping rule.
//	;                   # Semicolon to separate rules
//
// Rules are evaluated in order, and the first matched rule will be used
// (unless it has \fallthrough, which continues matching).
func ParseTagRuleConfig(ruleContent string) (*TagRuleConfig, error) {
	p := &tagRuleParser{}
	if err := p.parse(ruleContent); err != nil {
		return nil, err
	}

	return &TagRuleConfig{
		Rules:   p.rules,
		TagDefs: p.tagDefs,
	}, nil
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
	// fallthrough means this rule's tags are always included.
	fallthrough_ bool
	// seenTags is true if we've seen @ tags in this rule.
	// Directives must come before tags.
	seenTags bool
}

func (pr *pendingRule) flush(lineno int) *TagRule {
	rule := &TagRule{
		Tags:        pr.tags,
		Lineno:      lineno,
		Dirs:        pr.dirs,
		Files:       pr.files,
		Patterns:    pr.patterns,
		Fallthrough: pr.fallthrough_,
	}
	*pr = pendingRule{} // reset all fields
	return rule
}

func (pr *pendingRule) isEmpty() bool {
	return len(pr.tags) == 0 && len(pr.dirs) == 0 &&
		len(pr.files) == 0 && len(pr.patterns) == 0 &&
		!pr.fallthrough_ && !pr.seenTags
}

// tagRuleParser holds the intermediate state while parsing rule config content.
type tagRuleParser struct {
	// tagDefs is the list of tag definitions seen in the order they were parsed.
	tagDefs []string
	// rules is the list of TagRules seen in the order they were parsed.
	rules []*TagRule
	// pending is the current rule being accumulated.
	pending pendingRule
	// lineno is the line number of the current line being parsed.
	lineno int
	// tagDefsEnded is true if the tag definitions have ended.
	tagDefsEnded bool
}

func (p *tagRuleParser) parse(ruleContent string) error {
	for i, rawLine := range strings.Split(ruleContent, "\n") {
		p.lineno = i + 1
		if err := p.parseLine(rawLine); err != nil {
			return err
		}
	}
	return p.flushFinalRule()
}

// parseLine parses a single line by dispatching to the appropriate handler
// based on the line structure.
func (p *tagRuleParser) parseLine(rawLine string) error {
	line := sanitizeLine(rawLine)
	if line == "" {
		return nil
	}

	if strings.HasPrefix(line, "!") {
		return p.handleTagDef(line)
	}

	p.tagDefsEnded = true
	switch {
	case strings.HasPrefix(line, "@"):
		p.handleTags(line)
	case strings.HasPrefix(line, ";"):
		return p.handleRuleEnd(line)
	case strings.HasPrefix(line, "\\"):
		return p.handleDirective(line)
	default:
		return p.handlePathOrPattern(line)
	}
	return nil
}

// handleTagDef takes all tags separated by whitespace and adds them to the tagDefs list.
// Any tag definitions after a rule will cause an error, as no tag definitions
// are allowed after defining a rule.
//
// Tag declarations use "!" prefix with optional space:
//   - "! tag1 tag2" adds tag1 and tag2 to tagDefs
//   - "!tag1 tag2" also adds tag1 and tag2 to tagDefs
func (p *tagRuleParser) handleTagDef(line string) error {
	if p.tagDefsEnded {
		return fmt.Errorf(
			"tag must be declared at file start. Line %d: %s",
			p.lineno, line,
		)
	}

	content := strings.TrimPrefix(line, "!")
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return nil
	}

	p.tagDefs = append(p.tagDefs, fields...)
	return nil
}

// handleDirective handles rule directives that modify rule behavior.
// Supported directives:
//   - \fallthrough: Tags are always included, matching continues to next rule
//
// Directives must appear before @ tags within a rule.
func (p *tagRuleParser) handleDirective(line string) error {
	// Directives must come before tags
	if p.pending.seenTags {
		return fmt.Errorf(
			"directive on line %d must appear before @ tags",
			p.lineno,
		)
	}
	directive := strings.TrimPrefix(line, "\\")
	switch directive {
	case "fallthrough":
		p.pending.fallthrough_ = true
	default:
		return fmt.Errorf("unknown directive on line %d: %s", p.lineno, line)
	}
	return nil
}

// handleTags takes all tags separated by whitespace and adds them to the pending rule's tags list.
func (p *tagRuleParser) handleTags(line string) {
	if fields := strings.Fields(strings.TrimPrefix(line, "@")); len(fields) > 0 {
		p.pending.tags = append(p.pending.tags, fields...)
		p.pending.seenTags = true
	}
}

// validateAndAppendRule validates the pending rule and appends it to the rules list.
func (p *tagRuleParser) validateAndAppendRule() error {
	p.rules = append(p.rules, p.pending.flush(p.lineno))
	return nil
}

// handleRuleEnd flushes the pending rule and adds it to the rules list.
func (p *tagRuleParser) handleRuleEnd(line string) error {
	if line != ";" {
		return fmt.Errorf(
			"unexpected tokens after semicolon on line %d: %s",
			p.lineno, line,
		)
	}
	return p.validateAndAppendRule()
}

// handlePathOrPattern handles paths and patterns by dispatching to the
// appropriate handler based on the line structure.
// If the line contains a * or ?, it is a pattern and is converted to a regex pattern.
// If the line ends with a /, it is a directory and is added to the pending rule's
// dirs list.
// Otherwise, it is a file and is added to the pending rule's files list.
func (p *tagRuleParser) handlePathOrPattern(line string) error {
	switch {
	case strings.Contains(line, "*") || strings.Contains(line, "?"):
		re, err := globToRegexp(line)
		if err != nil {
			return fmt.Errorf("invalid pattern on line %d: %q: %w", p.lineno, line, err)
		}
		p.pending.patterns = append(p.pending.patterns, re)
	case strings.HasSuffix(line, "/"):
		// Store directory without trailing slash, consistent with matcher.
		p.pending.dirs = append(p.pending.dirs, line[:len(line)-1])
	default:
		p.pending.files = append(p.pending.files, line)
	}
	return nil
}

// flushFinalRule flushes any remaining pending rules and adds it to the rules list.
func (p *tagRuleParser) flushFinalRule() error {
	if !p.pending.isEmpty() {
		return p.validateAndAppendRule()
	}
	return nil
}

// sanitizeLine removes comments (everything after #) and trims whitespace.
func sanitizeLine(line string) string {
	if i := strings.Index(line, "#"); i != -1 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}
