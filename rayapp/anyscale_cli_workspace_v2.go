package rayapp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
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
	if name, ok := workspaceStateName[ws]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", int(ws))
}

// workspaceCreatedIDRe matches the "Workspace created successfully id: ..."
// status line that `anyscale workspace_v2 create` prints to stderr.
var workspaceCreatedIDRe = regexp.MustCompile(
	`Workspace created successfully id:\s+(expwrk_[A-Za-z0-9_-]+)`,
)

// createEmptyWorkspace creates a workspace and returns its id.
//
// The id is parsed from the CLI status line because every other workspace_v2
// subcommand needs to address the workspace by --id rather than --name:
// the by-name path on the Anyscale API has been observed to fail to find
// newly-created workspaces, while the by-id path is reliable.
func (ac *AnyscaleCLI) createEmptyWorkspace(c *WorkspaceTestConfig) (string, error) {
	if c.template == nil {
		return "", fmt.Errorf("template is required")
	}
	args := []string{"workspace_v2", "create"}
	args = append(args, "--name", c.workspaceName)
	if c.template.ClusterEnv != nil {
		env := c.template.ClusterEnv
		if env.BYOD != nil && env.BYOD.ContainerFile != "" {
			resolvedPath := filepath.Join(c.buildDir, env.BYOD.ContainerFile)
			args = append(
				args,
				"--containerfile",
				resolvedPath,
				"--ray-version",
				env.BYOD.RayVersion,
			)
		} else {
			imageURI, rayVersion, err := getImageURIAndRayVersionFromClusterEnv(
				c.template.ClusterEnv,
			)
			if err != nil {
				return "", fmt.Errorf("cluster env: %w", err)
			}
			args = append(args, "--image-uri", imageURI)
			// Nightly images don't need --ray-version; the version
			// is embedded in the image URI tag.
			if !nightlyTagRe.MatchString(imageURI) {
				args = append(args, "--ray-version", rayVersion)
			}
		}
	}

	if c.computeConfig != "" {
		args = append(args, "--compute-config", c.computeConfig)
	}

	out, err := ac.runAnyscaleCLICombined(args)
	if err != nil {
		return "", fmt.Errorf("create empty workspace failed: %w", err)
	}
	m := workspaceCreatedIDRe.FindStringSubmatch(out)
	if len(m) != 2 {
		return "", fmt.Errorf(
			"create empty workspace: could not parse workspace id from output",
		)
	}
	return m[1], nil
}

func (ac *AnyscaleCLI) getWorkspaceDescription(workspaceID string) (map[string]any, error) {
	output, err := ac.runAnyscaleCLI(
		[]string{"workspace_v2", "get", "--id", workspaceID, "--json"},
	)
	if err != nil {
		return nil, fmt.Errorf("get workspace failed: %w", err)
	}
	var workspaceDescription map[string]any
	if err := json.Unmarshal([]byte(output), &workspaceDescription); err != nil {
		return nil, fmt.Errorf("parse workspace get output: %w", err)
	}
	return workspaceDescription, nil
}

func (ac *AnyscaleCLI) terminateWorkspace(workspaceID string) error {
	_, err := ac.runAnyscaleCLI([]string{"workspace_v2", "terminate", "--id", workspaceID})
	if err != nil {
		return fmt.Errorf("terminate workspace failed: %w", err)
	}
	return nil
}

func (ac *AnyscaleCLI) pushFolderToWorkspace(workspaceID, localFilePath string) error {
	_, err := ac.runAnyscaleCLI(
		[]string{"workspace_v2", "push", "--id", workspaceID, "--local-dir", localFilePath},
	)
	if err != nil {
		return fmt.Errorf("push file to workspace failed: %w", err)
	}
	return nil
}

func (ac *AnyscaleCLI) runCmdInWorkspace(workspaceID string, cmd string) error {
	_, err := ac.runAnyscaleCLI(
		[]string{"workspace_v2", "run_command", "--id", workspaceID, cmd},
	)
	if err != nil {
		return fmt.Errorf("run command in workspace failed: %w", err)
	}
	return nil
}

func (ac *AnyscaleCLI) startWorkspace(workspaceID string) error {
	_, err := ac.runAnyscaleCLI([]string{"workspace_v2", "start", "--id", workspaceID})
	if err != nil {
		return fmt.Errorf("start workspace failed: %w", err)
	}
	return nil
}

func (ac *AnyscaleCLI) getWorkspaceStatus(workspaceID string) (string, error) {
	output, err := ac.runAnyscaleCLI([]string{"workspace_v2", "status", "--id", workspaceID})
	if err != nil {
		return "", fmt.Errorf("get workspace state failed: %w", err)
	}
	return output, nil
}

func (ac *AnyscaleCLI) waitForWorkspaceState(
	workspaceID string,
	state WorkspaceState,
) (string, error) {
	output, err := ac.runAnyscaleCLI(
		[]string{"workspace_v2", "wait", "--id", workspaceID, "--state", state.String()},
	)
	if err != nil {
		return "", fmt.Errorf("wait for workspace state failed: %w", err)
	}
	return output, nil
}
