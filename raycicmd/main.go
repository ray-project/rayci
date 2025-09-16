// Package raycicmd implements a command that generates buildkite pipeline
// definitions from yaml files under the .buildkite/ directory. It scans
// for .buildkite/*.rayci.yaml files and forms the pipeline definition from
// them.
package raycicmd

import (
	"bytes"
	"flag"
	"fmt"
	"io"
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
	BuildkiteDir   string // flag -buildkite-dir
	OutputFile     string // flag -output
	UploadPipeline bool   // flag -upload
	BuildkiteAgent string // flag -bkagent
	Select         string // flag -select
}

func parseFlags(args []string) (*Flags, []string, error) {
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
		"Select specific step IDs or keys to run, separated by commas.",
	)
	set.StringVar(
		&flags.BuildkiteDir, "buildkite-dir", "",
		"Path to the buildkite pipeline files; "+
			"if empty, will use the directory from the config file.",
	)

	if len(args) == 0 {
		err := set.Parse(nil)
		if err != nil {
			return nil, nil, fmt.Errorf("parse flags: %w", err)
		}
	} else {
		err := set.Parse(args[1:])
		if err != nil {
			return nil, nil, fmt.Errorf("parse flags: %w", err)
		}
	}

	return flags, set.Args(), nil
}

func execWithInput(
	bin string, args []string, pipeline []byte, stdout io.Writer,
) error {
	r := bytes.NewReader(pipeline)

	cmd := exec.Command(bin, args...)
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	cmd.Stdout = stdout

	return cmd.Run()
}

func stepSelects(s string, envs Envs) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		if v, ok := envs.Lookup("RAYCI_SELECT"); ok {
			s = strings.TrimSpace(v)
		}
	}

	selects := strings.FieldsFunc(s, func(r rune) bool {
		return r == ','
	})
	if len(selects) == 0 {
		return nil
	}
	return selects
}

func makeBuildInfo(flags *Flags, envs Envs) (*buildInfo, error) {
	buildID, err := makeBuildID(envs)
	if err != nil {
		return nil, fmt.Errorf("make build id: %w", err)
	}

	rayciBranch, _ := envs.Lookup("RAYCI_BRANCH")

	// buildAuthorEmail is the email of the user who triggered the
	// buildkite webhook event; for most parts, it is the same as the
	// github author email.
	buildAuthorEmail, _ := envs.Lookup("BUILDKITE_BUILD_CREATOR_EMAIL")
	commit := gitCommit(envs)
	selects := stepSelects(flags.Select, envs)

	return &buildInfo{
		buildID:          buildID,
		buildAuthorEmail: buildAuthorEmail,
		launcherBranch:   rayciBranch,
		gitCommit:        commit,
		selects:          selects,
	}, nil
}

func loadConfig(configFile, buildkiteDir string, envs Envs) (*config, error) {
	var config *config
	if configFile == "" {
		config = defaultConfig(envs)
	} else {
		c, err := loadConfigFromFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("load config from file: %w", err)
		}
		config = c
	}

	if buildkiteDir != "" {
		config.BuildkiteDirs = strings.Split(buildkiteDir, ":")
	}

	return config, nil
}

// Main runs tha main function of rayci command.
func Main(args []string, envs Envs) error {
	flags, args, err := parseFlags(args)
	if err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if len(args) != 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}

	if envs == nil {
		envs = &osEnvs{}
	}

	config, err := loadConfig(flags.ConfigFile, flags.BuildkiteDir, envs)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	info, err := makeBuildInfo(flags, envs)
	if err != nil {
		return fmt.Errorf("make build info: %w", err)
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
		agent := flags.BuildkiteAgent
		if err := execWithInput(agent, args, bs, nil); err != nil {
			return fmt.Errorf("upload pipeline: %w", err)
		}
	}

	return nil
}
