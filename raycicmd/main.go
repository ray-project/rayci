// Command rayci generates buildkite pipeline definitions from yaml files
// under the .buildkite/ directory. It scans for .buildkite/*.rayci.yml files
// and forms the pipeline definition from them.
package raycicmd

import (
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
)

// Flags is the structure for all the command the flags of rayci.
type Flags struct {
	RepoDir    string // flag -repo
	ConfigFile string // flag -config
}

// Main runs tha main function of rayci command.
func Main(flags *Flags, envs Envs) error {
	if envs == nil {
		envs = &osEnvs{}
	}

	config, err := loadConfig(flags.ConfigFile, envs)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pipeline, err := makePipeline(flags.RepoDir, config)
	if err != nil {
		return fmt.Errorf("make pipeline: %w", err)
	}

	enc := yaml.NewEncoder(os.Stdout)
	if err := enc.Encode(pipeline); err != nil {
		return fmt.Errorf("output pipeline: %w", err)
	}

	return nil
}
