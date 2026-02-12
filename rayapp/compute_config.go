package rayapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// ComputeConfigListItem represents one entry from "compute-config list --json" results.
type ComputeConfigListItem struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	CloudID        string  `json:"cloud_id"`
	Version        float64 `json:"version"`
	CreatedAt      string  `json:"created_at"`
	LastModifiedAt string  `json:"last_modified_at"`
	URL            string  `json:"url"`
}

// OldComputeConfig represents the old compute config format
type OldComputeConfig struct {
	HeadNodeType    OldHeadNodeType     `yaml:"head_node_type"`
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

// parseComputeConfigName parses the config path and converts it to a config name.
// e.g., "configs/basic-single-node/aws.yaml" -> "basic-single-node-aws"
func parseComputeConfigName(configPath string) string {
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	ext := filepath.Ext(base)
	filename := strings.TrimSuffix(base, ext)
	configDir := filepath.Base(dir)
	if configDir == "." || configDir == string(filepath.Separator) {
		return filename
	}
	return configDir + "-" + filename
}

// isOldComputeConfigFormat checks if a YAML file uses the old compute config format
// by looking for old-style keys like "head_node_type" or "worker_node_types".
func isOldComputeConfigFormat(configFilePath string) (bool, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to read config file: %w", err)
	}
	var configMap map[string]any
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return false, fmt.Errorf("failed to parse config file: %w", err)
	}
	_, hasHeadNodeType := configMap["head_node_type"]
	_, hasWorkerNodeTypes := configMap["worker_node_types"]
	return hasHeadNodeType || hasWorkerNodeTypes, nil
}

// hasCloudKey checks if a YAML config file contains a "cloud" key.
// Returns true if the cloud key exists, false otherwise.
func hasCloudKey(configFilePath string) (bool, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to read config file: %w", err)
	}

	var configMap map[string]any
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return false, fmt.Errorf("failed to parse config file: %w", err)
	}

	_, exists := configMap["cloud"]
	return exists, nil
}

// addCloudKey reads a YAML file and sets the "cloud" key to the given value,
// then writes the file back.
func addCloudKey(configFilePath, cloud string) error {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var configMap map[string]any
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	configMap["cloud"] = cloud

	updatedData, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	if err := os.WriteFile(configFilePath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write updated config file: %w", err)
	}

	return nil
}
