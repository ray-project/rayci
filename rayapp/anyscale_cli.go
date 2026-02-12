package rayapp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// AnyscaleCLI provides methods for interacting with the Anyscale CLI.
type AnyscaleCLI struct {
	client *http.Client
	bin    string // path to the anyscale binary; defaults to "anyscale"
}

const maxOutputBufferSize = 1024 * 1024 // 1 MB

// NewAnyscaleCLI creates a new AnyscaleCLI instance.
func NewAnyscaleCLI() *AnyscaleCLI {
	return &AnyscaleCLI{bin: "anyscale", client: &http.Client{}}
}

type WorkspaceState int

const (
	StateTerminated WorkspaceState = iota
	StateStarting
	StateRunning
)

var WorkspaceStateName = map[WorkspaceState]string{
	StateTerminated: "TERMINATED",
	StateStarting:   "STARTING",
	StateRunning:    "RUNNING",
}

func (ws WorkspaceState) String() string {
	return WorkspaceStateName[ws]
}

func (ac *AnyscaleCLI) isAnyscaleInstalled() bool {
	_, err := exec.LookPath(ac.bin)
	return err == nil
}

// runAnyscaleCLI runs the anyscale CLI with the given arguments.
// Returns the combined output and any error that occurred.
// Output is displayed to the terminal with colors preserved.
func (ac *AnyscaleCLI) runAnyscaleCLI(args []string) (string, error) {
	if !ac.isAnyscaleInstalled() {
		return "", errors.New("anyscale is not installed")
	}

	fmt.Println("anyscale cli args: ", args)
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
	if strings.Contains(tw.String(), "exec failed with exit code") {
		return "", fmt.Errorf("anyscale error: command failed: %s", tw.String())
	}

	return tw.String(), nil
}
