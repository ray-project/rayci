package rayapp

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// generateComputeConfigName converts a config path to a config name.
// e.g., "configs/basic-single-node/aws.yaml" -> "basic-single-node-aws"
func generateComputeConfigName(configPath string) string {
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

// versionLegacyRe matches optional spaces, then "version" = "legacy" (zero or more spaces between each), in a comment line.
var versionLegacyRe = regexp.MustCompile(` *version *= *legacy`)

// isLegacyComputeConfigFormat checks if a YAML file uses the legacy compute config format.
// Primary check: a comment line containing "version=legacy".
// Secondary check: legacy-style keys "head_node_type" or "worker_node_types" in the YAML.
func isLegacyComputeConfigFormat(configFilePath string) (bool, error) {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to read config file: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if len(trimmed) > 0 && trimmed[0] == '#' {
			if versionLegacyRe.MatchString(trimmed[1:]) {
				return true, nil
			}
		}
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
