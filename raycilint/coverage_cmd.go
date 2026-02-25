package raycilint

import (
	"flag"
	"fmt"
	"os"
)

const defaultMinCoveragePct = 80.0

var coverageUsage = fmt.Sprintf(`rayci-lint go-coverage - Run test coverage checks

Runs 'go test -cover' on all packages and reports coverage.
Fails if any package is below -min-coverage-pct.

Usage:
  rayci-lint go-coverage [flags]

Flags:
	-min-coverage-pct float
		Minimum coverage percentage required to pass (default %.f).`, defaultMinCoveragePct) + `

Improving Coverage (for AI agents):

  1. Run coverage check to identify failing packages:
       ./rayci-lint go-coverage -min-coverage-pct <target-coverage-pct>

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

func parseCoverageConfig(args []string) (*CoverageConfig, error) {
	set := flag.NewFlagSet("rayci-lint go-coverage", flag.ExitOnError)
	set.Usage = func() {
		fmt.Fprint(os.Stderr, coverageUsage)
		set.PrintDefaults()
	}

	cfg := new(CoverageConfig)
	set.Float64Var(
		&cfg.MinCoveragePct, "min-coverage-pct",
		defaultMinCoveragePct,
		fmt.Sprintf(
			"Minimum coverage percentage required to pass (default %.f).",
			defaultMinCoveragePct,
		),
	)

	set.Parse(args)

	if set.NArg() > 0 {
		return nil, fmt.Errorf("unexpected arguments: %v", set.Args())
	}

	return cfg, nil
}

func cmdCoverage(args []string) error {
	cfg, err := parseCoverageConfig(args)
	if err != nil {
		return err
	}
	return cfg.Run()
}
