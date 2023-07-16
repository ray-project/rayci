// Package raycicmd implements a command that generates buildkite pipeline
// definitions from yaml files under the .buildkite/ directory. It scans
// for .buildkite/*.rayci.yaml files and forms the pipeline definition from
// them.
package raycicmd

import (
	"bytes"
	"fmt"
	"log"
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

	buildID, err := makeBuildID(envs)
	if err != nil {
		return fmt.Errorf("make build id: %w", err)
	}

	pipeline, err := makePipeline(flags.RepoDir, config, buildID)
	if err != nil {
		return fmt.Errorf("make pipeline: %w", err)
	}

	// Upload pipeline to buildkite.
	bs, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("marshal pipeline: %w", err)
	}

	if !flags.UploadPipeline {
		if _, err := os.Stdout.Write(bs); err != nil {
			return fmt.Errorf("write pipeline: %w", err)
		}
		return nil
	}

	// Prints out the pipeline content to logs.
	log.Printf("%s", bs)

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
