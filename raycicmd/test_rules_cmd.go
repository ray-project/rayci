package raycicmd

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
)

type ruleTestCase struct {
	File   string
	Tags   []string
	Lineno int
}

func parseTestCases(content string) ([]*ruleTestCase, error) {
	var cases []*ruleTestCase
	for lineno, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.Index(line, ":")
		if idx == -1 {
			return nil, fmt.Errorf("line %d: invalid format, expected 'file: tags'", lineno+1)
		}

		file := strings.TrimSpace(line[:idx])
		if file == "" {
			return nil, fmt.Errorf("line %d: empty file path", lineno+1)
		}

		cases = append(cases, &ruleTestCase{
			File:   file,
			Tags:   strings.Fields(line[idx+1:]),
			Lineno: lineno + 1,
		})
	}

	return cases, nil
}

func parseTestCasesFile(path string) ([]*ruleTestCase, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseTestCases(string(content))
}

// companionTestFile returns the companion test file for a rules file.
// e.g., "go.rules.txt" -> "go.rules.test.txt"
func companionTestFile(rulesFile string) string {
	return strings.TrimSuffix(rulesFile, ".txt") + ".test.txt"
}

type testCaseResult struct {
	File    string
	Lineno  int
	Extra   []string // tags in got but not in want
	Missing []string // tags in want but not in got
}

func diffTags(got, want []string) (extra, missing []string) {
	gotSet := make(map[string]struct{}, len(got))
	for _, t := range got {
		gotSet[t] = struct{}{}
	}

	wantSet := make(map[string]struct{}, len(want))
	for _, t := range want {
		wantSet[t] = struct{}{}
	}

	for _, t := range got {
		if _, ok := wantSet[t]; !ok {
			extra = append(extra, t)
		}
	}

	for _, t := range want {
		if _, ok := gotSet[t]; !ok {
			missing = append(missing, t)
		}
	}

	sort.Strings(extra)
	sort.Strings(missing)
	return extra, missing
}

func runTestRules(ruleSets []*TagRuleSet, testCases []*ruleTestCase) []*testCaseResult {
	var failures []*testCaseResult
	for _, tc := range testCases {
		got := tagsForChangedFiles(ruleSets, []string{tc.File})

		want := slices.Clone(tc.Tags)
		sort.Strings(want)
		want = slices.Compact(want)

		if slices.Equal(got, want) {
			continue
		}

		extra, missing := diffTags(got, want)
		failures = append(failures, &testCaseResult{
			File:    tc.File,
			Lineno:  tc.Lineno,
			Extra:   extra,
			Missing: missing,
		})
	}

	return failures
}

func printFailures(failures []*testCaseResult) {
	for _, f := range failures {
		fmt.Printf("FAIL: %s (line %d)\n", f.File, f.Lineno)
		for _, tag := range f.Extra {
			fmt.Printf("  +%s (unexpected)\n", tag)
		}
		for _, tag := range f.Missing {
			fmt.Printf("  -%s (missing)\n", tag)
		}
	}
}

const testRulesUsage = `usage: rayci [-buildkite-dir DIR] test-rules

Validates test rules against expected file→tag mappings.

Discovers *.rules.txt files in buildkite directories and runs their
companion *.rules.test.txt files if they exist.

Example: go.rules.txt is tested by go.rules.test.txt

Test file format:
  # Comments start with #
  file_path: tag1 tag2 tag3

Example:
  wanda/main.go: wanda
  raycicmd/main.go: raycicmd
`

func runTestRulesCmd(args []string, config *config) error {
	totalCases := 0
	var allFailures []*testCaseResult

	for _, dir := range config.buildkiteDirs() {
		rulesFiles, err := listRulesFiles(dir)
		if err != nil {
			return fmt.Errorf("list rules files in %s: %w", dir, err)
		}

		for _, rulesFile := range rulesFiles {
			testFile := companionTestFile(rulesFile)
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				continue
			}

			cases, err := parseTestCasesFile(testFile)
			if err != nil {
				return err
			}
			if len(cases) == 0 {
				continue
			}

			ruleSets, err := loadTagRuleConfigs([]string{rulesFile})
			if err != nil {
				return fmt.Errorf("load rules %s: %w", rulesFile, err)
			}

			failures := runTestRules(ruleSets, cases)
			totalCases += len(cases)
			allFailures = append(allFailures, failures...)
		}
	}

	if totalCases == 0 {
		fmt.Fprintf(os.Stderr, "warning: no test cases found (add *.rules.test.txt companion files)\n")
		return nil
	}

	if len(allFailures) > 0 {
		printFailures(allFailures)
		return fmt.Errorf("%d/%d test(s) failed", len(allFailures), totalCases)
	}

	return nil
}
