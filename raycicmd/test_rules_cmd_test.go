package raycicmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseTestCases(t *testing.T) {
	content := strings.Join([]string{
		"# Comment line",
		"python/ray/data/__init__.py: always lint data ml",
		"",
		"# Another file",
		"python/setup.py: always lint ml train python dashboard",
		"",
		"# Single tag",
		"README.md: lint",
	}, "\n")

	cases, err := parseTestCases(content)
	if err != nil {
		t.Fatalf("parseTestCases() error: %v", err)
	}

	if len(cases) != 3 {
		t.Fatalf("parseTestCases() got %d cases, want 3", len(cases))
	}

	// First case
	if cases[0].File != "python/ray/data/__init__.py" {
		t.Errorf("case[0].File = %q, want %q", cases[0].File, "python/ray/data/__init__.py")
	}
	wantTags := []string{"always", "lint", "data", "ml"}
	if !reflect.DeepEqual(cases[0].Tags, wantTags) {
		t.Errorf("case[0].Tags = %v, want %v", cases[0].Tags, wantTags)
	}

	// Second case
	if cases[1].File != "python/setup.py" {
		t.Errorf("case[1].File = %q, want %q", cases[1].File, "python/setup.py")
	}
	wantTags = []string{"always", "lint", "ml", "train", "python", "dashboard"}
	if !reflect.DeepEqual(cases[1].Tags, wantTags) {
		t.Errorf("case[1].Tags = %v, want %v", cases[1].Tags, wantTags)
	}

	// Third case
	if cases[2].File != "README.md" {
		t.Errorf("case[2].File = %q, want %q", cases[2].File, "README.md")
	}
	wantTags = []string{"lint"}
	if !reflect.DeepEqual(cases[2].Tags, wantTags) {
		t.Errorf("case[2].Tags = %v, want %v", cases[2].Tags, wantTags)
	}
}

func TestParseTestCases_LineNumbers(t *testing.T) {
	content := strings.Join([]string{
		"# line 1",
		"# line 2",
		"python/foo.py: tag1",
		"# line 4",
		"python/bar.py: tag2",
	}, "\n")

	cases, err := parseTestCases(content)
	if err != nil {
		t.Fatalf("parseTestCases() error: %v", err)
	}

	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}

	if cases[0].Lineno != 3 {
		t.Errorf("case[0].Lineno = %d, want 3", cases[0].Lineno)
	}
	if cases[1].Lineno != 5 {
		t.Errorf("case[1].Lineno = %d, want 5", cases[1].Lineno)
	}
}

func newTestRuleSets(t *testing.T, rulesContent string) []*TagRuleSet {
	t.Helper()
	cfg, err := ParseTagRuleConfig(rulesContent)
	if err != nil {
		t.Fatalf("ParseTagRuleConfig() error: %v", err)
	}

	ruleSet := &TagRuleSet{
		tagDefs: make(map[string]struct{}),
		rules:   cfg.Rules,
	}
	for _, tag := range cfg.TagDefs {
		ruleSet.tagDefs[tag] = struct{}{}
	}
	return []*TagRuleSet{ruleSet}
}

