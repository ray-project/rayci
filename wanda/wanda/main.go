package main

import (
	"flag"
	"log"

	"github.com/ray-project/rayci/wanda"
)

func main() {
	workDir := flag.String("work_dir", ".", "root directory for the build")
	docker := flag.String("docker", "", "path to the docker client binary")
	cacheRepo := flag.String("cache_repo", "", "cache container repository")
	readOnly := flag.Bool("read_only", false, "read-only cache repository")

	flag.Parse()

	args := flag.Args()

	if len(args) != 1 {
		log.Fatal("needs exactly one argument for the spec file")
	}

	config := &wanda.ForgeConfig{
		WorkDir:       *workDir,
		DockerBin:     *docker,
		CacheRepo:     *cacheRepo,
		ReadOnlyCache: *readOnly,
	}

	if err := wanda.Build(args[0], config); err != nil {
		log.Fatal(err)
	}
}
