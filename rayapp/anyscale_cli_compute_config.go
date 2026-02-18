package rayapp

import (
	"encoding/json"
	"fmt"
	"os"
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

// CreateComputeConfig creates a new compute config from a YAML file if it doesn't already exist.
// If the config file uses the old format (head_node_type, worker_node_types), it will
// have the cloud added to it if missing.
// name: the name for the compute config (without version tag)
// configFilePath: path to the YAML config file
func (ac *AnyscaleCLI) CreateComputeConfig(name, configFilePath string) error {
	foundComputeConfigs, err := ac.ListComputeConfigs(&name)
	if err != nil {
		return fmt.Errorf("list compute configs failed: %w", err)
	}
	if len(foundComputeConfigs) > 0 {
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
			if err := tmpFile.Close(); err != nil {
				return fmt.Errorf("failed to close temp file: %w", err)
			}
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
			fmt.Printf("Temp copy: %s\n", actualConfigPath)
		}
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
	id, ok := m["id"].(string)
	if !ok {
		return li, fmt.Errorf("missing or non-string field \"id\"")
	}
	li.ID = id
	name, ok := m["name"].(string)
	if !ok {
		return li, fmt.Errorf("missing or non-string field \"name\"")
	}
	li.Name = name
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
