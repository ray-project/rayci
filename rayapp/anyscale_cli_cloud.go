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

// GetCloud retrieves the cloud with the given name from the Anyscale CLI.
func (ac *AnyscaleCLI) GetCloud(name string) (*CloudInfo, error) {
	args := []string{"cloud", "get", "--name", name}
	output, err := ac.runAnyscaleCLI(args)
	if err != nil {
		return nil, fmt.Errorf("get cloud %q failed: %w", name, err)
	}

	var cloudInfo CloudInfo
	if err := yaml.Unmarshal([]byte(output), &cloudInfo); err != nil {
		return nil, fmt.Errorf("failed to parse cloud info: %w", err)
	}
	// "cloud get" logs "Cloud not found." and exits 0 with nothing on
	// stdout, so an empty ID is the only sign the cloud does not exist.
	if cloudInfo.ID == "" {
		return nil, fmt.Errorf("cloud %q not found", name)
	}

	return &cloudInfo, nil
}
