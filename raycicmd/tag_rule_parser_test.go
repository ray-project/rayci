package raycicmd

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestSanitizeLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "line without comment",
			input: "python/ray/air/",
			want:  "python/ray/air/",
		},
		{
			name:  "line with trailing comment",
			input: "python/ray/air/ # This is a comment",
			want:  "python/ray/air/",
		},
		{
			name:  "only comment",
			input: "# This is just a comment",
			want:  "",
		},
		{
			name:  "line with leading whitespace",
			input: "   python/ray/air/",
			want:  "python/ray/air/",
		},
		{
			name:  "line with trailing whitespace",
			input: "python/ray/air/   ",
			want:  "python/ray/air/",
		},
		{
			name:  "line with whitespace and comment",
			input: "   python/ray/air/   # comment",
			want:  "python/ray/air/",
		},
		{
			name:  "empty line",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   \t  ",
			want:  "",
		},
		{
			name:  "comment with hash in path preserved before hash",
			input: "path/to/file # my comment # whoa! a double comment",
			want:  "path/to/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeLine(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeLine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTagRuleParserParse_TagDefinitions(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantTagDefs []string
		wantErr     bool
	}{
		{
			name:        "single tag definition line",
			input:       "! tag1 tag2 tag3",
			wantTagDefs: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:        "multiple tag definition lines",
			input:       "! tag1 tag2\n! tag3 tag4",
			wantTagDefs: []string{"tag1", "tag2", "tag3", "tag4"},
		},
		{
			name:        "tag definition with comment",
			input:       "! tag1 tag2 # these are tags",
			wantTagDefs: []string{"tag1", "tag2"},
		},
		{
			name:        "tag definition with extra whitespace",
			input:       "!   tag1    tag2   tag3  ",
			wantTagDefs: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:        "empty tag definition",
			input:       "!",
			wantTagDefs: nil,
		},
		{
			name: "tag definition after rule should error",
			input: `python/
@ sometag
;
! late_tag`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &TagRuleParser{}
			err := p.Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(p.tagDefs, tt.wantTagDefs) {
				t.Errorf("Parse() tagDefs = %v, want %v", p.tagDefs, tt.wantTagDefs)
			}
		})
	}
}

func TestTagRuleParserParse_SimpleRules(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantRules int
		checkRule func(t *testing.T, rules []*TagRule)
	}{
		{
			name: "single rule with directory",
			input: `! mytag
python/
@ mytag
;`,
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if want := []string{"python"}; !reflect.DeepEqual(rules[0].Dirs, want) {
					t.Errorf("expected Dirs=%v, got %v", want, rules[0].Dirs)
				}
				if want := []string{"mytag"}; !reflect.DeepEqual(rules[0].Tags, want) {
					t.Errorf("expected Tags=%v, got %v", want, rules[0].Tags)
				}
			},
		},
		{
			name: "single rule with file",
			input: `! mytag
README.md
@ mytag
;`,
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if len(rules[0].Files) != 1 || rules[0].Files[0] != "README.md" {
					t.Errorf("expected Files=[README.md], got %v", rules[0].Files)
				}
			},
		},
		{
			name: "single rule with pattern",
			input: `! mytag
python/*.py
@ mytag
;`,
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if len(rules[0].Patterns) != 1 {
					t.Errorf("expected 1 pattern, got %d", len(rules[0].Patterns))
				}
				// Verify the pattern matches expected files
				if !rules[0].Patterns[0].MatchString("python/test.py") {
					t.Errorf("pattern should match python/test.py")
				}
			},
		},
		{
			name: "multiple rules",
			input: `! tag1 tag2
python/
@ tag1
;
golang/
@ tag2
;`,
			wantRules: 2,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if rules[0].Dirs[0] != "python" {
					t.Errorf("first rule Dirs = %v, want [python]", rules[0].Dirs)
				}
				if rules[1].Dirs[0] != "golang" {
					t.Errorf("second rule Dirs = %v, want [golang]", rules[1].Dirs)
				}
			},
		},
		{
			name: "rule without semicolon at end",
			input: `! mytag
python/
@ mytag`,
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if len(rules[0].Dirs) != 1 || rules[0].Dirs[0] != "python" {
					t.Errorf("expected Dirs=[python], got %v", rules[0].Dirs)
				}
			},
		},
		{
			name: "skip rule (no tags)",
			input: `.git/
;`,
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if len(rules[0].Tags) != 0 {
					t.Errorf("expected empty Tags for skip rule, got %v", rules[0].Tags)
				}
				if rules[0].Dirs[0] != ".git" {
					t.Errorf("expected Dirs=[.git], got %v", rules[0].Dirs)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &TagRuleParser{}
			err := p.Parse(tt.input)

			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			if len(p.rules) != tt.wantRules {
				t.Errorf("Parse() rules count = %d, want %d", len(p.rules), tt.wantRules)
				return
			}

			if tt.checkRule != nil {
				tt.checkRule(t, p.rules)
			}
		})
	}
}

