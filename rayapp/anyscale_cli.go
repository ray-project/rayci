package rayapp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorkspaceState represents the state of a workspace.
type WorkspaceState int

const (
	StateTerminated WorkspaceState = iota
	StateStarting
	StateRunning
)

// WorkspaceStateName maps WorkspaceState values to their string representations.
var WorkspaceStateName = map[WorkspaceState]string{
	StateTerminated: "TERMINATED",
	StateStarting:   "STARTING",
	StateRunning:    "RUNNING",
}

// String returns the string representation of a WorkspaceState.
func (ws WorkspaceState) String() string {
	return WorkspaceStateName[ws]
}

// AnyscaleCLI provides methods for interacting with the Anyscale CLI.
type AnyscaleCLI struct{}

var errAnyscaleNotInstalled = errors.New("anyscale is not installed")

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
		return "", errAnyscaleNotInstalled
	}

	fmt.Println("anyscale cli args: ", args)
	cmd := exec.Command("anyscale", args...)

	// Capture output while also displaying to terminal with colors
	var outputBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &outputBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &outputBuf)

	if err := cmd.Run(); err != nil {
		return outputBuf.String(), fmt.Errorf("anyscale error: %w", err)
	}

	return outputBuf.String(), nil
}

// convertBuildIdToImageURI converts a build ID to an image URI and Ray version.
// Build IDs have the format "anyscaleray{version}-{suffix}" where:
// - version is a 4+ digit string like "2441" representing major.minor.patch (2.44.1)
// - suffix is optional and contains Python version and CUDA version (e.g., "py312-cu128")
// Returns the image URI (e.g., "anyscale/ray:2.44.1-py312-cu128") and Ray version (e.g., "2.44.1").
func convertBuildIdToImageURI(buildId string) (string, string, error) {
	// Convert build ID like "anyscaleray2441-py312-cu128" to "anyscale/ray:2.44.1-py312-cu128"
	const prefix = "anyscaleray"
	if !strings.HasPrefix(buildId, prefix) {
		return "", "", fmt.Errorf("build ID must start with %q: %s", prefix, buildId)
	}

	// Remove the prefix to get "2441-py312-cu128"
	remainder := strings.TrimPrefix(buildId, prefix)

	// Find the first hyphen to separate version from suffix
	hyphenIdx := strings.Index(remainder, "-")
	var versionStr, suffix string
	if hyphenIdx == -1 {
		versionStr = remainder
		suffix = ""
	} else {
		versionStr = remainder[:hyphenIdx]
		suffix = remainder[hyphenIdx:] // includes the hyphen
	}

	// Parse version: "2441" -> "2.44.1"
	// Format: first digit = major, next two = minor, rest = patch
	if len(versionStr) < 4 {
		return "", "", fmt.Errorf("version string too short: %s", versionStr)
	}

	major := versionStr[0:1]
	minor := versionStr[1:3]
	patch := versionStr[3:]

	return fmt.Sprintf("anyscale/ray:%s.%s.%s%s", major, minor, patch, suffix), fmt.Sprintf("%s.%s.%s", major, minor, patch), nil
}

// parseComputeConfigName parses the AWS config path and converts it to a config name.
// e.g., "configs/basic-single-node/aws.yaml" -> "basic-single-node-aws"
func parseComputeConfigName(awsConfigPath string) string {
	// Get the directory and filename
	dir := filepath.Dir(awsConfigPath)        // "configs/basic-single-node"
	base := filepath.Base(awsConfigPath)      // "aws.yaml"
	ext := filepath.Ext(base)                 // ".yaml"
	filename := strings.TrimSuffix(base, ext) // "aws"

	// Get the last directory component (the config name)
	configDir := filepath.Base(dir) // "basic-single-node"

	// Combine: "basic-single-node-aws"
	return configDir + "-" + filename
}

// CreateComputeConfig creates a new compute config from a YAML file if it doesn't already exist.
// name: the name for the compute config (without version tag)
// configFilePath: path to the YAML config file
// Returns the output from the CLI and any error.
func (ac *AnyscaleCLI) CreateComputeConfig(name, configFilePath string) (string, error) {
	// Check if compute config already exists
	if output, err := ac.GetComputeConfig(name); err == nil {
		fmt.Printf("Compute config %q already exists, skipping creation\n", name)
		return output, nil
	}

	// Create the compute config
	args := []string{"compute-config", "create", "-n", name, "-f", configFilePath}
	output, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return output, fmt.Errorf("create compute config failed: %w", err)
	}
	return output, nil
}

// GetComputeConfig retrieves the details of a compute config by name.
// name: the name of the compute config (optionally with version tag, e.g., "name:1")
// Returns the output from the CLI and any error.
func (ac *AnyscaleCLI) GetComputeConfig(name string) (string, error) {
	args := []string{"compute-config", "get", "-n", name}
	output, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return output, fmt.Errorf("get compute config failed: %w", err)
	}
	return output, nil
}
