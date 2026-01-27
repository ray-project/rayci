package goqualgate

import (
	"flag"
	"fmt"
	"os"
)

const runAll = "all"
const usage = `goqualgate - Go quality gates for CI

Usage:
  goqualgate <command> [flags]

Commands:
	all         Run all quality gates with default settings
	coverage    Run test coverage and check minimum thresholds
	filelength  Check that Go files don't exceed line limit
`

const coverageUsage = `goqualgate coverage - Run test coverage checks

Runs 'go test -cover' on all packages and reports coverage.
Fails if any package is below -min-coverage-pct. 

Usage:
  goqualgate coverage [flags]

Flags:
	-min-coverage-pct float
		Minimum coverage percentage required to pass (default 60).

Improving Coverage (for AI agents):

  1. Run coverage check to identify failing packages:
       ./goqualgate coverage -min-coverage-pct <target-coverage-pct>

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

const filelengthUsage = `goqualgate filelength - Check Go file lengths against limits

Finds all .go files in the current directory (recursive) and checks their
line counts against -max-lines. Test files, generated files,
vendor/, and symlinks are excluded.

Usage:
  goqualgate filelength [flags]

Flags:
	-max-lines int
		Maximum allowed lines per file (default 500).
`

type subcommand struct {
	name string
	run  func([]string) error
}

var subcommands = []*subcommand{
	{"coverage", cmdCoverage},
	{"filelength", cmdFilelength},
}

// Main is the entry point for the goqualgate CLI, dispatching to the appropriate subcommand
// based on the first argument.
func Main(args []string) (int, error) {
	if len(args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		return 1, nil
	}

	cmd := args[1]
	subArgs := args[2:]

	switch cmd {
	case "-h", "-help", "--help", "help":
		fmt.Print(usage)
		return 0, nil
	}

	matched := false
	for i, sub := range subcommands {
		if sub.name == cmd || cmd == runAll {
			if cmd == runAll && i > 0 {
				// Print a separator between commands
				fmt.Println()
			}
			if err := sub.run(subArgs); err != nil {
				return 1, err
			}
			matched = true
		}
	}
	if matched {
		return 0, nil
	}

	fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
	return 1, nil
}

func parseCoverageConfig(args []string) (*CoverageConfig, error) {
	set := flag.NewFlagSet("goqualgate coverage", flag.ExitOnError)
	set.Usage = func() {
		fmt.Fprint(os.Stderr, coverageUsage)
		set.PrintDefaults()
	}

	cfg := new(CoverageConfig)
	set.Float64Var(&cfg.MinCoveragePct, "min-coverage-pct", 60,
		"Minimum coverage percentage required to pass (default 60).")

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

func parseFileLengthConfig(args []string) (*FileLengthConfig, error) {
	set := flag.NewFlagSet("goqualgate filelength", flag.ExitOnError)
	set.Usage = func() {
		fmt.Fprint(os.Stderr, filelengthUsage)
		set.PrintDefaults()
	}

	cfg := new(FileLengthConfig)
	set.IntVar(&cfg.MaxLines, "max-lines", 500,
		"Maximum allowed lines per file (default 500).")

	set.Parse(args)

	if set.NArg() > 0 {
		return nil, fmt.Errorf("unexpected arguments: %v", set.Args())
	}

	return cfg, nil
}

func cmdFilelength(args []string) error {
	cfg, err := parseFileLengthConfig(args)
	if err != nil {
		return err
	}
	return cfg.Run()
}
