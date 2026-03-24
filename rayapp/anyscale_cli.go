package rayapp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// AnyscaleCLI provides methods for interacting with the Anyscale CLI.
type AnyscaleCLI struct {
	bin     string                              // path to the anyscale binary; defaults to "anyscale"
	runFunc func(args []string) (string, error) // function to run the CLI.
}

const maxOutputBufferSize = 1024 * 1024 // 1 MB

// NewAnyscaleCLI creates a new AnyscaleCLI instance.
func NewAnyscaleCLI() *AnyscaleCLI {
	return &AnyscaleCLI{bin: "anyscale"}
}

func (ac *AnyscaleCLI) isAnyscaleInstalled() bool {
	_, err := exec.LookPath(ac.bin)
	return err == nil
}

// setRunFunc sets a custom function to run the CLI.
// This is used to run the CLI in a custom way, for example, to mock the CLI for testing.
func (ac *AnyscaleCLI) setRunFunc(f func(args []string) (string, error)) {
	ac.runFunc = f
}

// runAnyscaleCLI runs the anyscale CLI with the given arguments.
// Returns the combined output and any error that occurred.
// Output is displayed to the terminal with colors preserved.
func (ac *AnyscaleCLI) runAnyscaleCLI(args []string) (string, error) {
	if ac.runFunc != nil {
		return ac.runFunc(args)
	}
	if !ac.isAnyscaleInstalled() {
		return "", errors.New("anyscale is not installed")
	}

	fmt.Fprintf(os.Stdout, ">>> anyscale %s\n", strings.Join(args, " "))
	cmd := exec.Command(ac.bin, args...)

	tw := newTailWriter(maxOutputBufferSize)
	cmd.Stdout = io.MultiWriter(os.Stdout, tw)
	cmd.Stderr = io.MultiWriter(os.Stderr, tw)

	err := cmd.Run()
	output := tw.String()
	if err != nil {
		return output, fmt.Errorf("anyscale error: %w", err)
	}
	if strings.Contains(output, "exec failed with exit code") {
		return output, fmt.Errorf("anyscale error: command failed: %s", output)
	}

	return output, nil
}

// stripCLIWarnings removes leading lines that look like CLI warnings
// (e.g. "[WARNING] ...") before structured output. This handles the
// Anyscale CLI printing upgrade notices to stdout before JSON/YAML.
func stripCLIWarnings(s string) string {
	for {
		if len(s) == 0 {
			return s
		}
		if s[0] != '[' && s[0] != '(' {
			return s
		}
		// Lines starting with "[" could be a JSON array or a warning
		// like "[WARNING]". Check if it looks like a bracketed tag
		// followed by a space (e.g., "[WARNING] ...", "(NOTICE) ...").
		closers := map[byte]byte{'[': ']', '(': ')'}
		closer := closers[s[0]]
		end := strings.IndexByte(s, closer)
		if end < 0 {
			return s
		}
		// If the bracket pair is followed by content on the same
		// line (e.g., "[WARNING] ..."), treat it as a warning line.
		rest := s[end+1:]
		if len(rest) > 0 && rest[0] == ' ' {
			if idx := strings.IndexByte(s, '\n'); idx >= 0 {
				s = s[idx+1:]
				continue
			}
			return "" // single-line warning, nothing after it
		}
		// Otherwise it's likely structured data (e.g., JSON array).
		return s
	}
}
