package rayapp

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

var errAnyscaleNotInstalled = errors.New("anyscale is not installed")

func isAnyscaleInstalled() bool {
	_, err := exec.LookPath("anyscale")
	return err == nil
}

// RunAnyscaleCLI runs the anyscale CLI with the given arguments.
func RunAnyscaleCLI(args []string) (string, error) {
	if !isAnyscaleInstalled() {
		return "", errAnyscaleNotInstalled
	}
	
	cmd := exec.Command("anyscale", args...)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("anyscale error: %v\nstderr: %s", err, stderr.String())
	}
	
	return stdout.String(), nil
}