package rayapp

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// projectInfo represents the project information returned from the CLI.
type projectInfo struct {
	Name string `yaml:"name"`
	ID   string `yaml:"id"`
}

// getDefaultProject retrieves the default project from the Anyscale CLI.
func (ac *AnyscaleCLI) getDefaultProject(cloudID string) (*projectInfo, error) {
	args := []string{"project", "get-default", "--cloud", cloudID}
	output, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return nil, fmt.Errorf("get default project failed: %w", err)
	}

	var pInfo projectInfo
	if err := yaml.Unmarshal([]byte(output), &pInfo); err != nil {
		return nil, fmt.Errorf("failed to parse project info: %w", err)
	}

	return &pInfo, nil
}
