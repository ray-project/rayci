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

	"gopkg.in/yaml.v2"
)

type WorkspaceState int

// WorkspaceTestConfig contains all the details to test a workspace.
type WorkspaceTestConfig struct {
	tmplName      string
	buildFile     string
	workspaceName string
	configFile    string
	computeConfig string
	imageURI      string
	rayVersion    string
	template      *Template
}

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

// isOldComputeConfigFormat checks if a YAML file uses the old compute config format
// by looking for old-style keys like "head_node_type" or "worker_node_types".
func isOldComputeConfigFormat(configFilePath string) (bool, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse into a generic map to check for old-style keys
	var configMap map[string]interface{}
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return false, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Check for old format keys
	_, hasHeadNodeType := configMap["head_node_type"]
	_, hasWorkerNodeTypes := configMap["worker_node_types"]

	return hasHeadNodeType || hasWorkerNodeTypes, nil
}

// CreateComputeConfig creates a new compute config from a YAML file if it doesn't already exist.
// If the config file uses the old format (head_node_type, worker_node_types), it will be
// automatically converted to the new format before creation.
// name: the name for the compute config (without version tag)
// configFile: path to the YAML config file
// Returns the output from the CLI and any error.
func (ac *AnyscaleCLI) CreateComputeConfig(name, configFilePath string) (string, error) {
	// Check if compute config already exists
	if output, err := ac.GetComputeConfig(name); err == nil {
		fmt.Printf("Compute config %q already exists, skipping creation\n", name)
		return output, nil
	}

	// Check if the config file uses the old format
	isOldFormat, err := isOldComputeConfigFormat(configFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to check config format: %w", err)
	}

	// If old format, convert to new format and use a temp file
	actualConfigPath := configFilePath
	if isOldFormat {
		fmt.Printf("Detected old compute config format, converting to new format...\n")

		newConfigData, err := ConvertComputeConfig(configFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to convert old config: %w", err)
		}

		// Create a temp file for the converted config
		tmpFile, err := os.CreateTemp("", "compute-config-*.yaml")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.Write(newConfigData); err != nil {
			tmpFile.Close()
			return "", fmt.Errorf("failed to write temp file: %w", err)
		}
		tmpFile.Close()

		actualConfigPath = tmpFile.Name()
		fmt.Printf("Converted config saved to temp file: %s\n", actualConfigPath)
	}

	// Create the compute config
	args := []string{"compute-config", "create", "-n", name, "-f", actualConfigPath}
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

func (ac *AnyscaleCLI) pushTemplateToWorkspace(workspaceName, localFilePath string) error {
	output, err := ac.runAnyscaleCLI([]string{"workspace_v2", "push", "--name", workspaceName, "--local-dir", localFilePath})
	if err != nil {
		return fmt.Errorf("push file to workspace failed: %w", err)
	}
	fmt.Println("push file to workspace output:\n", output)
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

// OldComputeConfig represents the old compute config format
type OldComputeConfig struct {
	HeadNodeType    OldHeadNodeType    `yaml:"head_node_type"`
	WorkerNodeTypes []OldWorkerNodeType `yaml:"worker_node_types"`
}

// OldHeadNodeType represents the head node configuration in old format
type OldHeadNodeType struct {
	Name         string `yaml:"name"`
	InstanceType string `yaml:"instance_type"`
}

// OldWorkerNodeType represents a worker node configuration in old format
type OldWorkerNodeType struct {
	Name         string `yaml:"name"`
	InstanceType string `yaml:"instance_type"`
}

// NewComputeConfig represents the new compute config format
type NewComputeConfig struct {
	HeadNode               NewHeadNode `yaml:"head_node"`
	AutoSelectWorkerConfig bool        `yaml:"auto_select_worker_config"`
}

// NewHeadNode represents the head node configuration in new format
type NewHeadNode struct {
	InstanceType string `yaml:"instance_type"`
}

// ConvertComputeConfig converts an old format compute config to the new format.
// It reads the old YAML file, transforms the structure, and returns the new YAML content.
func ConvertComputeConfig(oldConfigPath string) ([]byte, error) {
	// Read the old config file
	data, err := os.ReadFile(oldConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read old config file: %w", err)
	}

	// Parse the old format
	var oldConfig OldComputeConfig
	if err := yaml.Unmarshal(data, &oldConfig); err != nil {
		return nil, fmt.Errorf("failed to parse old config: %w", err)
	}

	// Convert to new format
	newConfig := NewComputeConfig{
		HeadNode: NewHeadNode{
			InstanceType: oldConfig.HeadNodeType.InstanceType,
		},
		AutoSelectWorkerConfig: true,
	}

	// Marshal to YAML
	newData, err := yaml.Marshal(&newConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new config: %w", err)
	}

	return newData, nil
}

// ConvertComputeConfigFile converts an old format compute config file to a new format file.
// If outputPath is empty, the new config is written to stdout.
func ConvertComputeConfigFile(oldConfigPath, newConfigPath string) error {
	newData, err := ConvertComputeConfig(oldConfigPath)
	if err != nil {
		return err
	}

	if newConfigPath == "" {
		fmt.Print(string(newData))
		return nil
	}

	if err := os.WriteFile(newConfigPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write new config file: %w", err)
	}

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

	return fmt.Sprintf("anyscale/ray:%s.%s.%s%s", major, minor, patch, suffix), fmt.Sprintf("%s.%s.%s", major, minor, patch), nil
}
