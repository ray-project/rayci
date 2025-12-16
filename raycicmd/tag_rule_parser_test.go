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
				t.Errorf(
					"sanitizeLine(%q) = %q, want %q",
					tt.input,
					got,
					tt.want,
				)
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
			name:    "tag definition after rule should error",
			input:   strings.Join([]string{"python/", "@ sometag", ";", "! late_tag"}, "\n"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseTagRuleConfig(tt.input)

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

			if !reflect.DeepEqual(cfg.TagDefs, tt.wantTagDefs) {
				t.Errorf(
					"Parse() tagDefs = %v, want %v",
					cfg.TagDefs,
					tt.wantTagDefs,
				)
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
			name:      "single rule with directory",
			input:     strings.Join([]string{"! mytag", "python/", "@ mytag", ";"}, "\n"),
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if want := []string{"python"}; !reflect.DeepEqual(
					rules[0].Dirs,
					want,
				) {
					t.Errorf("got Dirs=%v, want %v", rules[0].Dirs, want)
				}
				if want := []string{"mytag"}; !reflect.DeepEqual(
					rules[0].Tags,
					want,
				) {
					t.Errorf("got Tags=%v, want %v", rules[0].Tags, want)
				}
			},
		},
		{
			name:      "single rule with file",
			input:     strings.Join([]string{"! mytag", "README.md", "@ mytag", ";"}, "\n"),
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if len(rules[0].Files) != 1 ||
					rules[0].Files[0] != "README.md" {
					t.Errorf(
						"got Files=%v, want [README.md]",
						rules[0].Files,
					)
				}
			},
		},
		{
			name:      "single rule with pattern",
			input:     strings.Join([]string{"! mytag", "python/*.py", "@ mytag", ";"}, "\n"),
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if len(rules[0].Patterns) != 1 {
					t.Errorf(
						"got %d patterns, want 1",
						len(rules[0].Patterns),
					)
				}
				// Verify the pattern matches expected files
				if !rules[0].Patterns[0].MatchString("python/test.py") {
					t.Errorf("pattern should match python/test.py")
				}
			},
		},
		{
			name:      "multiple rules",
			input:     strings.Join([]string{"! tag1 tag2", "python/", "@ tag1", ";", "golang/", "@ tag2", ";"}, "\n"),
			wantRules: 2,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if rules[0].Dirs[0] != "python" {
					t.Errorf(
						"first rule Dirs = %v, want [python]",
						rules[0].Dirs,
					)
				}
				if rules[1].Dirs[0] != "golang" {
					t.Errorf(
						"second rule Dirs = %v, want [golang]",
						rules[1].Dirs,
					)
				}
			},
		},
		{
			name:      "rule without semicolon at end",
			input:     strings.Join([]string{"! mytag", "python/", "@ mytag"}, "\n"),
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if len(rules[0].Dirs) != 1 || rules[0].Dirs[0] != "python" {
					t.Errorf("got Dirs=%v, want [python]", rules[0].Dirs)
				}
			},
		},
		{
			name:      "skip rule (no tags)",
			input:     strings.Join([]string{".git/", ";"}, "\n"),
			wantRules: 1,
			checkRule: func(t *testing.T, rules []*TagRule) {
				if len(rules[0].Tags) != 0 {
					t.Errorf(
						"got Tags=%v, want empty for skip rule",
						rules[0].Tags,
					)
				}
				if rules[0].Dirs[0] != ".git" {
					t.Errorf("got Dirs=%v, want [.git]", rules[0].Dirs)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseTagRuleConfig(tt.input)

			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			if len(cfg.Rules) != tt.wantRules {
				t.Errorf(
					"Parse() rules count = %d, want %d",
					len(cfg.Rules),
					tt.wantRules,
				)
				return
			}

			if tt.checkRule != nil {
				tt.checkRule(t, cfg.Rules)
			}
		})
	}
}

