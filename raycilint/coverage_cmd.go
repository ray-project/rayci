package raycilint

import (
	"flag"
	"fmt"
	"os"
)

const defaultMinCoveragePct = 80.0

var coverageUsage = `rayci-lint go-coverage - Run test coverage checks

Runs 'go test -cover' on all packages and reports coverage.
Fails if any package is below the configured min_coverage_pct.

Usage:
  rayci-lint go-coverage [-config-value key=value]

Flags:
  -config-value key=value
    Override a config value. Supported keys:
` + overrideKeysHelp(coverageConfig{}) + `

Improving Coverage (for AI agents):

  1. Run coverage check to identify failing packages:
       ./rayci-lint go-coverage

  2. For each failing package, get detailed function-level coverage:
       go test -coverprofile=cover.out ./<package>/...
       go tool cover -func=cover.out | grep -v "100.0%"

  3. Focus on functions with lowest coverage first. Common patterns:
     - Error paths: Create invalid inputs to trigger error returns
     - Default branches: Test functions with zero/empty values
     - Edge cases: Test boundary conditions

  4. Testing patterns to follow:
     - Use t.TempDir() for file-based tests
     - Format: "got, want" ordering in assertions
     - Run 'go test ./<package>/...' after each change

  5. Skip functions that require external services (AWS, Docker) unless
     mocking infrastructure exists. Focus on testable code paths.
`

func cmdCoverage(cfg *config, args []string) error {
	set := flag.NewFlagSet(
		"rayci-lint go-coverage", flag.ContinueOnError,
	)
	set.Usage = func() { fmt.Fprint(os.Stderr, coverageUsage) }

	var configOverrides multiFlag
	set.Var(&configOverrides, "config-value", "override a config value")

	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", set.Args())
	}

	if err := applyOverrides(cfg.Coverage, configOverrides); err != nil {
		return err
	}

	if cfg.Coverage.MinCoveragePct <= 0 {
		cfg.Coverage.MinCoveragePct = defaultMinCoveragePct
	}

	return runCoverage(cfg)
}