func TestTagRuleParserParse_MultiplePaths(t *testing.T) {
	input := `! mytag
python/
golang/
src/main.go
*.md
@ mytag
;`

	p := &TagRuleParser{}
	err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if len(p.rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(p.rules))
	}

	rule := p.rules[0]

	expectedDirs := []string{"python", "golang"}
	if !reflect.DeepEqual(rule.Dirs, expectedDirs) {
		t.Errorf("Dirs = %v, want %v", rule.Dirs, expectedDirs)
	}

	expectedFiles := []string{"src/main.go"}
	if !reflect.DeepEqual(rule.Files, expectedFiles) {
		t.Errorf("Files = %v, want %v", rule.Files, expectedFiles)
	}

	if len(rule.Patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(rule.Patterns))
	}
}

func TestTagRuleParserParse_MultipleTags(t *testing.T) {
	input := `! tag1 tag2 tag3
python/
@ tag1 tag2
@ tag3
;`

	p := &TagRuleParser{}
	err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if len(p.rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(p.rules))
	}

	expectedTags := []string{"tag1", "tag2", "tag3"}
	sort.Strings(p.rules[0].Tags)
	sort.Strings(expectedTags)
	if !reflect.DeepEqual(p.rules[0].Tags, expectedTags) {
		t.Errorf("Tags = %v, want %v", p.rules[0].Tags, expectedTags)
	}
}

func TestTagRuleParserParse_Comments(t *testing.T) {
	input := `# This is a comment at the start
! tag1 tag2 # tag definitions

# Another comment
python/ # Directory to match
@ tag1 # Tag assignment
; # Rule separator
`

	p := &TagRuleParser{}
	err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if len(p.tagDefs) != 2 {
		t.Errorf("expected 2 tag definitions, got %d: %v", len(p.tagDefs), p.tagDefs)
	}

	if len(p.rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(p.rules))
	}
}

func TestTagRuleParserParse_EmptyLines(t *testing.T) {
	input := `! tag1

python/

@ tag1

;`

	p := &TagRuleParser{}
	err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if len(p.rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(p.rules))
	}

	if len(p.rules[0].Dirs) != 1 || p.rules[0].Dirs[0] != "python" {
		t.Errorf("expected Dirs=[python], got %v", p.rules[0].Dirs)
	}
}

func TestTagRuleParserParse_Errors(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError string
	}{
		{
			name: "tag definition after rule starts",
			input: `python/
! tag1`,
			wantError: "tag must be declared at file start",
		},
		{
			name: "tokens after semicolon",
			input: `python/
; extra_stuff`,
			wantError: "unexpected tokens after semicolon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &TagRuleParser{}
			err := p.Parse(tt.input)

			if err == nil {
				t.Errorf("Parse() expected error containing %q, got nil", tt.wantError)
				return
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("Parse() error = %q, want error containing %q", err.Error(), tt.wantError)
			}
		})
	}
}

