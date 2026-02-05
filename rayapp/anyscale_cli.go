package rayapp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// AnyscaleCLI provides methods for interacting with the Anyscale CLI.
type AnyscaleCLI struct{}

const maxOutputBufferSize = 1024 * 1024 // 1 MB

// NewAnyscaleCLI creates a new AnyscaleCLI instance.
func NewAnyscaleCLI() *AnyscaleCLI {
	return &AnyscaleCLI{}
}

func isAnyscaleInstalled() bool {
	_, err := exec.LookPath("anyscale")
	return err == nil
}

// runAnyscaleCLI runs the anyscale CLI with the given arguments.
// Returns the combined output and any error that occurred.
// Output is displayed to the terminal with colors preserved.
func (ac *AnyscaleCLI) runAnyscaleCLI(args []string) (string, error) {
	if !isAnyscaleInstalled() {
		return "", errors.New("anyscale is not installed")
	}

	fmt.Println("anyscale cli args: ", args)
	cmd := exec.Command("anyscale", args...)

	var outputBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &outputBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &outputBuf)

	err := cmd.Run()
	output := outputBuf.Bytes()
	if len(output) > maxOutputBufferSize {
		output = output[len(output)-maxOutputBufferSize:]
	}

	if err != nil {
		return string(output), fmt.Errorf("anyscale error: %w", err)
	}
	return string(output), nil
}
