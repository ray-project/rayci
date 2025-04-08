package raycirun

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/buildkite/go-buildkite/v4"
)

var orgAliases = map[string]string{
	"r":   "ray-project",
	"ray": "ray-project",
	"as":  "anyscale",
	"p":   "anyscale",
}

// Build is the configuration for triggering a build on Buildkite.
type Build struct {
	// ServerBaseURL is the alternative base URL of the Buildkite server to use.
	// If not set, the default Buildkite API base URL will be used.
	ServerBaseURL *url.URL

	Org      string // The name of the organization to use.
	Pipeline string // The name of the pipeline to trigger.

	Branch string // The branch to trigger the build for.
	PR     string // The PR to trigger the build for.
	Commit string // The commit digest to trigger the build for.

	Message string // The message to use for the build.

	Tags    []string // The CI job tags to run.
	Selects []string // The CI job keys to run.
}

func (b *Build) branchName() string {
	if b.PR != "" {
		// When PR is set, we use the PR to reference the branch.
		// This is required for working with pull requests from forks.
		return fmt.Sprintf("refs/pull/%s/head", b.PR)
	}
	return b.Branch
}

func (b *Build) selectStr() string {
	var selects []string
	if len(b.Tags) > 0 {
		for _, tag := range b.Tags {
			selects = append(selects, fmt.Sprintf("tag:%s", tag))
		}
	}
	if len(b.Selects) > 0 {
		selects = append(selects, b.Selects...)
	}
	return strings.Join(selects, ",")
}

func (b *Build) orgName() string {
	if alias, ok := orgAliases[b.Org]; ok {
		return alias
	}
	return b.Org
}

func (b *Build) commit() string {
	if b.Commit == "" {
		return "HEAD"
	}
	return b.Commit
}

func (b *Build) message() string {
	const defaultMessage = "Build triggered by raycirun"
	if b.Message == "" {
		return defaultMessage
	}
	return b.Message
}

// Create creates a new build on Buildkite with the given parameters.
func (b *Build) Create(ctx context.Context, token string) (
	*buildkite.Build, error,
) {
	branchName := b.branchName()
	if branchName == "" {
		return nil, fmt.Errorf("branch or pr is required")
	}

	env := make(map[string]string)
	if selectStr := b.selectStr(); selectStr != "" {
		env["RAYCI_SELECT"] = selectStr
	}

	client, err := buildkite.NewOpts(buildkite.WithTokenAuth(token))
	if err != nil {
		return nil, fmt.Errorf("create buildkite client: %w", err)
	}
	if b.ServerBaseURL != nil {
		client.BaseURL = b.ServerBaseURL
	}

	orgName := b.orgName()

	req := &buildkite.CreateBuild{
		Message: b.message(),
		Commit:  b.commit(),
		Branch:  branchName,
		Env:     env,

		IgnorePipelineBranchFilters: true,
	}

	log.Printf("Creating build for %s/%s: %+v", orgName, b.Pipeline, req)

	build, _, err := client.Builds.Create(ctx, orgName, b.Pipeline, *req)
	if err != nil {
		return nil, fmt.Errorf("failed to create build: %w", err)
	}

	return &build, nil
}
