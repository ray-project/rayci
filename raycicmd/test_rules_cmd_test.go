package raycicmd

import (
	"bytes"
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

func newTestRuleSet(t *testing.T, rulesContent string) *TagRuleSet {
	t.Helper()
	cfg, err := ParseTagRuleConfig(rulesContent)
	if err != nil {
		t.Fatalf("ParseTagRuleConfig() error: %v", err)
	}

	ruleSet := &TagRuleSet{
		tagDefs:      make(map[string]struct{}),
		rules:        cfg.Rules,
		defaultRules: cfg.DefaultRules,
	}
	for _, tag := range cfg.TagDefs {
		ruleSet.tagDefs[tag] = struct{}{}
	}
	return ruleSet
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
		"\\fallthrough",
		"@ always lint",
		";",
		"",
		"python/ray/data/",
		"@ data",
		";",
	}, "\n")

	ruleSet := newTestRuleSet(t, rulesContent)

	testCases := []ruleTestCase{
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
		"\\fallthrough",
		"@ always lint",
		";",
		"",
		"python/ray/data/",
		"@ data",
		";",
	}, "\n")

	ruleSet := newTestRuleSet(t, rulesContent)

	testCases := []ruleTestCase{
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

func TestPrintFailures_Format(t *testing.T) {
	failures := []testCaseResult{
		{
			File:    "fail.py",
			Lineno:  2,
			Extra:   []string{"extra1", "extra2"},
			Missing: []string{"missing1"},
		},
	}

	var buf bytes.Buffer
	printFailures(&buf, failures)
	output := buf.String()

	if !strings.Contains(output, "FAIL: fail.py (line 2)") {
		t.Errorf("output should contain file and line, got: %s", output)
	}
	if !strings.Contains(output, "+extra1 (unexpected)") {
		t.Errorf("output should show extra tags, got: %s", output)
	}
	if !strings.Contains(output, "-missing1 (missing)") {
		t.Errorf("output should show missing tags, got: %s", output)
	}
}

func TestTestRulesMain_Success_Silent(t *testing.T) {
	dir := t.TempDir()

	rulesFile := filepath.Join(dir, "rules.txt")
	rulesContent := strings.Join([]string{
		"! always lint data ml",
		"",
		"\\fallthrough",
		"@ always lint",
		";",
		"",
		"python/ray/data/",
		"@ data ml",
		";",
	}, "\n")
	if err := os.WriteFile(rulesFile, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	testsFile := filepath.Join(dir, "tests.txt")
	testsContent := strings.Join([]string{
		"python/ray/data/__init__.py: always lint data ml",
		"python/ray/data/dataset.py: always data lint ml",
	}, "\n")
	if err := os.WriteFile(testsFile, []byte(testsContent), 0644); err != nil {
		t.Fatalf("write tests: %v", err)
	}

	envs := newEnvsMap(map[string]string{
		"RAYCI_TEST_RULE_FILES": rulesFile,
	})

	var buf bytes.Buffer
	err := TestRulesMain([]string{testsFile}, envs, &buf)

	if err != nil {
		t.Errorf("TestRulesMain() error: %v", err)
	}

	// Should be silent on success
	if buf.String() != "" {
		t.Errorf("output should be empty on success, got: %q", buf.String())
	}
}

func TestTestRulesMain_Failure_Output(t *testing.T) {
	dir := t.TempDir()

	rulesFile := filepath.Join(dir, "rules.txt")
	rulesContent := strings.Join([]string{
		"! always lint data",
		"",
		"\\fallthrough",
		"@ always lint",
		";",
		"",
		"python/",
		"@ data",
		";",
	}, "\n")
	if err := os.WriteFile(rulesFile, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	testsFile := filepath.Join(dir, "tests.txt")
	testsContent := "python/foo.py: always lint wrongtag"
	if err := os.WriteFile(testsFile, []byte(testsContent), 0644); err != nil {
		t.Fatalf("write tests: %v", err)
	}

	envs := newEnvsMap(map[string]string{
		"RAYCI_TEST_RULE_FILES": rulesFile,
	})

	var buf bytes.Buffer
	err := TestRulesMain([]string{testsFile}, envs, &buf)

	if err == nil {
		t.Error("TestRulesMain() should return error for failing tests")
	}

	output := buf.String()
	if !strings.Contains(output, "FAIL:") {
		t.Errorf("output should contain FAIL, got: %s", output)
	}
	if !strings.Contains(output, "+data (unexpected)") {
		t.Errorf("output should show extra tag, got: %s", output)
	}
	if !strings.Contains(output, "-wrongtag (missing)") {
		t.Errorf("output should show missing tag, got: %s", output)
	}
}

func TestTestRulesMain_MissingTestsArg(t *testing.T) {
	err := TestRulesMain([]string{}, newEnvsMap(nil), nil)

	if err == nil {
		t.Error("TestRulesMain() should return error when tests file is missing")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "usage:") {
		t.Errorf("error should show usage, got: %v", err)
	}
	if !strings.Contains(errMsg, "Test file format:") {
		t.Errorf("error should show file format, got: %v", err)
	}
}

func TestTestRulesMain_MissingEnvVar(t *testing.T) {
	dir := t.TempDir()
	testsFile := filepath.Join(dir, "tests.txt")
	if err := os.WriteFile(testsFile, []byte("foo.py: tag"), 0644); err != nil {
		t.Fatalf("write tests: %v", err)
	}

	err := TestRulesMain([]string{testsFile}, newEnvsMap(nil), nil)

	if err == nil {
		t.Error("TestRulesMain() should return error when env var is missing")
	}
	if !strings.Contains(err.Error(), "RAYCI_TEST_RULE_FILES") {
		t.Errorf("error should mention env var, got: %v", err)
	}
}

func TestTestRulesMain_MultipleRulesFiles(t *testing.T) {
	dir := t.TempDir()

	rules1 := filepath.Join(dir, "rules1.txt")
	rules1Content := strings.Join([]string{
		"! always lint tag1",
		"",
		"\\fallthrough",
		"@ always lint",
		";",
		"",
		"dir1/",
		"@ tag1",
		";",
	}, "\n")
	if err := os.WriteFile(rules1, []byte(rules1Content), 0644); err != nil {
		t.Fatalf("write rules1: %v", err)
	}

	rules2 := filepath.Join(dir, "rules2.txt")
	rules2Content := strings.Join([]string{
		"! tag2",
		"",
		"dir2/",
		"@ tag2",
		";",
	}, "\n")
	if err := os.WriteFile(rules2, []byte(rules2Content), 0644); err != nil {
		t.Fatalf("write rules2: %v", err)
	}

	testsFile := filepath.Join(dir, "tests.txt")
	testsContent := strings.Join([]string{
		"dir1/foo.py: always lint tag1",
		"dir2/bar.py: always lint tag2",
	}, "\n")
	if err := os.WriteFile(testsFile, []byte(testsContent), 0644); err != nil {
		t.Fatalf("write tests: %v", err)
	}

	envs := newEnvsMap(map[string]string{
		"RAYCI_TEST_RULE_FILES": rules1 + "," + rules2,
	})

	var buf bytes.Buffer
	err := TestRulesMain([]string{testsFile}, envs, &buf)

	if err != nil {
		t.Errorf("TestRulesMain() error: %v", err)
	}
}