func TestTagRuleParserParse_PatternTypes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantDirs    []string
		wantFiles   []string
		wantPattern bool
	}{
		{
			name:     "directory with trailing slash",
			input:    "python/",
			wantDirs: []string{"python"},
		},
		{
			name:      "file without trailing slash",
			input:     "README.md",
			wantFiles: []string{"README.md"},
		},
		{
			name:      "nested file path",
			input:     "src/main/java/App.java",
			wantFiles: []string{"src/main/java/App.java"},
		},
		{
			name:        "pattern with asterisk",
			input:       "*.py",
			wantPattern: true,
		},
		{
			name:        "pattern with question mark",
			input:       "test?.py",
			wantPattern: true,
		},
		{
			name:        "pattern with path and asterisk",
			input:       "python/**/*.py",
			wantPattern: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &TagRuleParser{}
			err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			// Should have one rule from flushFinalRule
			if len(p.rules) != 1 {
				t.Fatalf("expected 1 rule, got %d", len(p.rules))
			}

			rule := p.rules[0]

			if tt.wantDirs != nil && !reflect.DeepEqual(rule.Dirs, tt.wantDirs) {
				t.Errorf("Dirs = %v, want %v", rule.Dirs, tt.wantDirs)
			}

			if tt.wantFiles != nil && !reflect.DeepEqual(rule.Files, tt.wantFiles) {
				t.Errorf("Files = %v, want %v", rule.Files, tt.wantFiles)
			}

			if tt.wantPattern && len(rule.Patterns) == 0 {
				t.Errorf("expected pattern to be parsed, got none")
			}

			if !tt.wantPattern && len(rule.Patterns) > 0 {
				t.Errorf("expected no patterns, got %d", len(rule.Patterns))
			}
		})
	}
}

func TestTagRuleParserParse_LineNumbers(t *testing.T) {
	input := `! tag1
python/
@ tag1
;
golang/
@ tag1
;`

	p := &TagRuleParser{}
	err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(p.rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(p.rules))
	}

	// First rule ends at line 4 (the ;)
	if p.rules[0].Lineno != 4 {
		t.Errorf("first rule Lineno = %d, want 4", p.rules[0].Lineno)
	}

	// Second rule ends at line 7 (the ;)
	if p.rules[1].Lineno != 7 {
		t.Errorf("second rule Lineno = %d, want 7", p.rules[1].Lineno)
	}
}

func TestTagRuleParserParse_RealWorldExample(t *testing.T) {
	input := `# Conditional testing rules
! python ml data serve train tune

# Skip hidden files
.git/
.github/
;

# Python core
python/ray/
@ python
;

# ML libraries
python/ray/train/
python/ray/tune/
@ ml train tune
;

# Data processing
python/ray/data/
@ data
;
`

	p := &TagRuleParser{}
	err := p.Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Should have tag definitions
	expectedTagDefs := []string{"python", "ml", "data", "serve", "train", "tune"}
	sort.Strings(p.tagDefs)
	sort.Strings(expectedTagDefs)
	if !reflect.DeepEqual(p.tagDefs, expectedTagDefs) {
		t.Errorf("tagDefs = %v, want %v", p.tagDefs, expectedTagDefs)
	}

	// Should have 4 rules
	if len(p.rules) != 4 {
		t.Errorf("expected 4 rules, got %d", len(p.rules))
	}

	// First rule should be a skip rule (no tags)
	if len(p.rules[0].Tags) != 0 {
		t.Errorf("first rule should have no tags (skip rule), got %v", p.rules[0].Tags)
	}

	// Second rule should have python tag
	if len(p.rules[1].Tags) != 1 || p.rules[1].Tags[0] != "python" {
		t.Errorf("second rule Tags = %v, want [python]", p.rules[1].Tags)
	}
}

func TestTagRuleParserParse_FlushFinalRuleOnlyWhenNeeded(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantRules int
	}{
		{
			name: "rule with semicolon - no extra rule",
			input: `python/
@ tag1
;`,
			wantRules: 1,
		},
		{
			name: "rule without semicolon - flush creates rule",
			input: `python/
@ tag1`,
			wantRules: 1,
		},
		{
			name:      "empty input - no rules",
			input:     "",
			wantRules: 0,
		},
		{
			name:      "only comments - no rules",
			input:     "# just a comment\n# another comment",
			wantRules: 0,
		},
		{
			name:      "only tag definitions - no rules",
			input:     "! tag1 tag2 tag3",
			wantRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &TagRuleParser{}
			err := p.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			if len(p.rules) != tt.wantRules {
				t.Errorf("rules count = %d, want %d", len(p.rules), tt.wantRules)
			}
		})
	}
}
