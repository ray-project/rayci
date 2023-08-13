package main

import (
	"flag"
	"log"
	"os"

	"github.com/ray-project/rayci/wanda"
)

func main() {
	workDir := flag.String("work_dir", ".", "root directory for the build")
	docker := flag.String("docker", "", "path to the docker client binary")
	rayCI := flag.Bool("rayci", false, "takes RAYCI_ env vars for input")
	workRepo := flag.String("work_repo", "", "cache container repository")
	readOnly := flag.Bool("read_only", false, "read-only cache repository")

	flag.Parse()

	if *rayCI {
		*workRepo = os.Getenv("RAYCI_WORK_REPO")
		*readOnly = os.Getenv("BUILDKITE_CACHE_READONLY") == "true"
	}

	args := flag.Args()

	if len(args) != 1 {
		log.Fatal("needs exactly one argument for the spec file")
	}

	config := &wanda.ForgeConfig{
		WorkDir:       *workDir,
		DockerBin:     *docker,
		WorkRepo:      *workRepo,
		ReadOnlyCache: *readOnly,
	}

	if err := wanda.Build(args[0], config); err != nil {
		log.Fatal(err)
	}
}
