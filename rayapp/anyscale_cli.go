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
// On success, returns stdout only so that CLI warnings on stderr do not
// corrupt structured (JSON/YAML) output used for parsing.
// On failure, returns an empty string and an error containing both
// stdout and stderr for full diagnostic context.
// Both streams are always displayed to the terminal.
func (ac *AnyscaleCLI) runAnyscaleCLI(args []string) (string, error) {
	if ac.runFunc != nil {
		return ac.runFunc(args)
	}
	if !ac.isAnyscaleInstalled() {
		return "", errors.New("anyscale is not installed")
	}

	fmt.Fprintf(os.Stdout, ">>> anyscale %s\n", strings.Join(args, " "))
	cmd := exec.Command(ac.bin, args...)

	stdoutBuf := newTailWriter(maxOutputBufferSize)
	stderrBuf := newTailWriter(maxOutputBufferSize)
	cmd.Stdout = io.MultiWriter(os.Stdout, stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, stderrBuf)

	err := cmd.Run()
	stdout := stdoutBuf.String()
	if err != nil {
		stderr := stderrBuf.String()
		return "", fmt.Errorf(
			"anyscale error: %w\nstdout: %s\nstderr: %s",
			err, stdout, stderr,
		)
	}
	if strings.Contains(stdout, "exec failed with exit code") {
		return "", fmt.Errorf("anyscale error: command failed: %s", stdout)
	}

	return stdout, nil
}
