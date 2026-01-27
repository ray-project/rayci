package wanda

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
)

func resolveBuildArgs(buildArgs []string, lookup lookupFunc) map[string]string {
	m := make(map[string]string)
	for _, s := range buildArgs {
		k, v, ok := strings.Cut(s, "=")
		if ok {
			m[k] = v
		} else {
			if lookup != nil {
				v, _ := lookup(s)
				m[s] = v
			} else {
				m[s] = os.Getenv(s)
			}
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
	// Epoch changes from time to time.
	// When it changes, the cache is invalidated and the build reruns.
	Epoch string `json:",omitempty"`

	Dockerfile   string            // Name of the Dockerfile to use.
	Froms        map[string]string // Map from image names to image digests.
	BuildContext string            // Digests of the build context.
	BuildArgs    map[string]string // Resolved build args.

	Platform string `json:",omitempty"` // "amd64" (empty string) or GOARCH
	OS       string `json:",omitempty"` // "linux" (empty string) or GOOS
}

func (i *buildInput) makeCore(dockerfile string, lookup lookupFunc) (*buildInputCore, error) {
	context := ""
	if i.context != nil {
		d, err := i.context.digest()
		if err != nil {
			return nil, fmt.Errorf("compute build context digest: %w", err)
		}
		context = d
	}

	froms := make(map[string]string)
	for name, src := range i.froms {
		froms[name] = src.id
	}

	buildArgs := resolveBuildArgs(i.buildArgs, lookup)

	platform := runtime.GOARCH
	if platform == "amd64" {
		platform = ""
	}

	os := runtime.GOOS
	if os == "linux" {
		os = ""
	}

	core := &buildInputCore{
		Dockerfile:   dockerfile,
		Froms:        froms,
		BuildContext: context,
		BuildArgs:    buildArgs,
		Platform:     platform,
		OS:           os,
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

type buildInputHints struct {
	BuildArgs map[string]string
}

func newBuildInputHints(buildArgs []string, lookup lookupFunc) *buildInputHints {
	return &buildInputHints{
		BuildArgs: resolveBuildArgs(buildArgs, lookup),
	}
}
