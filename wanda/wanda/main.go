package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ray-project/rayci/wanda"
)

const usage = `--------------------------------
wanda - container image builder for RayCI using a container registry as a content-addressed build cache.
--------------------------------
Runs in either remote mode or local mode.
* Remote mode: Enabled by setting -rayci flag. Takes RAYCI_ env vars for input and runs in remote mode. Builds and uploads image to the cache repository.
* Local mode: Takes exactly one argument for the spec file and builds the image for local use only.
--------------------------------
`

func main() {
	workDir := flag.String("work_dir", ".", "root directory for the build")
	docker := flag.String("docker", "", "path to the docker client binary")
	rayCI := flag.Bool(
		"rayci", false,
		"takes RAYCI_ env vars for input and run in remote mode",
	)
	workRepo := flag.String("work_repo", "", "cache container repository")
	namePrefix := flag.String(
		"name_prefix", "cr.ray.io/rayproject/",
		"prefix for the image name",
	)
	buildID := flag.String("build_id", "", "build ID for the image tag")
	readOnly := flag.Bool("read_only", false, "read-only cache repository")
	epoch := flag.String("epoch", "", "epoch for the image tag")
	rebuild := flag.Bool("rebuild", false, "always rebuild the image")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *rayCI {
		*workRepo = os.Getenv("RAYCI_WORK_REPO")
		*readOnly = os.Getenv("BUILDKITE_CACHE_READONLY") == "true"
		*buildID = os.Getenv("RAYCI_BUILD_ID")
		*namePrefix = os.Getenv("RAYCI_FORGE_PREFIX")

		if *epoch == "" {
			*epoch = wanda.DefaultCacheEpoch()
		}
	}

	args := flag.Args()

	var input string
	if !*rayCI {
		if len(args) != 1 {
			log.Fatal("needs exactly one argument for the spec file in local mode. Run with -help for usage.")
		}
		input = args[0]
	} else {
		input = os.Getenv("RAYCI_WANDA_FILE")
	}

	config := &wanda.ForgeConfig{
		WorkDir:    *workDir,
		DockerBin:  *docker,
		WorkRepo:   *workRepo,
		NamePrefix: *namePrefix,
		BuildID:    *buildID,
		Epoch:      *epoch,

		RayCI:   *rayCI,
		Rebuild: *rebuild,

		ReadOnlyCache: *readOnly,
	}

	if err := wanda.Build(input, config); err != nil {
		log.Fatal(err)
	}
}
