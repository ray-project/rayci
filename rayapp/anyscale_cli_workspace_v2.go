package rayapp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// WorkspaceState represents the state of an Anyscale workspace.
type WorkspaceState int

const (
	StateTerminated WorkspaceState = iota // workspace is stopped
	StateStarting                         // workspace is starting up
	StateRunning                          // workspace is ready
)

var workspaceStateName = map[WorkspaceState]string{
	StateTerminated: "TERMINATED",
	StateStarting:   "STARTING",
	StateRunning:    "RUNNING",
}

// String returns the Anyscale API name of the state (e.g., "RUNNING").
func (ws WorkspaceState) String() string {
	return workspaceStateName[ws]
}

func (ac *AnyscaleCLI) createEmptyWorkspace(wtc *WorkspaceTestConfig) error {
	args := []string{"workspace_v2", "create"}
	args = append(args, "--name", wtc.workspaceName)
	if wtc.template.ClusterEnv != nil {
		env := wtc.template.ClusterEnv
		if env.BYOD != nil && env.BYOD.ContainerFile != "" {
			resolvedPath := filepath.Join(wtc.buildDir, env.BYOD.ContainerFile)
			args = append(
				args,
				"--containerfile",
				resolvedPath,
				"--ray-version",
				env.BYOD.RayVersion,
			)
		} else {
			imageURI, rayVersion, err := getImageURIAndRayVersionFromClusterEnv(
				wtc.template.ClusterEnv,
			)
			if err != nil {
				return fmt.Errorf("cluster env: %w", err)
			}
			args = append(args, "--image-uri", imageURI)
			args = append(args, "--ray-version", rayVersion)
		}
	}

	// Use compute config name if set
	if wtc.computeConfig != "" {
		args = append(args, "--compute-config", wtc.computeConfig)
	}

	_, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return fmt.Errorf("create empty workspace failed: %w", err)
	}

	return nil
}

func (ac *AnyscaleCLI) getWorkspaceID(workspaceName string) (string, error) {
	workspaceDescription, err := ac.getWorkspaceDescription(workspaceName)
	if err != nil {
		return "", fmt.Errorf("get workspace description failed: %w", err)
	}
	workspaceID, ok := workspaceDescription["id"].(string)
	if !ok {
		return "", fmt.Errorf("workspace ID not found in description")
	}
	return workspaceID, nil
}

func (ac *AnyscaleCLI) getWorkspaceDescription(workspaceName string) (map[string]any, error) {
	output, err := ac.runAnyscaleCLI([]string{"workspace_v2", "get", "--name", workspaceName, "--json"})
	if err != nil {
		return nil, fmt.Errorf("get workspace failed: %w", err)
	}
	var workspaceDescription map[string]any
	if err := json.Unmarshal([]byte(output), &workspaceDescription); err != nil {
		return nil, fmt.Errorf("parse workspace get output: %w", err)
	}
	return workspaceDescription, nil
}

func (ac *AnyscaleCLI) terminateWorkspace(workspaceName string) error {
	_, err := ac.runAnyscaleCLI([]string{"workspace_v2", "terminate", "--name", workspaceName})
	if err != nil {
		return fmt.Errorf("terminate workspace failed: %w", err)
	}
	return nil
}

func (ac *AnyscaleCLI) pushFolderToWorkspace(workspaceName, localFilePath string) error {
	_, err := ac.runAnyscaleCLI(
		[]string{"workspace_v2", "push", "--name", workspaceName, "--local-dir", localFilePath},
	)
	if err != nil {
		return fmt.Errorf("push file to workspace failed: %w", err)
	}
	return nil
}

func (ac *AnyscaleCLI) runCmdInWorkspace(workspaceName string, cmd string) error {
	_, err := ac.runAnyscaleCLI(
		[]string{"workspace_v2", "run_command", "--name", workspaceName, cmd},
	)
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

func (ac *AnyscaleCLI) waitForWorkspaceState(
	workspaceName string,
	state WorkspaceState,
) (string, error) {
	output, err := ac.runAnyscaleCLI(
		[]string{"workspace_v2", "wait", "--name", workspaceName, "--state", state.String()},
	)
	if err != nil {
		return "", fmt.Errorf("wait for workspace state failed: %w", err)
	}
	return output, nil
}
