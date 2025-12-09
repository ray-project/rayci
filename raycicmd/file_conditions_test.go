package raycicmd

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

//go:embed data/canonical_test_rules.txt
var canonicalTestRulesBytes []byte

func canonicalTestRules() string {
	return string(canonicalTestRulesBytes)
}

func TestGlobToRegex(t *testing.T) {
	for _, test := range []struct {
		pattern string
		want    string
	}{{
		pattern: "python/*.py",
		want:    "^python/.*\\.py$",
	}, {
		pattern: "python/?.py",
		want:    "^python/.\\.py$",
	}, {
		pattern: "python/*.py",
		want:    "^python/.*\\.py$",
	}, {
		pattern: "python/*.py",
		want:    "^python/.*\\.py$",
	}, {
		pattern: "python/*.py",
		want:    "^python/.*\\.py$",
	}} {
		got := globToRegex(test.pattern)
		if got != test.want {
			t.Errorf("globToRegex(%v): got %v, want %v", test.pattern, got, test.want)
		}
	}
}

func TestTagRuleMatch(t *testing.T) {
	rule := &TagRule{
		Tags:     []string{"hit"},
		Lineno:   1,
		Dirs:     []string{"fancy"},
		Files:    []string{"file.txt"},
		Patterns: []string{"python/*.py"},
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
	rule := &TagRule{
		Tags:     []string{"hit"},
		Lineno:   1,
		Dirs:     []string{"fancy"},
		Files:    []string{"file.txt"},
		Patterns: []string{"python/*.py"},
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
}

func TestSanitizeLine(t *testing.T) {
	for _, test := range []struct {
		line string
		want string
	}{{
		line: "a # b",
		want: "a",
	}, {
		line: "# a b c ",
		want: "",
	}, {
		line: "a # b c ",
		want: "a",
	}, {
		line: " ",
		want: "",
	}, {
		line: "a b c",
		want: "a b c",
	}, {
		line: "a #### b c    ",
		want: "a",
	}, {
		line: "####     ",
		want: "",
	}} {
		got := sanitizeLine(test.line)
		if got != test.want {
			t.Errorf("sanitizeLine(%v): got %v, want %v", test.line, got, test.want)
		}
	}
}

func TestParseRulesText(t *testing.T) {
	for _, test := range []struct {
		ruleContent string
		wantTagDefs []string
		wantRules   []*TagRule
	}{{
		ruleContent: canonicalTestRules(),
		wantTagDefs: []string{"paladin", "priest", "druid", "hunter", "rogue", "tank", "healer", "dps"},
		wantRules: []*TagRule{
			{Tags: []string{"paladin", "priest", "healer"}, Lineno: 16, Dirs: []string{"python"}, Files: []string{}, Patterns: []string{}},
			{Tags: []string{"paladin"}, Lineno: 20, Dirs: []string{}, Files: []string{".buildkite/data.rayci.yml"}, Patterns: []string{}},
			{Tags: []string{"druid"}, Lineno: 24, Dirs: []string{}, Files: []string{}, Patterns: []string{"doc/*.py"}},
		},
	}} {
		gotTagDefs, gotRules, err := parseRulesText(test.ruleContent)
		if err != nil {
			t.Errorf("parseRulesText(): %v", err)
		}

		sort.Strings(gotTagDefs)
		sort.Strings(test.wantTagDefs)
		if !reflect.DeepEqual(gotTagDefs, test.wantTagDefs) {
			t.Errorf("parseRulesText(): gotTagDefs %v, wantTagDefs %v", gotTagDefs, test.wantTagDefs)
		}
		if !reflect.DeepEqual(gotRules, test.wantRules) {
			fmt.Println("gotRules:")
			for _, rule := range gotRules {
				fmt.Printf("  %v\n", rule)
			}
			fmt.Println("wantRules:")
			for _, rule := range test.wantRules {
				fmt.Printf("  %v\n", rule)
			}
			t.Errorf("parseRulesText(): gotRules %v, wantRules %v", gotRules, test.wantRules)
		}
	}
}

func TestParseRulesError(t *testing.T) {
	ruleContent := `
	! paladin priest
	! druid hunter rogue
	! tank healer dps

	python/ # Directory to match
	@ paladin priest healer # Tags to emit for a rule. A rule without tags is a skipping rule.
	;

	! asdf qwerty
	python/ # Directory to match
	@ asdf qwerty
	;
	`
	_, _, err := parseRulesText(ruleContent)
	if err == nil {
		t.Errorf("parseRulesText(): got nil, want error")
	}
}

func TestFallbackTags(t *testing.T) {
	ruleContent := `
	! paladin priest
	! druid hunter rogue
	! tank healer dps

	python/ray/air/example.py
	@ paladin priest healer
	;
	`
	// Testing tagsForChangedFiles with a ruleSet that matches no files.
	ruleSet, err := NewTagRuleSet(ruleContent)
	if err != nil {
		t.Errorf("NewTagRuleSet(): %v", err)
	}
	if _, err := ruleSet.ValidateRules(); err != nil {
		t.Errorf("ValidateRules(): %v", err)
	}
	tags := tagsForChangedFiles(ruleSet, []string{"lol_i_dont_match_any_rule.py"})
	sort.Strings(tags)
	want := []string{"always", "lint", "core_cpp", "cpp", "dashboard", "doc", "java", "linux_wheels", "macos_wheels", "ml", "python", "release_tests", "serve", "tools", "tune", "train", "data"}
	sort.Strings(want)

	if !reflect.DeepEqual(tags, want) {
		t.Errorf("tagsForChangedFiles(): got %v, want %v", tags, want)
	}
}

func TestNewTagRuleSet(t *testing.T) {
	set, err := NewTagRuleSet(canonicalTestRules())
	if err != nil {
		t.Errorf("NewTagRuleSet(): %v", err)
	}
	if _, err := set.ValidateRules(); err != nil {
		t.Errorf("CheckRules(): %v", err)
	}
}

func TestTagRuleSetValidateRulesErrorUndeclaredTag(t *testing.T) {
	set, err := NewTagRuleSet(`
	! paladin priest

	python/ # Directory to match
	@ paladin priest healer # This tag is not declared
	;
	`)
	if err != nil {
		t.Errorf("NewTagRuleSet(): %v", err)
	}
	if _, err := set.ValidateRules(); err == nil {
		t.Errorf("ValidateRules(): got nil, want error")
	}
}

func TestTagRuleSetMatchTags(t *testing.T) {
	set, err := NewTagRuleSet(canonicalTestRules())
	if err != nil {
		t.Errorf("NewTagRuleSet(): %v", err)
	}
	if _, err := set.ValidateRules(); err != nil {
		t.Errorf("ValidateRules(): %v", err)
	}
	got, gotBool := set.MatchTags("python/ray/air/")
	sort.Strings(got)
	want := []string{"paladin", "priest", "healer"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MatchTags(): got %v, want %v", got, want)
	}
	if !gotBool {
		t.Errorf("MatchTags(): gotBool %v, wantBool %v", gotBool, true)
	}
}

func TestRunTagAnalysis(t *testing.T) {
	// Main adds "always" and "lint" tags by default. We want to test that these tags are added.
	testRules := "! always lint\n" + canonicalTestRules()
	tmp := t.TempDir()
	rulesPath := filepath.Join(tmp, "test_rules.txt")
	if err := os.WriteFile(rulesPath, []byte(testRules), 0644); err != nil {
		t.Fatalf("write test rules: %v", err)
	}

	env := newEnvsMap(map[string]string{
		"BUILDKITE":                          "true",
		"BUILDKITE_PULL_REQUEST":             "1",
		"BUILDKITE_PULL_REQUEST_BASE_BRANCH": "main",
		"BUILDKITE_COMMIT":                   "1234567890",
	})
	git := &MockGitClient{
		ChangedFiles: []string{"python/ray/air/example.py"},
	}

	cfg := &RunMainConfig{
		ConfigPaths: []string{rulesPath},
		Env:         env,
		Git:         git,
	}

	tags, err := RunTagAnalysis(cfg)
	if err != nil {
		t.Errorf("RunMain(): %v", err)
	}

	// Check stdout contains expected tags
	if !slices.Contains(tags, "paladin") {
		t.Errorf("RunMain(): expected 'paladin' in output, got %q", tags)
	}
}

func TestRunTagAnalysisMultipleConfigs(t *testing.T) {
	// Main adds "always" and "lint" tags by default. We want to test that these tags are added.
	testRulesOne := "! always lint\n" + canonicalTestRules()
	testRulesTwo := "! always lint\n" + `
! warlock mage

golang/ # This should also match golang/ray/air/example.py
@ warlock
;

`
	tmp := t.TempDir()
	rulesOnePath := filepath.Join(tmp, "test_rules_one.txt")
	rulesTwoPath := filepath.Join(tmp, "test_rules_two.txt")
	if err := os.WriteFile(rulesOnePath, []byte(testRulesOne), 0644); err != nil {
		t.Fatalf("write test rules one: %v", err)
	}
	if err := os.WriteFile(rulesTwoPath, []byte(testRulesTwo), 0644); err != nil {
		t.Fatalf("write test rules two: %v", err)
	}

	env := newEnvsMap(map[string]string{
		"BUILDKITE":                          "true",
		"BUILDKITE_PULL_REQUEST":             "1",
		"BUILDKITE_PULL_REQUEST_BASE_BRANCH": "main",
		"BUILDKITE_COMMIT":                   "1234567890",
	})
	git := &MockGitClient{
		ChangedFiles: []string{"golang/example.go", ".buildkite/data.rayci.yml"},
	}

	cfg := &RunMainConfig{
		ConfigPaths: []string{rulesOnePath, rulesTwoPath},
		Env:         env,
		Git:         git,
	}

	tags, err := RunTagAnalysis(cfg)
	if err != nil {
		t.Errorf("RunMain(): %v", err)
	}

	want := []string{"always", "lint", "warlock", "paladin"}
	sort.Strings(want)
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("RunMain(): got %v, want %v", tags, want)
	}
}

func TestRunTagAnalysisNonPullRequest(t *testing.T) {
	env := newEnvsMap(map[string]string{
		"BUILDKITE":              "true",
		"BUILDKITE_PULL_REQUEST": "false",
	})

	cfg := &RunMainConfig{
		ConfigPaths: []string{},
		Env:         env,
		Git:         &MockGitClient{},
	}

	tags, err := RunTagAnalysis(cfg)
	if err != nil {
		t.Errorf("RunMain(): %v", err)
	}

	// Non-PR builds should output "*" to run all tags
	if tags[0] != "*" {
		t.Errorf("RunMain(): expected '*' for non-PR build, got %q", tags)
	}
}

func runCommandFromDirectory(cmd *exec.Cmd, dir string) ([]byte, error) {
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run command: %w", err)
	}
	return output, nil
}

