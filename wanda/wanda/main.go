package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/ray-project/rayci/wanda"
)

func main() {
	workDir := flag.String("work_dir", ".", "root directory for the build")
	docker := flag.String("docker", "", "path to the docker client binary")
	rayCI := flag.Bool("rayci", false, "takes RAYCI_ env vars for input")
	workRepo := flag.String("work_repo", "", "cache container repository")
	namePrefix := flag.String("name_prefix", "", "prefix for the image name")
	buildID := flag.String("build_id", "", "build ID for the image tag")
	readOnly := flag.Bool("read_only", false, "read-only cache repository")
	epoch := flag.String("epoch", "", "epoch for the image tag")

	flag.Parse()

	if *rayCI {
		*workRepo = os.Getenv("RAYCI_WORK_REPO")
		*readOnly = os.Getenv("BUILDKITE_CACHE_READONLY") == "true"
		*buildID = os.Getenv("RAYCI_BUILD_ID")
		*namePrefix = os.Getenv("RAYCI_FORGE_PREFIX")
	}

	args := flag.Args()

	var input string
	if !*rayCI {
		if len(args) != 1 {
			log.Fatal("needs exactly one argument for the spec file")
		}
		input = args[0]
	} else {
		input = os.Getenv("RAYCI_WANDA_FILE")
	}

	if *epoch == "" {
		now := time.Now().UTC()
		*epoch = now.Format("20060102") // YYYYMMDD
	}

	config := &wanda.ForgeConfig{
		WorkDir:    *workDir,
		DockerBin:  *docker,
		WorkRepo:   *workRepo,
		NamePrefix: *namePrefix,
		BuildID:    *buildID,
		Epoch:      *epoch,

		RayCI: *rayCI,

		ReadOnlyCache: *readOnly,
	}

	if err := wanda.Build(input, config); err != nil {
		log.Fatal(err)
	}
}
