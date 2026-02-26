package raycilint

import (
	"flag"
	"fmt"
	"os"
)

const usage = `rayci-lint - Quality gates for CI

Usage:
  rayci-lint [-config-file <path>] <command> [flags]

Global flags:
  -config-file string
      Path to raycilint.yaml config file
      (default: .buildkite/raycilint.yaml if it exists).

Commands:
	go-coverage    Run test coverage and check minimum thresholds
	go-filelength  Check that Go files don't exceed line limit
`

// Main is the entry point for the rayci-lint CLI, dispatching
// to the appropriate subcommand.
func Main(args []string) (int, error) {
	top := flag.NewFlagSet("rayci-lint", flag.ContinueOnError)
	top.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	configFile := top.String(
		"config-file", "",
		"path to raycilint.yaml config file",
	)

	if err := top.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0, nil
		}
		return 1, err
	}

	rest := top.Args()
	if len(rest) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 1, nil
	}

	cfg, err := loadTopConfig(*configFile)
	if err != nil {
		return 1, err
	}

	cmd := rest[0]
	subArgs := rest[1:]

	switch cmd {
	case "-h", "-help", "--help", "help":
		fmt.Print(usage)
		return 0, nil
	case "go-coverage":
		err = cmdCoverage(cfg, subArgs)
	case "go-filelength":
		err = cmdFilelength(cfg, subArgs)
	default:
		fmt.Fprintf(
			os.Stderr, "unknown command: %s\n\n%s", cmd, usage,
		)
		return 1, nil
	}
	if err == flag.ErrHelp {
		return 0, nil
	}
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func loadTopConfig(configFile string) (*config, error) {
	if configFile != "" {
		return loadConfig(configFile)
	}
	if _, err := os.Stat(defaultConfigPath); err == nil {
		return loadConfig(defaultConfigPath)
	}
	return newConfig(), nil
}
