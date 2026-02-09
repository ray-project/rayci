package rayapp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// AnyscaleCLI provides methods for interacting with the Anyscale CLI.
type AnyscaleCLI struct {
	bin string // path to the anyscale binary; defaults to "anyscale"
}

const maxOutputBufferSize = 1024 * 1024 // 1 MB

// NewAnyscaleCLI creates a new AnyscaleCLI instance.
func NewAnyscaleCLI() *AnyscaleCLI {
	return &AnyscaleCLI{bin: "anyscale"}
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

// extractWorkspaceID extracts the workspace ID from the CLI output.
// Expected format: "Workspace created successfully id: expwrk_xxx"
func extractWorkspaceID(output string) (string, error) {
	re := regexp.MustCompile(`id:\s*(expwrk_[a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract workspace ID from output: %s", output)
	}
	return matches[1], nil
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

func (ac *AnyscaleCLI) createEmptyWorkspace(wtc *WorkspaceTestConfig) (string, error) {
	args := []string{"workspace_v2", "create"}
	args = append(args, "--name", wtc.workspaceName)
	if wtc.template.ClusterEnv != nil {
		env := wtc.template.ClusterEnv
		if env.BYOD != nil && env.BYOD.ContainerFile != "" {
			buildDir := filepath.Dir(wtc.buildFile)
			resolvedPath := filepath.Join(buildDir, env.BYOD.ContainerFile)
			args = append(args, "--containerfile", resolvedPath, "--ray-version", env.BYOD.RayVersion)
		} else {
			imageURI, rayVersion, err := getImageURIAndRayVersionFromClusterEnv(wtc.template.ClusterEnv)
			if err != nil {
				return "", fmt.Errorf("cluster env: %w", err)
			}
			args = append(args, "--image-uri", imageURI)
			args = append(args, "--ray-version", rayVersion)
		}
	}

	// Use compute config name if set
	if wtc.computeConfig != "" {
		args = append(args, "--compute-config", wtc.computeConfig)
	}

	output, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return "", fmt.Errorf("create empty workspace failed: %w", err)
	}

	workspaceID, err := extractWorkspaceID(output)
	if err != nil {
		return "", fmt.Errorf("failed to extract workspace ID: %w", err)
	}

	return workspaceID, nil
}

func (ac *AnyscaleCLI) terminateWorkspace(workspaceName string) error {
	_, err := ac.runAnyscaleCLI([]string{"workspace_v2", "terminate", "--name", workspaceName})
	if err != nil {
		return fmt.Errorf("terminate workspace failed: %w", err)
	}
	return nil
}

// deleteWorkspaceByID deletes a workspace by its ID using the Anyscale REST API.
// It uses the ANYSCALE_HOST environment variable for the API host and
// ANYSCALE_CLI_TOKEN for authentication.
func (ac *AnyscaleCLI) deleteWorkspaceByID(workspaceID string) error {
	anyscaleHost := os.Getenv("ANYSCALE_HOST")
	if anyscaleHost == "" {
		return errors.New("ANYSCALE_HOST environment variable is not set")
	}

	apiToken := os.Getenv("ANYSCALE_CLI_TOKEN")
	if apiToken == "" {
		return errors.New("ANYSCALE_CLI_TOKEN environment variable is not set")
	}

	url := fmt.Sprintf("%s/api/v2/experimental_workspaces/%s", anyscaleHost, workspaceID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("delete workspace failed with status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("delete workspace %s succeeded: %s\n", workspaceID, string(body))
	return nil
}

func (ac *AnyscaleCLI) pushFolderToWorkspace(workspaceName, localFilePath string) error {
	_, err := ac.runAnyscaleCLI([]string{"workspace_v2", "push", "--name", workspaceName, "--local-dir", localFilePath})
	if err != nil {
		return fmt.Errorf("push file to workspace failed: %w", err)
	}
	return nil
}

func (ac *AnyscaleCLI) runCmdInWorkspace(workspaceName string, cmd string) error {
	_, err := ac.runAnyscaleCLI([]string{"workspace_v2", "run_command", "--name", workspaceName, cmd})
	if err != nil {
		return fmt.Errorf("run command in workspace failed: %w", err)
	}
	return nil
}

func (ac *AnyscaleCLI) startWorkspace(workspaceName string) error {
	_, err := ac.runAnyscaleCLI([]string{"workspace_v2", "start", "--name", workspaceName})
	if err != nil {
		return fmt.Errorf("start workspace failed: %w", err)
	}
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