func TestTagRuleParserParse_MultiplePaths(t *testing.T) {
	input := strings.Join([]string{
		"! mytag",
		"python/",
		"golang/",
		"src/main.go",
		"*.md",
		"@ mytag",
		";",
	}, "\n")

	cfg, err := ParseTagRuleConfig(input)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if len(cfg.Rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(cfg.Rules))
	}

	rule := cfg.Rules[0]

	wantDirs := []string{"python", "golang"}
	if !reflect.DeepEqual(rule.Dirs, wantDirs) {
		t.Errorf("got Dirs=%v, want %v", rule.Dirs, wantDirs)
	}

	wantFiles := []string{"src/main.go"}
	if !reflect.DeepEqual(rule.Files, wantFiles) {
		t.Errorf("got Files=%v, want %v", rule.Files, wantFiles)
	}

	if len(rule.Patterns) != 1 {
		t.Errorf("got %d patterns, want 1", len(rule.Patterns))
	}
}

func TestTagRuleParserParse_MultipleTags(t *testing.T) {
	input := strings.Join([]string{"! tag1 tag2 tag3", "python/", "@ tag1 tag2", "@ tag3", ";"}, "\n")

	cfg, err := ParseTagRuleConfig(input)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if len(cfg.Rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(cfg.Rules))
	}

	wantTags := []string{"tag1", "tag2", "tag3"}
	sort.Strings(cfg.Rules[0].Tags)
	sort.Strings(wantTags)
	if !reflect.DeepEqual(cfg.Rules[0].Tags, wantTags) {
		t.Errorf("got Tags=%v, want %v", cfg.Rules[0].Tags, wantTags)
	}
}

func TestTagRuleParserParse_Comments(t *testing.T) {
	input := strings.Join([]string{
		"# This is a comment at the start",
		"! tag1 tag2 # tag definitions",
		"",
		"# Another comment",
		"python/ # Directory to match",
		"@ tag1 # Tag assignment",
		"; # Rule separator",
	}, "\n")

	cfg, err := ParseTagRuleConfig(input)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if len(cfg.TagDefs) != 2 {
		t.Errorf(
			"got %d tag definitions %v, want 2",
			len(cfg.TagDefs),
			cfg.TagDefs,
		)
	}

	if len(cfg.Rules) != 1 {
		t.Errorf("got %d rules, want 1", len(cfg.Rules))
	}
}

func TestTagRuleParserParse_EmptyLines(t *testing.T) {
	input := strings.Join([]string{"! tag1", "", "python/", "", "@ tag1", "", ";"}, "\n")

	cfg, err := ParseTagRuleConfig(input)
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if len(cfg.Rules) != 1 {
		t.Errorf("got %d rules, want 1", len(cfg.Rules))
	}

	if len(cfg.Rules[0].Dirs) != 1 || cfg.Rules[0].Dirs[0] != "python" {
		t.Errorf("got Dirs=%v, want [python]", cfg.Rules[0].Dirs)
	}
}

func TestTagRuleParserParse_Errors(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError string
	}{
		{
			name:      "tag definition after rule starts",
			input:     strings.Join([]string{"python/", "! tag1"}, "\n"),
			wantError: "tag must be declared at file start",
		},
		{
			name:      "tokens after semicolon",
			input:     strings.Join([]string{"python/", "; extra_stuff"}, "\n"),
			wantError: "unexpected tokens after semicolon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTagRuleConfig(tt.input)

			if err == nil {
				t.Errorf(
					"got no error, want error containing %q",
					tt.wantError,
				)
				return
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf(
					"Parse() error = %q, want error containing %q",
					err.Error(),
					tt.wantError,
				)
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
			cfg, err := ParseTagRuleConfig(tt.input)
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			// Should have one rule from flushFinalRule
			if len(cfg.Rules) != 1 {
				t.Fatalf("got %d rules, want 1", len(cfg.Rules))
			}

			rule := cfg.Rules[0]

			if tt.wantDirs != nil &&
				!reflect.DeepEqual(rule.Dirs, tt.wantDirs) {
				t.Errorf("Dirs = %v, want %v", rule.Dirs, tt.wantDirs)
			}

			if tt.wantFiles != nil &&
				!reflect.DeepEqual(rule.Files, tt.wantFiles) {
				t.Errorf("Files = %v, want %v", rule.Files, tt.wantFiles)
			}

			if tt.wantPattern && len(rule.Patterns) == 0 {
				t.Errorf("got no patterns, want 1")
			}

			if !tt.wantPattern && len(rule.Patterns) > 0 {
				t.Errorf("got %d patterns, want 0", len(rule.Patterns))
			}
		})
	}
}

