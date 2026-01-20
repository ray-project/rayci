package goqualgate

import (
	"flag"
	"fmt"
	"os"
)

const usage = `goqualgate - Go quality gates for CI

Usage:
  goqualgate <command> [flags]

Commands:
  coverage    Compare test coverage between branches
`

const coverageUsage = `goqualgate coverage - Compare test coverage between branches

For PR builds:
  1. Runs 'go test -cover' on the current branch
  2. Checks out the base branch and runs coverage there
  3. Compares per-package coverage and fails if coverage decreased
     beyond threshold or new packages are below minimum

For non-PR builds, it simply measures and reports coverage.

Configuration is read from .goqualgate.yaml (or -config path).

Usage:
  goqualgate coverage [flags]

Flags:
`

func Main() error {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "coverage":
		return cmdCoverage(os.Args[2:])
	case "-h", "-help", "--help", "help":
		fmt.Print(usage)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
	return nil
}

func cmdCoverage(args []string) error {
	defaultBaseBranch := os.Getenv("BUILDKITE_PULL_REQUEST_BASE_BRANCH")
	prEnv := os.Getenv("BUILDKITE_PULL_REQUEST")
	defaultIsPR := prEnv != "" && prEnv != "false"

	set := flag.NewFlagSet("goqualgate coverage", flag.ExitOnError)
	set.Usage = func() {
		fmt.Fprint(os.Stderr, coverageUsage)
		set.PrintDefaults()
	}

	var configFile string
	var threshold, newPkgThreshold float64
	var baseBranch string
	var isPR bool
	set.StringVar(&configFile, "config", DefaultConfigFile,
		"Path to goqualgate config file.")
	set.Float64Var(&threshold, "threshold", 0,
		"Maximum allowed coverage decrease percentage (overrides config).")
	set.Float64Var(&newPkgThreshold, "new-package-threshold", 0,
		"Minimum required coverage for new packages (overrides config).")
	set.StringVar(&baseBranch, "base-branch", defaultBaseBranch,
		"Base branch to compare against.")
	set.BoolVar(&isPR, "pr", defaultIsPR,
		"Whether this is a PR build.")

	set.Parse(args)

	config, err := loadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfg := CoverageConfig{
		Threshold:           config.Coverage.Threshold,
		NewPackageThreshold: config.Coverage.NewPackageThreshold,
		BaseBranch:          baseBranch,
		IsPR:                isPR,
	}

	// Apply defaults and flag overrides
	if cfg.Threshold == 0 {
		cfg.Threshold = DefaultThreshold
	}
	if threshold > 0 {
		cfg.Threshold = threshold
	}
	if newPkgThreshold > 0 {
		cfg.NewPackageThreshold = newPkgThreshold
	}

	return cfg.Run()
}

type multiString []string

func (m *multiString) String() string {
	return fmt.Sprintf("%v", *m)
}

func (m *multiString) Set(value string) error {
	*m = append(*m, value)
	return nil
}
