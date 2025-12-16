package raycicmd

import (
	"reflect"
	"regexp"
	"testing"
)

func TestGlobToRegexp(t *testing.T) {
	for _, test := range []struct {
		pattern string
		want    string
	}{{
		pattern: "python/*.py",
		want:    "^python/.*\\.py$",
	}, {
		pattern: "python/?.py",
		want:    "^python/.\\.py$",
	}} {
		got, err := globToRegexp(test.pattern)
		if err != nil {
			t.Errorf("globToRegexp(%v): %v", test.pattern, err)
		}
		if got.String() != test.want {
			t.Errorf(
				"globToRegexp(%v): got %v, want %v",
				test.pattern,
				got.String(),
				test.want,
			)
		}
	}
}

func TestTagRuleMatch(t *testing.T) {
	re, err := globToRegexp("python/*.py")
	if err != nil {
		t.Fatalf("globToRegexp(%v): %v", "python/*.py", err)
	}
	rule := &TagRule{
		Tags:     []string{"hit"},
		Lineno:   1,
		Dirs:     []string{"fancy"},
		Files:    []string{"file.txt"},
		Patterns: []*regexp.Regexp{re},
	}

	for _, test := range []struct {
		changedFilePath string
		want            bool
	}{{
		changedFilePath: "fancy",
		want:            true,
	}, {
		changedFilePath: "fancy/a.md",
		want:            true,
	}, {
		changedFilePath: "python/a.py",
		want:            true,
	}, {
		changedFilePath: "python/subdir/a.py",
		want:            true,
	}, {
		changedFilePath: "file.txt",
		want:            true,
	}, {
		changedFilePath: "fancy_file.txt",
		want:            false,
	}, {
		changedFilePath: "python/a.txt",
		want:            false,
	}} {
		got := rule.Match(test.changedFilePath)
		if got != test.want {
			t.Errorf(
				"match(%v, %v): got %v, want %v",
				rule,
				test.changedFilePath,
				got,
				test.want,
			)
		}
	}

	skipRule := &TagRule{
		Tags:   []string{},
		Lineno: 1,
		Files:  []string{"skip.txt"},
	}
	for _, test := range []struct {
		changedFilePath string
		want            bool
	}{{
		changedFilePath: "skip.txt",
		want:            true,
	}, {
		changedFilePath: "not_match",
		want:            false,
	}} {
		got := skipRule.Match(test.changedFilePath)
		if got != test.want {
			t.Errorf(
				"match(%v, %v): got %v, want %v",
				skipRule,
				test.changedFilePath,
				got,
				test.want,
			)
		}
	}
}

func TestTagRuleMatchTags(t *testing.T) {
	re, err := globToRegexp("python/*.py")
	if err != nil {
		t.Fatalf("globToRegexp(%v): %v", "python/*.py", err)
	}
	rule := &TagRule{
		Tags:     []string{"hit"},
		Lineno:   1,
		Dirs:     []string{"fancy"},
		Files:    []string{"file.txt"},
		Patterns: []*regexp.Regexp{re},
	}

	for _, test := range []struct {
		changedFilePath string
		want            []string
		wantBool        bool
	}{{
		changedFilePath: "fancy",
		want:            []string{"hit"},
		wantBool:        true,
	}, {
		changedFilePath: "not_match",
		want:            []string{},
		wantBool:        false,
	}} {
		got, gotBool := rule.MatchTags(test.changedFilePath)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"matchTags(%v, %v): got %v, want %v",
				rule,
				test.changedFilePath,
				got,
				test.want,
			)
		}
		if gotBool != test.wantBool {
			t.Errorf(
				"matchTags(%v, %v): gotBool %v, wantBool %v",
				rule,
				test.changedFilePath,
				gotBool,
				test.wantBool,
			)
		}
	}

	skipRule := &TagRule{
		Tags:   []string{},
		Lineno: 1,
		Files:  []string{"skip.txt"},
	}
	for _, test := range []struct {
		changedFilePath string
		want            []string
		wantBool        bool
	}{{
		changedFilePath: "skip.txt",
		want:            []string{},
		wantBool:        true,
	}, {
		changedFilePath: "not_match",
		want:            []string{},
		wantBool:        false,
	}} {
		got, gotBool := skipRule.MatchTags(test.changedFilePath)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"matchTags(%v, %v): got %v, want %v",
				skipRule,
				test.changedFilePath,
				got,
				test.want,
			)
		}
		if gotBool != test.wantBool {
			t.Errorf(
				"matchTags(%v, %v): gotBool %v, wantBool %v",
				skipRule,
				test.changedFilePath,
				gotBool,
				test.wantBool,
			)
		}
	}
}

