package wanda

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// ContainerCmd is the interface for building container images across
// different container runtimes and builders.
type ContainerCmd interface {
	// setWorkDir sets the working directory for commands.
	setWorkDir(dir string)

	// run executes a command with the given arguments.
	run(args ...string) error

	// pull pulls an image and optionally tags it.
	pull(src, asTag string) error

	// inspectImage returns information about an image, or nil if not found.
	inspectImage(tag string) (*imageInfo, error)

	// tag tags an image.
	tag(src, asTag string) error

	// build builds an image from the given input.
	build(in *buildInput, core *buildInputCore, hints *buildInputHints) error
}

// imageInfo contains information about a container image.
type imageInfo struct {
	ID          string `json:"Id"`
	RepoDigests []string
	RepoTags    []string
}

// baseContainerCmd provides common functionality for container commands.
type baseContainerCmd struct {
	bin     string
	workDir string
	envs    []string
}

func (c *baseContainerCmd) setWorkDir(dir string) { c.workDir = dir }

func (c *baseContainerCmd) cmd(args ...string) *exec.Cmd {
	cmd := exec.Command(c.bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = c.envs
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}
	return cmd
}

func (c *baseContainerCmd) run(args ...string) error {
	return c.cmd(args...).Run()
}

func (c *baseContainerCmd) pull(src, asTag string) error {
	if err := c.run("pull", src); err != nil {
		return fmt.Errorf("pull %s: %w", src, err)
	}
	if src != asTag {
		if err := c.tag(src, asTag); err != nil {
			return fmt.Errorf("tag %s %s: %w", src, asTag, err)
		}
	}
	return nil
}

func (c *baseContainerCmd) inspectImage(tag string) (*imageInfo, error) {
	cmd := c.cmd("image", "inspect", tag)
	buf := new(bytes.Buffer)
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Docker returns 1, podman returns 125 for "image not found".
			code := exitErr.ExitCode()
			if code == 1 || code == 125 {
				return nil, nil
			}
		}
		return nil, err
	}
	var info []*imageInfo
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		return nil, fmt.Errorf("unmarshal image info: %w", err)
	}
	if len(info) != 1 {
		return nil, fmt.Errorf("%d image(s) found, expect 1", len(info))
	}
	return info[0], nil
}

func (c *baseContainerCmd) tag(src, asTag string) error {
	return c.run("tag", src, asTag)
}

func (c *baseContainerCmd) build(in *buildInput, core *buildInputCore, hints *buildInputHints) error {
	return c.doBuild(in, core, hints, nil)
}

// doBuild is the common build implementation that accepts extra flags.
func (c *baseContainerCmd) doBuild(in *buildInput, core *buildInputCore, hints *buildInputHints, extraFlags []string) error {
	if hints == nil {
		hints = newBuildInputHints(nil)
	}

	// Pull down the required images, and tag them properly.
	var froms []string
	for from := range core.Froms {
		froms = append(froms, from)
	}
	sort.Strings(froms)

	for _, from := range froms {
		src, ok := in.froms[from]
		if !ok {
			return fmt.Errorf("missing base image source for %q", from)
		}
		if src.local != "" { // local image, already ready.
			continue
		}
		if err := c.pull(src.src, src.name); err != nil {
			return fmt.Errorf("pull %s(%s): %w", src.name, src.src, err)
		}
	}

	// Build the image.
	var args []string
	args = append(args, "build")
	args = append(args, extraFlags...)
	args = append(args, "-f", core.Dockerfile)

	for _, t := range in.tagList() {
		args = append(args, "-t", t)
	}

	buildArgs := make(map[string]string)
	for k, v := range hints.BuildArgs {
		buildArgs[k] = v
	}
	// non-hint args can overwrite hint args
	for k, v := range core.BuildArgs {
		buildArgs[k] = v
	}

	var buildArgKeys []string
	for k := range buildArgs {
		buildArgKeys = append(buildArgKeys, k)
	}
	sort.Strings(buildArgKeys)
	for _, k := range buildArgKeys {
		v := buildArgs[k]
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	// read context from stdin
	args = append(args, "-")

	log.Printf("%s %s", c.bin, strings.Join(args, " "))

	buildCmd := c.cmd(args...)
	if in.context != nil {
		buildCmd.Stdin = newWriterToReader(in.context)
	}

	return buildCmd.Run()
}