func TestWithRealGitClient(t *testing.T) {
	origin := t.TempDir()
	workDir := t.TempDir()
	scratchDir := t.TempDir()

	testRulesPath := filepath.Join(scratchDir, "test_rules.txt")
	if err := os.WriteFile(testRulesPath, []byte(canonicalTestRules()), 0644); err != nil {
		t.Fatalf("write test_rules.txt: %v", err)
	}

	if _, err := runCommandFromDirectory(exec.Command("git", "init", "--bare"), origin); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}

	if _, err := runCommandFromDirectory(exec.Command("git", "init"), workDir); err != nil {
		t.Fatalf("git init: %v", err)
	}

	if _, err := runCommandFromDirectory(exec.Command("git", "config", "user.email", "rayci@ray.io"), workDir); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}

	if _, err := runCommandFromDirectory(exec.Command("git", "config", "user.name", "Ray CI Test"), workDir); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}

	if _, err := runCommandFromDirectory(exec.Command("git", "remote", "add", "origin", origin), workDir); err != nil {
		t.Fatalf("git remote add origin: %v", err)
	}

	// Create README.md
	readmePath := filepath.Join(workDir, "README.md")
	readmeFile, err := os.Create(readmePath)
	if err != nil {
		t.Fatalf("create README.md: %v", err)
	}
	defer readmeFile.Close()
	_, err = readmeFile.WriteString("# README\n")
	if err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	if _, err := runCommandFromDirectory(exec.Command("git", "add", "README.md"), workDir); err != nil {
		t.Fatalf("git add README.md: %v", err)
	}

	if _, err := runCommandFromDirectory(exec.Command("git", "commit", "-m", "init with readme"), workDir); err != nil {
		t.Fatalf("git commit -m init with readme: %v", err)
	}

	if _, err := runCommandFromDirectory(exec.Command("git", "push", "origin", "master"), workDir); err != nil {
		t.Fatalf("git push origin master: %v", err)
	}

	// Create each file in this list in the work directory
	files := []string{
		"python/ray/air/example.py",
		"python/ray/air/example2.py",
		"python/ray/air/example3.py",
		"doc/asdf.py",
	}
	if _, err := runCommandFromDirectory(exec.Command("git", "checkout", "-B", "pr01", "master"), workDir); err != nil {
		t.Fatalf("git checkout -B pr01 master: %v", err)
	}
	for _, file := range files {
		filePath := filepath.Join(workDir, file)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(filePath), err)
		}
		if err := os.WriteFile(filepath.Join(workDir, file), []byte("...\n"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if _, err := runCommandFromDirectory(exec.Command("git", "add", file), workDir); err != nil {
			t.Fatalf("git add %s: %v", file, err)
		}
		if _, err := runCommandFromDirectory(exec.Command("git", "commit", "-m", "add test files"), workDir); err != nil {
			t.Fatalf("git commit -m add test files: %v", err)
		}
	}

	output, err := runCommandFromDirectory(exec.Command("git", "show", "HEAD", "-q", "--format=%H"), workDir)
	if err != nil {
		t.Fatalf("git show HEAD: %v", err)
	}

	commit := strings.TrimSpace(string(output))

	env := newEnvsMap(map[string]string{
		"BUILDKITE":                          "true",
		"BUILDKITE_PULL_REQUEST":             "1",
		"BUILDKITE_PULL_REQUEST_BASE_BRANCH": "master",
		"BUILDKITE_COMMIT":                   commit,
	})

	tags, err := RunTagAnalysis(&RunMainConfig{
		ConfigPaths: []string{testRulesPath},
		Env:         env,
		Git:         &RealGitClient{WorkDir: workDir},
	})
	if err != nil {
		t.Fatalf("RunMain(): %v", err)
	}

	// Tags are already deduplicated and sorted by RunMain
	// Expected tags:
	// - always, lint: hardcoded
	// - paladin, priest, healer: from python/ray/air/* files matching python/ rule
	// - druid: from doc/asdf.py matching doc/*.py rule
	want := []string{"always", "druid", "healer", "lint", "paladin", "priest"}
	sort.Strings(want)
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("RunMain(): got %v, want %v", tags, want)
	}
}
