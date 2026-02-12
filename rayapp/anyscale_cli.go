package rayapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
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

var workspaceIDRe = regexp.MustCompile(`id:\s*(expwrk_[a-zA-Z0-9]+)`)

// extractWorkspaceID extracts the workspace ID from the CLI output.
// Expected format: "Workspace created successfully id: expwrk_xxx"
func extractWorkspaceID(output string) (string, error) {
	matches := workspaceIDRe.FindStringSubmatch(output)
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
	if err != nil {
		return tw.String(), fmt.Errorf("anyscale error: %w", err)
	}
	if strings.Contains(tw.String(), "exec failed with exit code") {
		return "", fmt.Errorf("anyscale error: command failed: %s", tw.String())
	}

	return tw.String(), nil
}

// CreateComputeConfig creates a new compute config from a YAML file if it doesn't already exist.
// If the config file uses the old format (head_node_type, worker_node_types), it will be
// have the cloud added to it if missing.
// name: the name for the compute config (without version tag)
// configFilePath: path to the YAML config file
func (ac *AnyscaleCLI) CreateComputeConfig(name, configFilePath string) error {
	list, err := ac.ListComputeConfigs(&name)
	if err != nil {
		return fmt.Errorf("list compute configs: %w", err)
	}
	if len(list) > 0 {
		fmt.Printf("Compute config %q already exists, skipping creation\n", name)
		return nil
	}

	// Check if the config file uses the old format
	isOldFormat, err := isLegacyComputeConfigFormat(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to check config format: %w", err)
	}

	// If old format, create a temp copy, add cloud key if missing, then use the copy
	actualConfigPath := configFilePath
	if isOldFormat {
		fmt.Printf("Detected old compute config format, using temp copy...\n")

		hasCloud, err := hasCloudKey(actualConfigPath)
		if err != nil {
			return fmt.Errorf("failed to check cloud key: %w", err)
		}

		if !hasCloud {
			tmpFile, err := os.CreateTemp("", "compute-config-*.yaml")
			if err != nil {
				return fmt.Errorf("failed to create temp file: %w", err)
			}
			tmpPath := tmpFile.Name()
			tmpFile.Close()
			defer os.Remove(tmpPath)

			if err := CopyFile(actualConfigPath, tmpPath); err != nil {
				return fmt.Errorf("failed to copy config file: %w", err)
			}
			cloudInfo, err := ac.GetDefaultCloud()
			if err != nil {
				return fmt.Errorf("failed to get default cloud: %w", err)
			}
			if err := addCloudKey(tmpPath, cloudInfo.Name); err != nil {
				return fmt.Errorf("failed to add cloud key: %w", err)
			}
			actualConfigPath = tmpPath
		}
		fmt.Printf("Temp copy: %s\n", actualConfigPath)
	}

	// Create the compute config
	var args []string
	if isOldFormat {
		args = []string{"compute-config", "create", "-n", name, actualConfigPath}
	} else {
		args = []string{"compute-config", "create", "-n", name, "-f", actualConfigPath}
	}
	_, err = ac.runAnyscaleCLI(args)
	if err != nil {
		return fmt.Errorf("create compute config failed: %w", err)
	}
	return nil
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

// ListComputeConfigs returns compute configs from "compute-config list --json". Returns an empty list when there are no results.
func (ac *AnyscaleCLI) ListComputeConfigs(name *string) ([]ComputeConfigListItem, error) {
	args := []string{"compute-config", "list", "--json"}
	if name != nil {
		args = append(args, "--name", *name)
	}
	output, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return nil, fmt.Errorf("list compute configs failed: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(output), &m); err != nil {
		return nil, fmt.Errorf("parse list output: %w", err)
	}

	resultsAny, ok := m["results"]
	if !ok || resultsAny == nil {
		return []ComputeConfigListItem{}, nil
	}

	resultsSlice, ok := resultsAny.([]any)
	if !ok {
		return nil, fmt.Errorf("results is not an array")
	}

	out := make([]ComputeConfigListItem, 0, len(resultsSlice))
	for i, itemAny := range resultsSlice {
		item, ok := itemAny.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("results[%d] is not an object", i)
		}
		li, err := computeConfigListItemFromMap(item)
		if err != nil {
			return nil, fmt.Errorf("results[%d]: %w", i, err)
		}
		out = append(out, li)
	}
	return out, nil
}

func computeConfigListItemFromMap(m map[string]any) (ComputeConfigListItem, error) {
	li := ComputeConfigListItem{}
	if v, ok := m["id"].(string); ok {
		li.ID = v
	}
	if v, ok := m["name"].(string); ok {
		li.Name = v
	}
	if v, ok := m["cloud_id"].(string); ok {
		li.CloudID = v
	}
	if v, ok := m["version"].(float64); ok {
		li.Version = v
	}
	if v, ok := m["created_at"].(string); ok {
		li.CreatedAt = v
	}
	if v, ok := m["last_modified_at"].(string); ok {
		li.LastModifiedAt = v
	}
	if v, ok := m["url"].(string); ok {
		li.URL = v
	}
	return li, nil
}

// CloudInfo represents the cloud information returned from the CLI.
type CloudInfo struct {
	Name string `yaml:"name"`
	ID   string `yaml:"id"`
}

// GetDefaultCloud retrieves the default cloud from the Anyscale CLI.
// Returns the cloud name and ID from the YAML output.
func (ac *AnyscaleCLI) GetDefaultCloud() (*CloudInfo, error) {
	args := []string{"cloud", "get-default"}
	output, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return nil, fmt.Errorf("get default cloud failed: %w", err)
	}

	var cloudInfo CloudInfo
	if err := yaml.Unmarshal([]byte(output), &cloudInfo); err != nil {
		return nil, fmt.Errorf("failed to parse cloud info: %w", err)
	}

	return &cloudInfo, nil
}

func (ac *AnyscaleCLI) createEmptyWorkspace(wtc *WorkspaceTestConfig) (string, error) {
	args := []string{"workspace_v2", "create"}
	args = append(args, "--name", wtc.workspaceName)
	if wtc.template.ClusterEnv != nil {
		env := wtc.template.ClusterEnv
		if env.BYOD != nil && env.BYOD.ContainerFile != "" {
			buildDir := filepath.Dir(wtc.buildFile)
			resolvedPath := filepath.Join(buildDir, env.BYOD.ContainerFile)
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

	resp, err := ac.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"delete workspace failed with status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	fmt.Printf("delete workspace %s succeeded: %s\n", workspaceID, string(body))
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
