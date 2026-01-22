package rayapp

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

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
// Returns the stdout output and any error that occurred.
func (ac *AnyscaleCLI) RunAnyscaleCLI(args []string) (string, error) {
	if !isAnyscaleInstalled() {
		return "", errAnyscaleNotInstalled
	}

	log.Println("anyscale cli args: ", args)
	cmd := exec.Command("anyscale", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("anyscale error: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
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
	if config.computeConfig != "" {
		args = append(args, "--compute-config", "tmpl-test-basic-serverless-aws:1")
	}
	output, err := ac.RunAnyscaleCLI(args)
	if err != nil {
		return fmt.Errorf("create empty workspace failed: %w", err)
	}
	log.Println("create empty workspace output:\n", output)
	return nil
}

func (ac *AnyscaleCLI) terminateWorkspace(workspaceName string) error {
	output, err := ac.RunAnyscaleCLI([]string{"workspace_v2", "terminate", "--name", workspaceName})
	if err != nil {
		return fmt.Errorf("delete workspace failed: %w", err)
	}
	log.Println("terminate workspace output:\n", output)
	return nil
}

func (ac *AnyscaleCLI) copyTemplateToWorkspace(config *WorkspaceTestConfig) error {
	output, err := ac.RunAnyscaleCLI([]string{"workspace_v2", "push", "--name", config.workspaceName, "--local-dir", config.template.Dir})
	if err != nil {
		return fmt.Errorf("copy template to workspace failed: %w", err)
	}
	log.Println("copy template to workspace output:\n", output)
	return nil
}

func (ac *AnyscaleCLI) runCmdInWorkspace(config *WorkspaceTestConfig, cmd string) error {
	output, err := ac.RunAnyscaleCLI([]string{"workspace_v2", "run_command", "--name", config.workspaceName, cmd})
	if err != nil {
		return fmt.Errorf("run command in workspace failed: %w", err)
	}
	log.Println("run command in workspace output:\n", output)
	return nil
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

	return fmt.Sprintf("anyscale/ray:%s.%s.%s%s", major, minor, patch, suffix), versionStr, nil
}
