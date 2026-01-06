package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ray-project/rayci/wanda"
)

const usage = `
wanda - container image builder for RayCI using a container registry as a
content-addressed build cache.

Usage:
  wanda [flags] <spec.wanda.yaml>      Build an image (with dependencies)
  wanda deps <spec.wanda.yaml>         Show dependency build order

Modes:
- Remote:
   Enabled by setting -rayci flag. Takes RAYCI_ env vars for input and runs in
   remote mode. Builds and uploads image to the cache repository.
- Local:
   Takes exactly one argument for the spec file and builds the image for local
   use only.

Flags:
`

func main() {
	// Check for subcommand before flag parsing
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		switch os.Args[1] {
		case "deps":
			runDeps(os.Args[2:])
			return
		}
	}

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
		*rebuild = os.Getenv("RAYCI_WANDA_ALWAYS_REBUILD") == "true"

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

	if err := wanda.BuildWithDeps(input, config); err != nil {
		log.Fatal(err)
	}
}

func runDeps(args []string) {
	if len(args) != 1 {
		log.Fatal("usage: wanda deps <spec.wanda.yaml>")
	}

	graph, err := wanda.BuildDepGraph(args[0], os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}

	if err := graph.ValidateDeps(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Build order:")
	for i, name := range graph.Order() {
		rs := graph.Get(name)
		fmt.Printf("  %d. %s (%s)\n", i+1, name, rs.Path)
	}
}
