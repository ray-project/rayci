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
			t.Errorf("globToRegexp(%v): got %v, want %v", test.pattern, got.String(), test.want)
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
			t.Errorf("match(%v, %v): got %v, want %v", rule, test.changedFilePath, got, test.want)
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
			t.Errorf("match(%v, %v): got %v, want %v", skipRule, test.changedFilePath, got, test.want)
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
			t.Errorf("matchTags(%v, %v): got %v, want %v", rule, test.changedFilePath, got, test.want)
		}
		if gotBool != test.wantBool {
			t.Errorf("matchTags(%v, %v): gotBool %v, wantBool %v", rule, test.changedFilePath, gotBool, test.wantBool)
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
			t.Errorf("matchTags(%v, %v): got %v, want %v", skipRule, test.changedFilePath, got, test.want)
		}
		if gotBool != test.wantBool {
			t.Errorf("matchTags(%v, %v): gotBool %v, wantBool %v", skipRule, test.changedFilePath, gotBool, test.wantBool)
		}
	}
}

func TestTagRuleSetValidateRules(t *testing.T) {
	for _, test := range []struct {
		set     *TagRuleSet
		wantErr bool
	}{{
		set: &TagRuleSet{
			tagDefs: map[string]struct{}{"hit": {}},
			rules:   []*TagRule{{Tags: []string{"hit"}, Lineno: 1}},
		},
		wantErr: false,
	}, {
		set: &TagRuleSet{
			tagDefs: map[string]struct{}{"hit": {}},
			rules:   []*TagRule{{Tags: []string{"i_dont_exist"}, Lineno: 1}},
		},
		wantErr: true,
	}} {
		err := test.set.ValidateRules()
		if test.wantErr && err == nil {
			t.Errorf("ValidateRules(): got nil, want error")
		}
		if !test.wantErr && err != nil {
			t.Errorf("ValidateRules(): got error %v, want nil", err)
		}
	}
}

func TestTagRuleSetMatchTags(t *testing.T) {
	set := &TagRuleSet{
		tagDefs: map[string]struct{}{"tag-hit": {}},
		rules:   []*TagRule{{Tags: []string{"tag-hit"}, Lineno: 1, Files: []string{"fancy.txt"}}},
	}
	for _, test := range []struct {
		changedFilePath string
		want            []string
		wantBool        bool
	}{{
		changedFilePath: "fancy.txt",
		want:            []string{"tag-hit"},
		wantBool:        true,
	}, {
		changedFilePath: "not_match",
		want:            []string{},
		wantBool:        false,
	}} {
		got, gotBool := set.MatchTags(test.changedFilePath)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("matchTags(%v, %v): got %v, want %v", set, test.changedFilePath, got, test.want)
		}
		if gotBool != test.wantBool {
			t.Errorf("matchTags(%v, %v): gotBool %v, wantBool %v", set, test.changedFilePath, gotBool, test.wantBool)
		}
	}
}
