package rayapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// parseComputeConfigName parses the AWS config path and converts it to a config name.
// e.g., "configs/basic-single-node/aws.yaml" -> "basic-single-node-aws"
func parseComputeConfigName(awsConfigPath string) string {
	dir := filepath.Dir(awsConfigPath)
	base := filepath.Base(awsConfigPath)
	ext := filepath.Ext(base)
	filename := strings.TrimSuffix(base, ext)
	configDir := filepath.Base(dir)
	return configDir + "-" + filename
}

// isOldComputeConfigFormat checks if a YAML file uses the old compute config format
// by looking for old-style keys like "head_node_type" or "worker_node_types".
func isOldComputeConfigFormat(configFilePath string) (bool, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to read config file: %w", err)
	}
	var configMap map[string]interface{}
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return false, fmt.Errorf("failed to parse config file: %w", err)
	}
	_, hasHeadNodeType := configMap["head_node_type"]
	_, hasWorkerNodeTypes := configMap["worker_node_types"]
	return hasHeadNodeType || hasWorkerNodeTypes, nil
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
	data, err := os.ReadFile(oldConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read old config file: %w", err)
	}
	var oldConfig OldComputeConfig
	if err := yaml.Unmarshal(data, &oldConfig); err != nil {
		return nil, fmt.Errorf("failed to parse old config: %w", err)
	}
	newConfig := NewComputeConfig{
		HeadNode: NewHeadNode{
			InstanceType: oldConfig.HeadNodeType.InstanceType,
		},
		AutoSelectWorkerConfig: true,
	}
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
