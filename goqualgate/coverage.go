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

// CoverageConfig holds settings for coverage checks.
type CoverageConfig struct {
	MinCoveragePct float64
}

// PackageCoverage maps package paths to coverage percentages.
type PackageCoverage map[string]float64

// Run executes coverage checks on the repository.
func (cfg CoverageConfig) Run() error {
	coverage, err := execGoTestCover()
	if err != nil {
		return fmt.Errorf("running coverage: %w", err)
	}

	return cfg.checkCoverage(coverage)
}

func (cfg CoverageConfig) checkCoverage(coverage PackageCoverage) error {
	var pkgs []string
	for pkg := range coverage {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	var failures []string

	fmt.Printf("=== Coverage Results ===\n")
	for _, pkg := range pkgs {
		cov := coverage[pkg]
		if cfg.MinCoveragePct > 0 && cov < cfg.MinCoveragePct {
			fmt.Printf("FAIL  %s: %.2f%% (need %.2f%%)\n", pkg, cov, cfg.MinCoveragePct)
			failures = append(failures, fmt.Sprintf("%s: %.2f%%", pkg, cov))
		} else {
			fmt.Printf("pass  %s: %.2f%%\n", pkg, cov)
		}
	}

	if len(failures) > 0 {
		fmt.Printf("FAILED: %d package(s) below minimum\n", len(failures))
		return fmt.Errorf("coverage check failed")
	}
	fmt.Printf("PASSED: %d package(s) checked\n", len(pkgs))
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
			continue
		}
		if event.Action == "output" && event.Output != "" {
			if matches := coverageRe.FindStringSubmatch(event.Output); len(matches) == 2 {
				if pct, err := strconv.ParseFloat(matches[1], 64); err == nil {
					coverage[event.Package] = pct
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		if len(coverage) > 0 {
			return coverage, nil
		}
		return nil, fmt.Errorf("go test: %w", err)
	}
	if len(coverage) == 0 {
		return nil, fmt.Errorf("no coverage data found")
	}
	return coverage, nil
}
