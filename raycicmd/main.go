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
		"Select specific step IDs or keys to run, separated by commas.",
	)
	set.StringVar(
		&flags.BuildkiteDir, "buildkite-dir", "",
		"Path to the buildkite pipeline files; "+
			"if empty, will use the directory from the config file.",
	)

	if len(args) == 0 {
		set.Parse(nil)
	} else {
		set.Parse(args[1:])
	}

	return flags, set.Args()
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

func tagFilterConfig(envs Envs) []string {
	v, ok := envs.Lookup("RAYCI_TAG_FILTER_CONFIG")
	if !ok {
		return nil
	}

	var result []string
	for _, p := range strings.Split(v, ",") {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
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

	if tfc := tagFilterConfig(envs); tfc != nil {
		config.TagFilterConfig = tfc
	}

	return config, nil
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

	config, err := loadConfig(flags.ConfigFile, flags.BuildkiteDir, envs)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	info, err := makeBuildInfo(flags, envs)
	if err != nil {
		return fmt.Errorf("make build info: %w", err)
	}

	lister := &GitChangeLister{
		WorkDir:    flags.RepoDir,
		BaseBranch: getEnv(envs, "BUILDKITE_PULL_REQUEST_BASE_BRANCH"),
		Commit:     getEnv(envs, "BUILDKITE_COMMIT"),
	}
	pipeline, err := makePipeline(flags.RepoDir, config, info, envs, lister)
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
