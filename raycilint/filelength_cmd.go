package raycilint

import (
	"flag"
	"fmt"
	"os"
)

const defaultMaxLines = 500

var filelengthUsage = fmt.Sprintf(`rayci-lint go-filelength - Check Go file lengths against limits

Finds all .go files in the current directory (recursive) and checks their
line counts against -max-lines. Test files, generated files,
vendor/, and symlinks are excluded.

Usage:
  rayci-lint go-filelength [flags]

Flags:
	-max-lines int
		Maximum allowed lines per file (default %d).
`, defaultMaxLines)

func parseFileLengthConfig(args []string) (*FileLengthConfig, error) {
	set := flag.NewFlagSet("rayci-lint go-filelength", flag.ExitOnError)
	set.Usage = func() {
		fmt.Fprint(os.Stderr, filelengthUsage)
		set.PrintDefaults()
	}

	cfg := new(FileLengthConfig)
	set.IntVar(
		&cfg.MaxLines, "max-lines", defaultMaxLines,
		fmt.Sprintf(
			"Maximum allowed lines per file (default %d).",
			defaultMaxLines,
		),
	)

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