func TestParseTestCases_InvalidFormat(t *testing.T) {
	content := "invalid line without colon"

	_, err := parseTestCases(content)
	if err == nil {
		t.Error("parseTestCases() should return error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("error should mention invalid format, got: %v", err)
	}
}

func TestDiffTags(t *testing.T) {
	got := []string{"a", "b", "c"}
	want := []string{"b", "c", "d"}

	extra, missing := diffTags(got, want)

	if !reflect.DeepEqual(extra, []string{"a"}) {
		t.Errorf("extra = %v, want [a]", extra)
	}
	if !reflect.DeepEqual(missing, []string{"d"}) {
		t.Errorf("missing = %v, want [d]", missing)
	}
}

func TestRunTestRules_AllPass(t *testing.T) {
	rulesContent := strings.Join([]string{
		"! always lint data",
		"",
		"python/ray/data/",
		"@ always lint data",
		";",
	}, "\n")

	ruleSet := newTestRuleSets(t, rulesContent)

	testCases := []*ruleTestCase{
		{File: "python/ray/data/__init__.py", Tags: []string{"always", "lint", "data"}, Lineno: 1},
		{File: "python/ray/data/dataset.py", Tags: []string{"always", "lint", "data"}, Lineno: 2},
	}

	failures := runTestRules(ruleSet, testCases)

	if len(failures) != 0 {
		t.Errorf("len(failures) = %d, want 0", len(failures))
	}
}

func TestRunTestRules_SomeFail(t *testing.T) {
	rulesContent := strings.Join([]string{
		"! always lint data",
		"",
		"python/ray/data/",
		"@ always lint data",
		";",
	}, "\n")

	ruleSet := newTestRuleSets(t, rulesContent)

	testCases := []*ruleTestCase{
		{File: "python/ray/data/__init__.py", Tags: []string{"always", "lint", "data"}, Lineno: 1},
		// This one has a typo: "daat" instead of "data"
		{File: "python/ray/data/dataset.py", Tags: []string{"always", "lint", "daat"}, Lineno: 2},
	}

	failures := runTestRules(ruleSet, testCases)

	if len(failures) != 1 {
		t.Errorf("len(failures) = %d, want 1", len(failures))
	}

	if len(failures) > 0 {
		if !reflect.DeepEqual(failures[0].Extra, []string{"data"}) {
			t.Errorf("failures[0].Extra = %v, want [data]", failures[0].Extra)
		}
		if !reflect.DeepEqual(failures[0].Missing, []string{"daat"}) {
			t.Errorf("failures[0].Missing = %v, want [daat]", failures[0].Missing)
		}
	}
}

func TestParseTestCases_EmptyFilePath(t *testing.T) {
	content := ": tag1 tag2"

	_, err := parseTestCases(content)
	if err == nil {
		t.Error("parseTestCases() should return error for empty file path")
	}
	if !strings.Contains(err.Error(), "empty file path") {
		t.Errorf("error should mention empty file path, got: %v", err)
	}
}

func TestParseTestCases_EmptyTags(t *testing.T) {
	content := "file.py:"

	cases, err := parseTestCases(content)
	if err != nil {
		t.Fatalf("parseTestCases() error: %v", err)
	}

	if len(cases) != 1 {
		t.Fatalf("got %d cases, want 1", len(cases))
	}
	if cases[0].File != "file.py" {
		t.Errorf("File = %q, want %q", cases[0].File, "file.py")
	}
	if len(cases[0].Tags) != 0 {
		t.Errorf("Tags = %v, want empty", cases[0].Tags)
	}
}

func TestDiffTags_Identical(t *testing.T) {
	got := []string{"a", "b", "c"}
	want := []string{"a", "b", "c"}

	extra, missing := diffTags(got, want)

	if len(extra) != 0 {
		t.Errorf("extra = %v, want empty", extra)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
}

func TestDiffTags_Empty(t *testing.T) {
	extra, missing := diffTags(nil, nil)

	if len(extra) != 0 {
		t.Errorf("extra = %v, want empty", extra)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
}

func TestDiffTags_AllExtra(t *testing.T) {
	got := []string{"a", "b"}
	want := []string{}

	extra, missing := diffTags(got, want)

	if !reflect.DeepEqual(extra, []string{"a", "b"}) {
		t.Errorf("extra = %v, want [a b]", extra)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
}

func TestDiffTags_AllMissing(t *testing.T) {
	got := []string{}
	want := []string{"a", "b"}

	extra, missing := diffTags(got, want)

	if len(extra) != 0 {
		t.Errorf("extra = %v, want empty", extra)
	}
	if !reflect.DeepEqual(missing, []string{"a", "b"}) {
		t.Errorf("missing = %v, want [a b]", missing)
	}
}

func TestRunTestRules_DuplicateTagsInExpected(t *testing.T) {
	rulesContent := strings.Join([]string{
		"! always lint",
		"",
		"*",
		"@ always lint",
		";",
	}, "\n")

	ruleSet := newTestRuleSets(t, rulesContent)

	testCases := []*ruleTestCase{
		{File: "foo.py", Tags: []string{"always", "lint", "always", "lint"}, Lineno: 1},
	}

	failures := runTestRules(ruleSet, testCases)

	if len(failures) != 0 {
		t.Errorf("len(failures) = %d, want 0 (duplicates should be deduplicated)", len(failures))
	}
}

func TestRunTestRules_EmptyTestCases(t *testing.T) {
	rulesContent := strings.Join([]string{
		"! always",
		"",
		"*",
		"@ always",
		";",
	}, "\n")

	ruleSet := newTestRuleSets(t, rulesContent)

	failures := runTestRules(ruleSet, nil)

	if len(failures) != 0 {
		t.Errorf("len(failures) = %d, want 0", len(failures))
	}
}

func TestRunTestRulesCmd_AutoDiscover(t *testing.T) {
	tmp := t.TempDir()
	bkDir := filepath.Join(tmp, ".buildkite")
	if err := os.MkdirAll(bkDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create rules file
	rulesContent := strings.Join([]string{
		"! mytag",
		"src/",
		"@ mytag",
		";",
	}, "\n")
	if err := os.WriteFile(filepath.Join(bkDir, "test.rules.txt"), []byte(rulesContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create companion test file
	testContent := "src/foo.py: mytag\n"
	if err := os.WriteFile(filepath.Join(bkDir, "test.rules.test.txt"), []byte(testContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir so .buildkite is found
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

	cfg := &config{}
	err := runTestRulesCmd(nil, cfg)
	if err != nil {
		t.Errorf("runTestRulesCmd() error = %v, want nil", err)
	}
}

func TestRunTestRulesCmd_ExplicitRulesFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create rules file (not in .buildkite)
	rulesContent := strings.Join([]string{
		"! explicit",
		"src/",
		"@ explicit",
		";",
	}, "\n")
	rulesPath := filepath.Join(tmp, "explicit.rules.txt")
	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create companion test file
	testContent := "src/foo.py: explicit\n"
	testPath := filepath.Join(tmp, "explicit.rules.test.txt")
	if err := os.WriteFile(testPath, []byte(testContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config{
		TestRulesFiles: []string{rulesPath},
	}
	err := runTestRulesCmd(nil, cfg)
	if err != nil {
		t.Errorf("runTestRulesCmd() error = %v, want nil", err)
	}
}

func TestRunTestRulesCmd_ExplicitTakesPrecedence(t *testing.T) {
	tmp := t.TempDir()
	bkDir := filepath.Join(tmp, ".buildkite")
	if err := os.MkdirAll(bkDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create auto-discoverable rules file that would FAIL
	autoRulesContent := strings.Join([]string{
		"! autotag",
		"src/",
		"@ autotag",
		";",
	}, "\n")
	if err := os.WriteFile(filepath.Join(bkDir, "auto.rules.txt"), []byte(autoRulesContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Companion test expects wrong tag - would fail if auto-discovered
	autoTestContent := "src/foo.py: wrongtag\n"
	if err := os.WriteFile(filepath.Join(bkDir, "auto.rules.test.txt"), []byte(autoTestContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create explicit rules file that will PASS
	explicitRulesContent := strings.Join([]string{
		"! explicit",
		"src/",
		"@ explicit",
		";",
	}, "\n")
	explicitPath := filepath.Join(tmp, "explicit.rules.txt")
	if err := os.WriteFile(explicitPath, []byte(explicitRulesContent), 0o644); err != nil {
		t.Fatal(err)
	}
	explicitTestContent := "src/foo.py: explicit\n"
	if err := os.WriteFile(filepath.Join(tmp, "explicit.rules.test.txt"), []byte(explicitTestContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp dir
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

	// With explicit TestRulesFiles, should use those and pass (not auto-discover the failing one)
	cfg := &config{
		TestRulesFiles: []string{explicitPath},
	}
	err := runTestRulesCmd(nil, cfg)
	if err != nil {
		t.Errorf("runTestRulesCmd() error = %v, want nil (explicit should take precedence)", err)
	}
}
