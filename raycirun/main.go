package raycirun

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"
)

func parseTags(tags string) []string {
	if tags == "" {
		return nil
	}
	return strings.Split(tags, ",")
}

// Main is the entry point for the raycirun command.
func Main() {
	var (
		org      = flag.String("org", "ray-project", "organization to run")
		pipeline = flag.String(
			"pipeline", "microcheck", "buildkite pipeline to trigger",
		)

		branch = flag.String("branch", "", "branch to run")
		pr     = flag.String("pr", "", "pr to run")
		commit = flag.String("commit", "HEAD", "commit to run")

		tag = flag.String(
			"tags", "", "tags to run, separated by commas",
		)
	)

	flag.Parse()
	args := flag.Args()

	token := os.Getenv("BUILDKITE_TOKEN")
	if token == "" {
		log.Fatal("BUILDKITE_TOKEN is not set")
	}

	b := &Build{
		Org:      *org,
		Branch:   *branch,
		PR:       *pr,
		Commit:   *commit,
		Pipeline: *pipeline,
		Tags:     parseTags(*tag),
		Selects:  args,
	}

	ctx := context.Background()
	build, err := b.Create(ctx, token)
	if err != nil {
		log.Fatalf("failed to create build: %v", err)
	}
	log.Printf("Build created: %s", build.WebURL)
}
