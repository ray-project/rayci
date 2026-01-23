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

type WorkspaceState int

const (
    StateTerminated WorkspaceState = iota
    StateStarting
    StateRunning
)

var WorkspaceStateName = map[WorkspaceState]string{
    StateTerminated: "TERMINATED",
    StateStarting: "STARTING",
    StateRunning:  "RUNNING",
}

func (ws WorkspaceState) String() string {
    return WorkspaceStateName[ws]
}

type AnyscaleCLI struct {
	token string
}

var errAnyscaleNotInstalled = errors.New("anyscale is not installed")

func NewAnyscaleCLI(token string) *AnyscaleCLI {
	return &AnyscaleCLI{token: token}
}

func isAnyscaleInstalled() bool {
	_, err := exec.LookPath("anyscale")
	return err == nil
}

func (ac *AnyscaleCLI) Authenticate() error {
	cmd := exec.Command("anyscale", "login")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("anyscale auth login failed, please set ANYSCALE_CLI_TOKEN & ANYSCALE_HOST env variables: %w", err)
	}
	return nil
}

// RunAnyscaleCLI runs the anyscale CLI with the given arguments.
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

// parseComputeConfigName parses the AWS config path and converts it to a config name.
// e.g., "configs/basic-single-node/aws.yaml" -> "basic-single-node-aws"
func parseComputeConfigName(awsConfigPath string) string {
	// Get the directory and filename
	dir := filepath.Dir(awsConfigPath)           // "configs/basic-single-node"
	base := filepath.Base(awsConfigPath)         // "aws.yaml"
	ext := filepath.Ext(base)                    // ".yaml"
	filename := strings.TrimSuffix(base, ext)    // "aws"

	// Get the last directory component (the config name)
	configDir := filepath.Base(dir) // "basic-single-node"

	// Combine: "basic-single-node-aws"
	return configDir + "-" + filename
}

func (ac *AnyscaleCLI) createEmptyWorkspace(config *WorkspaceTestConfig) error {
	args := []string{"workspace_v2", "create"}
	// get image URI and ray version from build ID
	imageURI, rayVersion, err := convertBuildIdToImageURI(config.template.ClusterEnv.BuildID)
	if err != nil {
		return fmt.Errorf("convert build ID to image URI failed: %w", err)
	}
	args = append(args, "--name", config.workspaceName)
	args = append(args, "--image-uri", imageURI)
	args = append(args, "--ray-version", rayVersion)

	// Use compute config name if set
	if config.computeConfig != "" {
		args = append(args, "--compute-config", config.computeConfig)
	}

	output, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return fmt.Errorf("create empty workspace failed: %w", err)
	}
	fmt.Println("create empty workspace output:\n", output)
	return nil
}

func (ac *AnyscaleCLI) terminateWorkspace(workspaceName string) error {
	output, err := ac.runAnyscaleCLI([]string{"workspace_v2", "terminate", "--name", workspaceName})
	if err != nil {
		return fmt.Errorf("delete workspace failed: %w", err)
	}
	fmt.Println("terminate workspace output:\n", output)
	return nil
}

func (ac *AnyscaleCLI) copyTemplateToWorkspace(config *WorkspaceTestConfig) error {
	output, err := ac.runAnyscaleCLI([]string{"workspace_v2", "push", "--name", config.workspaceName, "--local-dir", config.template.Dir})
	if err != nil {
		return fmt.Errorf("copy template to workspace failed: %w", err)
	}
	fmt.Println("copy template to workspace output:\n", output)
	return nil
}

func (ac *AnyscaleCLI) runCmdInWorkspace(config *WorkspaceTestConfig, cmd string) error {
	output, err := ac.runAnyscaleCLI([]string{"workspace_v2", "run_command", "--name", config.workspaceName, cmd})
	if err != nil {
		return fmt.Errorf("run command in workspace failed: %w", err)
	}
	fmt.Println("run command in workspace output:\n", output)
	return nil
}

func (ac *AnyscaleCLI) startWorkspace(config *WorkspaceTestConfig) error {
	output, err := ac.runAnyscaleCLI([]string{"workspace_v2", "start", "--name", config.workspaceName})
	if err != nil {
		return fmt.Errorf("start workspace failed: %w", err)
	}
	fmt.Println("start workspace output:\n", output)
	return nil
}

func (ac *AnyscaleCLI) getWorkspaceStatus(workspaceName string) (string, error) {
	output, err := ac.runAnyscaleCLI([]string{"workspace_v2", "status", "--name", workspaceName})
	if err != nil {
		return "", fmt.Errorf("get workspace state failed: %w", err)
	}
	return output, nil
}

func (ac *AnyscaleCLI) waitForWorkspaceState(workspaceName string, state WorkspaceState) (string, error) {
	output, err := ac.runAnyscaleCLI([]string{"workspace_v2", "wait", "--name", workspaceName, "--state", state.String()})
	if err != nil {
		return "", fmt.Errorf("wait for workspace state failed: %w", err)
	}
	return output, nil
}

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
