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
	stdout, _, err := ac.execAnyscale(args)
	return stdout, err
}

// runAnyscaleCLICombined runs the anyscale CLI and returns stdout and stderr
// merged into one string. Use this when you need to read CLI status messages
// (e.g. "Workspace created successfully id: ...") that the CLI writes to
// stderr.
func (ac *AnyscaleCLI) runAnyscaleCLICombined(args []string) (string, error) {
	stdout, stderr, err := ac.execAnyscale(args)
	if err != nil {
		return "", err
	}
	return stdout + stderr, nil
}

func (ac *AnyscaleCLI) execAnyscale(args []string) (string, string, error) {
	if ac.runFunc != nil {
		out, err := ac.runFunc(args)
		return out, "", err
	}
	if !ac.isAnyscaleInstalled() {
		return "", "", errors.New("anyscale is not installed")
	}

	fmt.Fprintf(os.Stdout, ">>> anyscale %s\n", strings.Join(args, " "))
	cmd := exec.Command(ac.bin, args...)

	stdoutBuf := newTailWriter(maxOutputBufferSize)
	stderrBuf := newTailWriter(maxOutputBufferSize)
	cmd.Stdout = io.MultiWriter(os.Stdout, stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, stderrBuf)

	err := cmd.Run()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	if err != nil {
		return "", "", fmt.Errorf(
			"anyscale error: %w\nstdout: %s\nstderr: %s",
			err, stdout, stderr,
		)
	}
	// The anyscale CLI sometimes exits 0 even when a remote `run_command`
	// failed, logging `exec failed with exit code N` via logrus (stderr).
	// Scan both streams so we don't miss it.
	if strings.Contains(stdout, "exec failed with exit code") ||
		strings.Contains(stderr, "exec failed with exit code") {
		return "", "", fmt.Errorf(
			"anyscale error: command failed:\nstdout: %s\nstderr: %s",
			stdout, stderr,
		)
	}

	return stdout, stderr, nil
}
