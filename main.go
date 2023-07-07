package main

import (
	"flag"
	"log"

	"github.com/ray-project/rayci/raycicmd"
)

func main() {
	flags := new(raycicmd.Flags)
	flag.StringVar(
		&flags.RepoDir, "repo", ".",
		"Path to the root of the repository.",
	)
	flag.StringVar(
		&flags.ConfigFile, "config", "",
		"Path to the config file; empty means default config for ray repo.",
	)
	flag.BoolVar(
		&flags.UploadPipeline, "upload", false,
		"Upload the pipeline using buildkite-agent.",
	)
	flag.StringVar(
		&flags.BuildkiteAgent, "bkagent", "buildkite-agent",
		"Path to the buildkite-agent binary.",
	)
	flag.Parse()

	if err := raycicmd.Main(flags, nil); err != nil {
		log.Fatal(err)
	}
}
