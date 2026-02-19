package rayapp

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

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
