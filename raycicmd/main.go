// Package raycicmd implements a command that generates buildkite pipeline
// definitions from yaml files under the .buildkite/ directory. It scans
// for .buildkite/*.rayci.yaml files and forms the pipeline definition from
// them.
package raycicmd

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Flags is the structure for all the command the flags of rayci.
type Flags struct {
	RepoDir        string // flag -repo
	ConfigFile     string // flag -config
	OutputFile     string // flag -output
	UploadPipeline bool   // flag -upload
	BuildkiteAgent string // flag -bkagent
	Select         string // flag -select
}

func parseFlags(args []string) (*Flags, []string) {
	set := flag.NewFlagSet("rayci", flag.ExitOnError)
	flags := new(Flags)
	set.StringVar(
		&flags.RepoDir, "repo", ".",
		"Path to the root of the repository.",
	)
	set.StringVar(
		&flags.ConfigFile, "config", "",
		"Path to the config file; empty means default config for ray repo.",
	)
	set.StringVar(
		&flags.OutputFile, "output", "pipeline.yaml",
		"Path to the output file; `-` means stdout.",
	)
	set.BoolVar(
		&flags.UploadPipeline, "upload", false,
		"Upload the pipeline using buildkite-agent.",
	)
	set.StringVar(
		&flags.BuildkiteAgent, "bkagent", "buildkite-agent",
		"Path to the buildkite-agent binary.",
	)
	set.StringVar(
		&flags.Select, "select", "",
		"Select specific jobs to run, separated by commas.",
	)

	if len(args) == 0 {
		set.Parse(nil)
	} else {
		set.Parse(args[1:])
	}

	return flags, set.Args()
}

func execWithInput(bin string, args []string, pipeline []byte) error {
	r := bytes.NewReader(pipeline)

	cmd := exec.Command(bin, args...)
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

// Main runs tha main function of rayci command.
func Main(args []string, envs Envs) error {
	flags, args := parseFlags(args)
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}

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

	rayciBranch, _ := envs.Lookup("RAYCI_BRANCH")
	commit := gitCommit(envs)

	selectStr := flags.Select
	if selectStr == "" {
		if v, ok := envs.Lookup("RAYCI_SELECT"); ok {
			selectStr = v
		}
	}
	selects := strings.FieldsFunc(selectStr, func(r rune) bool {
		return r == ','
	})

	info := &buildInfo{
		buildID:        buildID,
		launcherBranch: rayciBranch,
		gitCommit:      commit,
		selects:        selects,
	}

	pipeline, err := makePipeline(flags.RepoDir, config, info)
	if err != nil {
		return fmt.Errorf("make pipeline: %w", err)
	}

	// Upload pipeline to buildkite.
	bs, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("marshal pipeline: %w", err)
	}

	if !flags.UploadPipeline {
		if flags.OutputFile == "-" {
			if _, err := os.Stdout.Write(bs); err != nil {
				return fmt.Errorf("print pipeline: %w", err)
			}
		} else {
			if err := os.WriteFile(flags.OutputFile, bs, 0644); err != nil {
				return fmt.Errorf("write pipeline: %w", err)
			}
		}
	} else {
		// Prints out the pipeline content to logs.
		log.Printf("%s", bs)

		args := []string{"pipeline", "upload"}
		if err := execWithInput(flags.BuildkiteAgent, args, bs); err != nil {
			return fmt.Errorf("upload pipeline: %w", err)
		}
	}

	return nil
}