func TestTagRuleParserParse_LineNumbers(t *testing.T) {
	input := strings.Join([]string{"! tag1", "python/", "@ tag1", ";", "golang/", "@ tag1", ";"}, "\n")

	cfg, err := ParseTagRuleConfig(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(cfg.Rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(cfg.Rules))
	}

	// First rule ends at line 4 (the ;)
	if cfg.Rules[0].Lineno != 4 {
		t.Errorf("first rule Lineno = %d, want 4", cfg.Rules[0].Lineno)
	}

	// Second rule ends at line 7 (the ;)
	if cfg.Rules[1].Lineno != 7 {
		t.Errorf("second rule Lineno = %d, want 7", cfg.Rules[1].Lineno)
	}
}

func TestTagRuleParserParse_RealWorldExample(t *testing.T) {
	input := strings.Join([]string{
		"# Conditional testing rules",
		"! python ml data serve train tune",
		"",
		"# Skip hidden files",
		".git/",
		".github/",
		";",
		"",
		"# Python core",
		"python/ray/",
		"@ python",
		";",
		"",
		"# ML libraries",
		"python/ray/train/",
		"python/ray/tune/",
		"@ ml train tune",
		";",
		"",
		"# Data processing",
		"python/ray/data/",
		"@ data",
		";",
	}, "\n")

	cfg, err := ParseTagRuleConfig(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Should have tag definitions
	wantTagDefs := []string{
		"python",
		"ml",
		"data",
		"serve",
		"train",
		"tune",
	}
	sort.Strings(cfg.TagDefs)
	sort.Strings(wantTagDefs)
	if !reflect.DeepEqual(cfg.TagDefs, wantTagDefs) {
		t.Errorf("got tagDefs=%v, want %v", cfg.TagDefs, wantTagDefs)
	}

	// Should have 4 rules
	if len(cfg.Rules) != 4 {
		t.Errorf("got %d rules, want 4", len(cfg.Rules))
	}

	// First rule should be a skip rule (no tags)
	if len(cfg.Rules[0].Tags) != 0 {
		t.Errorf(
			"first rule should have no tags (skip rule), got %v",
			cfg.Rules[0].Tags,
		)
	}

	// Second rule should have python tag
	if len(cfg.Rules[1].Tags) != 1 || cfg.Rules[1].Tags[0] != "python" {
		t.Errorf("second rule Tags = %v, want [python]", cfg.Rules[1].Tags)
	}
}

func TestTagRuleParserParse_FlushFinalRuleOnlyWhenNeeded(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantRules int
	}{
		{
			name:      "rule with semicolon - no extra rule",
			input:     strings.Join([]string{"python/", "@ tag1", ";"}, "\n"),
			wantRules: 1,
		},
		{
			name:      "rule without semicolon - flush creates rule",
			input:     strings.Join([]string{"python/", "@ tag1"}, "\n"),
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
			cfg, err := ParseTagRuleConfig(tt.input)
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			if len(cfg.Rules) != tt.wantRules {
				t.Errorf(
					"rules count = %d, want %d",
					len(cfg.Rules),
					tt.wantRules,
				)
			}
		})
	}
}

