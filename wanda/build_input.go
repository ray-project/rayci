package wanda

import (
	"encoding/json"
	"fmt"
	"sort"
)

type buildInputCore struct {
	Dockerfile   string            // Name of the Dockerfile to use.
	Froms        map[string]string // Map from image names to image digests.
	BuildContext string            // Digests of the build context.
	BuildArgs    map[string]string // Resolved build args.
}

type fromSource struct {
	id   string
	srcs []string
}

type buildInput struct {
	context *tarStream
	froms   map[string]*fromSource

	tags map[string]struct{}
}

func newBuildInput(context *tarStream) *buildInput {
	return &buildInput{
		context: context,
		tags:    make(map[string]struct{}),
		froms:   make(map[string]*fromSource),
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

func (i *buildInput) makeCore(
	dockerfile string, buildArgs map[string]string,
) (*buildInputCore, error) {
	context, err := i.context.digest()
	if err != nil {
		return nil, fmt.Errorf("compute build context digest: %w", err)
	}

	froms := make(map[string]string)
	for name, src := range i.froms {
		froms[name] = src.id
	}

	core := &buildInputCore{
		Dockerfile:   dockerfile,
		Froms:        froms,
		BuildContext: context,
		BuildArgs:    buildArgs,
	}

	return core, nil
}

func (i *buildInputCore) digest() (string, error) {
	bs, err := json.Marshal(i)
	if err != nil {
		return "", fmt.Errorf("marshal build input: %w", err)
	}
	return sha256Digest(bs), nil
}
