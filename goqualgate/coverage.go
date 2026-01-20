package goqualgate

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CoverageConfig holds settings for coverage comparison.
type CoverageConfig struct {
	Threshold           float64
	NewPackageThreshold float64
	BaseBranch          string
	IsPR                bool
}

// DefaultThreshold is the maximum allowed coverage decrease (0.5%).
const DefaultThreshold = 0.5

// PackageCoverage maps package paths to coverage percentages.
type PackageCoverage map[string]float64

// Run executes coverage comparison between current and base branches.
func (cfg CoverageConfig) Run() error {
	if !cfg.IsPR {
		fmt.Printf("Not a PR build, measuring coverage only\n")
		coverage, err := execGoTestCover()
		if err != nil {
			return fmt.Errorf("measuring coverage: %w", err)
		}
		fmt.Printf("\nCoverage measured for %d packages\n", len(coverage))
		return nil
	}

	if cfg.BaseBranch == "" {
		return fmt.Errorf("base branch required for PR builds")
	}

	fmt.Printf("=== Running coverage on current branch ===\n")

	currentCoverage, err := execGoTestCover()
	if err != nil {
		return fmt.Errorf("current branch coverage: %w", err)
	}
	fmt.Printf("\nCoverage measured for %d packages\n", len(currentCoverage))

	currentRef, err := gitCurrentRef()
	if err != nil {
		return fmt.Errorf("get current ref: %w", err)
	}

	fmt.Printf("\n=== Checking out base branch: %s ===\n", cfg.BaseBranch)

	if err := git("fetch", "origin", cfg.BaseBranch); err != nil {
		return fmt.Errorf("fetch base branch: %w", err)
	}
	if err := git("checkout", "origin/"+cfg.BaseBranch); err != nil {
		return fmt.Errorf("checkout base branch: %w", err)
	}

	fmt.Printf("\n=== Running coverage on base branch ===\n")

	baseCoverage, err := execGoTestCover()
	if err != nil {
		git("checkout", currentRef) // best-effort restore
		return fmt.Errorf("base branch coverage: %w", err)
	}
	fmt.Printf("\nCoverage measured for %d packages\n", len(baseCoverage))

	fmt.Printf("\n=== Returning to: %s ===\n", currentRef)

	if err := git("checkout", currentRef); err != nil {
		return fmt.Errorf("checkout original ref: %w", err)
	}

	return compareCoverage(cfg, baseCoverage, currentCoverage)
}

func compareCoverage(cfg CoverageConfig, base, current PackageCoverage) error {
	threshold := cfg.Threshold
	if threshold == 0 {
		threshold = DefaultThreshold
	}

	var commonPkgs, newPkgs, removedPkgs []string
	for pkg := range current {
		if _, ok := base[pkg]; ok {
			commonPkgs = append(commonPkgs, pkg)
		} else {
			newPkgs = append(newPkgs, pkg)
		}
	}
	for pkg := range base {
		if _, ok := current[pkg]; !ok {
			removedPkgs = append(removedPkgs, pkg)
		}
	}
	sort.Strings(commonPkgs)
	sort.Strings(newPkgs)
	sort.Strings(removedPkgs)

	var failures []string

	fmt.Printf("\n=== Coverage Comparison ===\n")
	fmt.Printf("Threshold: -%.2f%% allowed\n", threshold)
	if cfg.NewPackageThreshold > 0 {
		fmt.Printf("New package minimum: %.2f%%\n", cfg.NewPackageThreshold)
	}

	if len(removedPkgs) > 0 {
		fmt.Printf("\nRemoved packages (%d):\n", len(removedPkgs))
		for _, pkg := range removedPkgs {
			fmt.Printf("  %s (was %.2f%%)\n", pkg, base[pkg])
		}
	}

	if len(newPkgs) > 0 {
		fmt.Printf("\nNew packages (%d):\n", len(newPkgs))
		for _, pkg := range newPkgs {
			cov := current[pkg]
			if cfg.NewPackageThreshold > 0 && cov < cfg.NewPackageThreshold {
				fmt.Printf("  FAIL  %s: %.2f%% (need %.2f%%)\n", pkg, cov, cfg.NewPackageThreshold)
				failures = append(failures, fmt.Sprintf("%s: %.2f%% (need %.2f%%)", pkg, cov, cfg.NewPackageThreshold))
			} else {
				fmt.Printf("  pass  %s: %.2f%%\n", pkg, cov)
			}
		}
	}

	if len(commonPkgs) > 0 {
		fmt.Printf("\nExisting packages (%d):\n", len(commonPkgs))
		for _, pkg := range commonPkgs {
			baseCov, currCov := base[pkg], current[pkg]
			diff := currCov - baseCov
			if baseCov-currCov > threshold {
				fmt.Printf("  FAIL  %s: %.2f%% -> %.2f%% (%.2f%%)\n", pkg, baseCov, currCov, diff)
				failures = append(failures, fmt.Sprintf("%s: %.2f%% -> %.2f%%", pkg, baseCov, currCov))
			} else if diff != 0 {
				fmt.Printf("  pass  %s: %.2f%% -> %.2f%% (%+.2f%%)\n", pkg, baseCov, currCov, diff)
			} else {
				fmt.Printf("  pass  %s: %.2f%% (no change)\n", pkg, currCov)
			}
		}
	}

	fmt.Printf("\n")
	if len(failures) > 0 {
		fmt.Printf("FAILED: %d issue(s)\n", len(failures))
		for _, f := range failures {
			fmt.Printf("  - %s\n", f)
		}
		return fmt.Errorf("coverage check failed")
	}

	fmt.Printf("PASSED\n")
	return nil
}

func getTestPackages() ([]string, error) {
	cmd := exec.Command("go", "list", "-f", "{{if .TestGoFiles}}{{.ImportPath}}{{end}}", "./...")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list: %w\n%s", err, stderr.String())
	}

	var pkgs []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		// Exclude goqualgate to avoid recursive test invocation
		if line != "" && !strings.Contains(line, "/goqualgate") {
			pkgs = append(pkgs, line)
		}
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no test packages found")
	}
	return pkgs, nil
}

type testEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Output  string `json:"Output"`
}

// Matches "coverage: 80.5% of statements"
var coverageRe = regexp.MustCompile(`coverage:\s+([\d.]+)%`)

func execGoTestCover() (PackageCoverage, error) {
	pkgs, err := getTestPackages()
	if err != nil {
		return nil, err
	}

	args := append([]string{"test", "-json", "-cover"}, pkgs...)
	cmd := exec.Command("go", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	coverage := make(PackageCoverage)
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		var event testEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			fmt.Println(scanner.Text())
			continue
		}
		if event.Action == "output" && event.Output != "" {
			fmt.Print(event.Output)
			if matches := coverageRe.FindStringSubmatch(event.Output); len(matches) == 2 {
				if pct, err := strconv.ParseFloat(matches[1], 64); err == nil {
					coverage[event.Package] = pct
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		if len(coverage) > 0 {
			fmt.Printf("\nWarning: some tests failed, coverage data may be incomplete\n")
			return coverage, nil
		}
		return nil, fmt.Errorf("go test: %w", err)
	}
	if len(coverage) == 0 {
		return nil, fmt.Errorf("no coverage data found")
	}
	return coverage, nil
}

func git(args ...string) error {
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w\n%s", args[0], err, out)
	}
	return nil
}

func gitCurrentRef() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