func TestTagRuleSetValidateRules(t *testing.T) {
	tests := []struct {
		name    string
		set     *TagRuleSet
		wantErr bool
	}{
		{
			name: "valid tag",
			set: &TagRuleSet{
				tagDefs: map[string]struct{}{"hit": {}},
				rules:   []*TagRule{{Tags: []string{"hit"}, Lineno: 1}},
			},
			wantErr: false,
		},
		{
			name: "undeclared tag",
			set: &TagRuleSet{
				tagDefs: map[string]struct{}{},
				rules:   []*TagRule{{Tags: []string{"i_dont_exist"}, Lineno: 1}},
			},
			wantErr: true,
		},
		{
			name: "rule with no tags",
			set: &TagRuleSet{
				tagDefs: map[string]struct{}{},
				rules:   []*TagRule{{Tags: []string{}, Lineno: 1}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.set.ValidateRules(); (err != nil) != tt.wantErr {
				t.Errorf("TagRuleSet.ValidateRules() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTagRuleSetMatchTags(t *testing.T) {
	set := &TagRuleSet{
		tagDefs: map[string]struct{}{"tag-hit": {}, "tag-hit-2": {}},
		rules: []*TagRule{
			{Tags: []string{"tag-hit"}, Lineno: 1, Files: []string{"fancy.txt"}},
			{Tags: []string{"tag-hit-2"}, Lineno: 2, Dirs: []string{"fancy"}},
			{Tags: []string{}, Lineno: 3, Files: []string{"empty.txt"}},
		},
	}

	tests := []struct {
		name            string
		changedFilePath string
		want            []string
		wantBool        bool
	}{
		{
			name:            "match first rule by file",
			changedFilePath: "fancy.txt",
			want:            []string{"tag-hit"},
			wantBool:        true,
		},
		{
			name:            "match second rule by dir",
			changedFilePath: "fancy/other.txt",
			want:            []string{"tag-hit-2"},
			wantBool:        true,
		},
		{
			name:            "match rule with no tags",
			changedFilePath: "empty.txt",
			want:            []string{},
			wantBool:        true,
		},
		{
			name:            "no match",
			changedFilePath: "not_match",
			want:            []string{},
			wantBool:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotBool := set.MatchTags(tt.changedFilePath)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchTags() got = %v, want %v", got, tt.want)
			}
			if gotBool != tt.wantBool {
				t.Errorf("MatchTags() gotBool = %v, want %v", gotBool, tt.wantBool)
			}
		})
	}
}

func TestTagRuleSetMatchTags_Fallthrough(t *testing.T) {
	set := &TagRuleSet{
		tagDefs: map[string]struct{}{"always": {}, "lint": {}, "python": {}, "ml": {}},
		rules: []*TagRule{
			// Fallthrough rule for python/ - matches python files and continues
			{Tags: []string{"always", "lint"}, Fallthrough: true,
				Lineno: 1, Dirs: []string{"python", "ml"}},
			// Python directory rule (more specific)
			{Tags: []string{"python"}, Lineno: 2, Dirs: []string{"python"}},
			// ML directory rule
			{Tags: []string{"ml"}, Lineno: 3, Dirs: []string{"ml"}},
		},
	}

	tests := []struct {
		name            string
		changedFilePath string
		want            []string
		wantBool        bool
	}{
		{
			name:            "fallthrough accumulates tags from python",
			changedFilePath: "python/foo.py",
			want:            []string{"always", "lint", "python"},
			wantBool:        true,
		},
		{
			name:            "fallthrough accumulates tags from ml",
			changedFilePath: "ml/model.py",
			want:            []string{"always", "lint", "ml"},
			wantBool:        true,
		},
		{
			name:            "no match for unrelated file",
			changedFilePath: "other/file.txt",
			want:            []string{},
			wantBool:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotBool := set.MatchTags(tt.changedFilePath)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchTags() got = %v, want %v", got, tt.want)
			}
			if gotBool != tt.wantBool {
				t.Errorf("MatchTags() gotBool = %v, want %v", gotBool, tt.wantBool)
			}
		})
	}
}

func TestTagRuleSetMatchTags_DefaultRules(t *testing.T) {
	set := &TagRuleSet{
		tagDefs: map[string]struct{}{"python": {}, "fallback": {}, "catchall": {}},
		rules: []*TagRule{
			// Python directory rule
			{Tags: []string{"python"}, Lineno: 1, Dirs: []string{"python"}},
		},
		defaultRules: []*TagRule{
			// Default catch-all rule
			{Tags: []string{"fallback", "catchall"}, Lineno: 2, Default: true},
		},
	}

	tests := []struct {
		name            string
		changedFilePath string
		want            []string
		wantBool        bool
	}{
		{
			name:            "matched file uses matched rule",
			changedFilePath: "python/foo.py",
			want:            []string{"python"},
			wantBool:        true,
		},
		{
			name:            "unmatched file uses default rules",
			changedFilePath: "other/file.txt",
			want:            []string{"fallback", "catchall"},
			wantBool:        false, // matched=false when default rules are used
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotBool := set.MatchTags(tt.changedFilePath)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchTags() got = %v, want %v", got, tt.want)
			}
			if gotBool != tt.wantBool {
				t.Errorf("MatchTags() gotBool = %v, want %v", gotBool, tt.wantBool)
			}
		})
	}
}
