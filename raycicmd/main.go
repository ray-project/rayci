// Package raycicmd implements a command that generates buildkite pipeline
// definitions from yaml files under the .buildkite/ directory. It scans
// for .buildkite/*.rayci.yml files and forms the pipeline definition from them.
package raycicmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	yaml "gopkg.in/yaml.v3"
)

// Flags is the structure for all the command the flags of rayci.
type Flags struct {
	RepoDir        string // flag -repo
	ConfigFile     string // flag -config
	UploadPipeline bool   // flag -upload
	BuildkiteAgent string // flag -bkagent
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

	if !flags.UploadPipeline {
		enc := yaml.NewEncoder(os.Stdout)
		if err := enc.Encode(pipeline); err != nil {
			return fmt.Errorf("output pipeline: %w", err)
		}
		return nil
	}

	// Upload pipeline to buildkite.
	bs, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("marshal pipeline: %w", err)
	}

	r := bytes.NewReader(bs)

	cmd := exec.Command(flags.BuildkiteAgent, "pipeline", "upload")
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upload pipeline: %w", err)
	}

	return nil
}
