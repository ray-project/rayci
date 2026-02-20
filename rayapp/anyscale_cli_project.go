package rayapp

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// ProjectInfo represents the Project information returned from the CLI.
type ProjectInfo struct {
	Name string `yaml:"name"`
	ID   string `yaml:"id"`
}

// GetDefaultProject retrieves the default Project from the Anyscale CLI.
// Returns the Project name and ID from the YAML output.
func (ac *AnyscaleCLI) GetDefaultProject(cloudID string) (*ProjectInfo, error) {
	args := []string{"project", "get-default", "--cloud", cloudID}
	output, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return nil, fmt.Errorf("get default Project failed: %w", err)
	}

	var ProjectInfo ProjectInfo
	if err := yaml.Unmarshal([]byte(output), &ProjectInfo); err != nil {
		return nil, fmt.Errorf("failed to parse Project info: %w", err)
	}

	return &ProjectInfo, nil
}
