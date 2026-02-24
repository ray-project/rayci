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
