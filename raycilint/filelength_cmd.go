package raycilint

import (
	"flag"
	"fmt"
	"os"
)

const defaultMaxLines = 500

var filelengthUsage = `rayci-lint go-filelength - Check Go file lengths against limits

Finds all .go files in the current directory (recursive) and checks their
line counts against the configured max_lines. Test files, generated files,
vendor/, and symlinks are excluded.

Usage:
  rayci-lint go-filelength [-config-value key=value]

Flags:
  -config-value key=value
    Override a config value. Supported keys:
` + overrideKeysHelp(filelengthConfig{}) + "\n"

func cmdFilelength(cfg *config, args []string) error {
	set := flag.NewFlagSet(
		"rayci-lint go-filelength", flag.ContinueOnError,
	)
	set.Usage = func() { fmt.Fprint(os.Stderr, filelengthUsage) }

	var configOverrides multiFlag
	set.Var(&configOverrides, "config-value", "override a config value")

	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", set.Args())
	}

	if err := applyOverrides(cfg.Filelength, configOverrides); err != nil {
		return err
	}

	if cfg.Filelength.MaxLines <= 0 {
		cfg.Filelength.MaxLines = defaultMaxLines
	}

	return runFilelength(cfg)
}
