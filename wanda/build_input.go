package wanda

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

func resolveBuildArgs(buildArgs []string) map[string]string {
	m := make(map[string]string)
	for _, s := range buildArgs {
		k, v, ok := strings.Cut(s, "=")
		if ok {
			m[k] = v
		} else {
			m[s] = os.Getenv(s)
		}
	}
	return m
}

type imageSource struct {
	name  string
	id    string
	src   string // where to fetch this image from
	local string // local reference/tag
}

type buildInput struct {
	context   *tarStream
	froms     map[string]*imageSource
	buildArgs []string

	tags map[string]struct{}
}

func newBuildInput(context *tarStream, buildArgs []string) *buildInput {
	return &buildInput{
		context:   context,
		tags:      make(map[string]struct{}),
		froms:     make(map[string]*imageSource),
		buildArgs: buildArgs,
	}
}

func (i *buildInput) addTag(tag string) {
	i.tags[tag] = struct{}{}
}

func (i *buildInput) tagList() []string {
	var tags []string
	for tag := range i.tags {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

type buildInputCore struct {
	Dockerfile   string            // Name of the Dockerfile to use.
	Froms        map[string]string // Map from image names to image digests.
	BuildContext string            // Digests of the build context.
	BuildArgs    map[string]string // Resolved build args.
}

func (i *buildInput) makeCore(dockerfile string) (*buildInputCore, error) {
	context, err := i.context.digest()
	if err != nil {
		return nil, fmt.Errorf("compute build context digest: %w", err)
	}

	froms := make(map[string]string)
	for name, src := range i.froms {
		froms[name] = src.id
	}

	buildArgs := resolveBuildArgs(i.buildArgs)

	core := &buildInputCore{
		Dockerfile:   dockerfile,
		Froms:        froms,
		BuildContext: context,
		BuildArgs:    buildArgs,
	}

	return core, nil
}

func (c *buildInputCore) digest() (string, error) {
	bs, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal build input: %w", err)
	}
	return sha256Digest(bs), nil
}