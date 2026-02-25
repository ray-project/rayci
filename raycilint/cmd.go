package raycilint

import (
	"fmt"
	"os"
)

const usage = `rayci-lint - Quality gates for CI

Usage:
  rayci-lint <command> [flags]

Commands:
	go-coverage    Run test coverage and check minimum thresholds
	go-filelength  Check that Go files don't exceed line limit
`

const installHint = `
To install and run rayci-lint locally, download 'rayci-lint' from the latest release:
  https://github.com/ray-project/rayci/releases/latest
`

// Main is the entry point for the rayci-lint CLI, dispatching
// to the appropriate subcommand.
func Main(args []string) (int, error) {
	if len(args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		return 1, nil
	}

	cmd := args[1]
	subArgs := args[2:]

	var err error
	switch cmd {
	case "-h", "-help", "--help", "help":
		fmt.Print(usage)
		return 0, nil
	case "go-coverage":
		err = cmdCoverage(subArgs)
	case "go-filelength":
		err = cmdFilelength(subArgs)
	default:
		fmt.Fprintf(
			os.Stderr, "unknown command: %s\n\n%s", cmd, usage,
		)
		return 1, nil
	}
	if err != nil {
		fmt.Fprint(os.Stderr, installHint)
		return 1, err
	}
	return 0, nil
}
