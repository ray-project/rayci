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

func parseTestCasesFile(path string) ([]*ruleTestCase, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseTestCases(string(content))
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

func runTestRules(ruleSet *TagRuleSet, testCases []*ruleTestCase) []*testCaseResult {
	var failures []*testCaseResult
	for _, tc := range testCases {
		got := tagsForChangedFiles(ruleSet, []string{tc.File})

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

const testRulesUsage = `usage: rayci test-rules TESTS_FILE

Validates test rules against expected file→tag mappings.

Requires RAYCI_TEST_RULE_FILES environment variable to specify rule files.

Test file format:
  # Comments start with #
  file_path: tag1 tag2 tag3

Example:
  python/ray/data/__init__.py: always lint data ml
  README.md: always lint
`

func subcmdTestRules(args []string, envs Envs) error {
	if len(args) != 1 {
		return fmt.Errorf(testRulesUsage)
	}
	testsFile := args[0]

	rulePaths := testRuleFilesFromEnv(envs)
	if len(rulePaths) == 0 {
		return fmt.Errorf("RAYCI_TEST_RULE_FILES environment variable is required")
	}

	merged, err := loadAndMergeTagRuleConfigs(rulePaths)
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}

	testCases, err := parseTestCasesFile(testsFile)
	if err != nil {
		return fmt.Errorf("load tests: %w", err)
	}
	if len(testCases) == 0 {
		return fmt.Errorf("no test cases found in %s", testsFile)
	}

	failures := runTestRules(merged.RuleSet, testCases)

	if len(failures) > 0 {
		printFailures(failures)
		return fmt.Errorf("%d/%d test(s) failed", len(failures), len(testCases))
	}

	return nil
}