func TestTagRuleParserParse_FallthroughAndDefaultDirectives(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantRules        int // regular rules count
		wantDefaultRules int // default rules count (Default=true)
		wantTagDefs      []string
		// Expected regular rules (non-default)
		wantRegularTags [][]string
		// Expected default rules (Default=true)
		wantDefaultTags [][]string
	}{
		{
			name:             "fallthrough directive only",
			input:            "! tag1\n\\fallthrough\n@ tag1\n;",
			wantRules:        1, // fallthrough without default goes to regular rules
			wantDefaultRules: 0,
			wantRegularTags:  [][]string{{"tag1"}},
			wantDefaultTags:  nil,
			wantTagDefs:      []string{"tag1"},
		},
		{
			name:             "default directive only",
			input:            "! tag1\n\\default\n@ tag1\n;",
			wantRules:        0,
			wantDefaultRules: 1, // default goes to DefaultRules
			wantRegularTags:  nil,
			wantDefaultTags:  [][]string{{"tag1"}},
			wantTagDefs:      []string{"tag1"},
		},
		{
			name:             "multiple rules with fallthrough then default",
			input:            "! tag1 tag2\n\\fallthrough\n@ tag1\n;\n\\default\n@ tag2\n;",
			wantRules:        1, // fallthrough rule
			wantDefaultRules: 1, // default rule
			wantRegularTags:  [][]string{{"tag1"}},
			wantDefaultTags:  [][]string{{"tag2"}},
			wantTagDefs:      []string{"tag1", "tag2"},
		},
		{
			name:             "default rule at end for catch-all",
			input:            "! tag1 fallback\npython/\n@ tag1\n;\n\\default\n@ fallback\n;",
			wantRules:        1, // python/ rule
			wantDefaultRules: 1, // default catch-all rule
			wantRegularTags:  [][]string{{"tag1"}},
			wantDefaultTags:  [][]string{{"fallback"}},
			wantTagDefs:      []string{"tag1", "fallback"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseTagRuleConfig(tt.input)
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			if len(cfg.Rules) != tt.wantRules {
				t.Fatalf("got %d regular rules, want %d", len(cfg.Rules), tt.wantRules)
			}

			if len(cfg.DefaultRules) != tt.wantDefaultRules {
				t.Fatalf("got %d default rules, want %d", len(cfg.DefaultRules), tt.wantDefaultRules)
			}

			// Check regular rules
			for i, rule := range cfg.Rules {
				if !reflect.DeepEqual(rule.Tags, tt.wantRegularTags[i]) {
					t.Errorf("regular rule %d: Tags = %v, want %v", i, rule.Tags, tt.wantRegularTags[i])
				}
			}

			// Check default rules
			for i, rule := range cfg.DefaultRules {
				if !rule.Default {
					t.Errorf("default rule %d: expected Default=true, got Default=%v", i, rule.Default)
				}
				if !reflect.DeepEqual(rule.Tags, tt.wantDefaultTags[i]) {
					t.Errorf("default rule %d: Tags = %v, want %v", i, rule.Tags, tt.wantDefaultTags[i])
				}
			}

			if tt.wantTagDefs != nil && !reflect.DeepEqual(cfg.TagDefs, tt.wantTagDefs) {
				t.Errorf("TagDefs = %v, want %v", cfg.TagDefs, tt.wantTagDefs)
			}
		})
	}
}

func TestTagRuleParserParse_DefaultAndFallthroughError(t *testing.T) {
	// A rule cannot have both \default and \fallthrough
	inputs := []string{
		"! tag1\n\\fallthrough\n\\default\n@ tag1\n;",
		"! tag1\n\\default\n\\fallthrough\n@ tag1\n;",
		"! tag1\n\\fallthrough\n\\default\n@ tag1", // without semicolon
	}

	for _, input := range inputs {
		_, err := ParseTagRuleConfig(input)
		if err == nil {
			t.Errorf("expected error for input %q, got nil", input)
		}
	}
}

func TestTagRuleParserParse_DefaultMustBeLast(t *testing.T) {
	// Non-default rules cannot appear after default rules
	errorInputs := []string{
		// default rule followed by non-default rule
		"! tag1 tag2\n\\default\n@ tag1\n;\npython/\n@ tag2\n;",
		// default rule followed by non-default rule (without semicolon)
		"! tag1 tag2\n\\default\n@ tag1\n;\npython/\n@ tag2",
	}

	for _, input := range errorInputs {
		_, err := ParseTagRuleConfig(input)
		if err == nil {
			t.Errorf("expected error for input %q, got nil", input)
		}
	}

	// Valid: multiple default rules at the end
	validInputs := []string{
		// non-default followed by default
		"! tag1 tag2\npython/\n@ tag1\n;\n\\default\n@ tag2\n;",
		// multiple default rules at the end
		"! tag1 tag2 tag3\npython/\n@ tag1\n;\n\\default\n@ tag2\n;\n\\default\n@ tag3\n;",
	}

	for _, input := range validInputs {
		_, err := ParseTagRuleConfig(input)
		if err != nil {
			t.Errorf("unexpected error for input %q: %v", input, err)
		}
	}
}

func TestTagRuleParserParse_UnknownDirective(t *testing.T) {
	_, err := ParseTagRuleConfig("! tag1\n\\unknown\n@ tag1\n;")
	if err == nil {
		t.Error("expected error for unknown directive, got nil")
	}
}
