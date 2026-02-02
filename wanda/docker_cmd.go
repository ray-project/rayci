package wanda

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func dockerCmdEnvs() []string {
	var envs []string
	for _, k := range []string{
		"HOME",
		"USER",
		"PATH",
		"DOCKER_CONFIG",
		"AWS_REGION",
	} {
		if v, ok := os.LookupEnv(k); ok {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return envs
}

type dockerCmd struct {
	bin     string
	workDir string

	envs []string

	useLegacyEngine bool
}

type dockerCmdConfig struct {
	bin string

	useLegacyEngine bool
}

func newDockerCmd(config *dockerCmdConfig) *dockerCmd {
	bin := config.bin
	if bin == "" {
		bin = "docker"
	}
	envs := dockerCmdEnvs()

	if config.useLegacyEngine {
		envs = append(envs, "DOCKER_BUILDKIT=0")
	} else {
		// Default using buildkit.
		envs = append(envs, "DOCKER_BUILDKIT=1")
	}

	return &dockerCmd{
		bin:             bin,
		envs:            envs,
		useLegacyEngine: config.useLegacyEngine,
	}
}

func (c *dockerCmd) setWorkDir(dir string) { c.workDir = dir }

func (c *dockerCmd) cmd(args ...string) *exec.Cmd {
	cmd := exec.Command(c.bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = c.envs
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}
	return cmd
}

func (c *dockerCmd) run(args ...string) error {
	cmd := c.cmd(args...)
	return cmd.Run()
}

func (c *dockerCmd) pull(src, asTag string) error {
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

type dockerImageInfo struct {
	ID          string `json:"Id"`
	RepoDigests []string
	RepoTags    []string
}

func (c *dockerCmd) inspectImage(tag string) (*dockerImageInfo, error) {
	cmd := c.cmd("image", "inspect", tag)
	buf := new(bytes.Buffer)
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	var info []*dockerImageInfo
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		return nil, fmt.Errorf("unmarshal image info: %w", err)
	}
	if len(info) != 1 {
		return nil, fmt.Errorf("%d image(s) found, expect 1", len(info))
	}
	return info[0], nil
}

func (c *dockerCmd) tag(src, asTag string) error {
	return c.run("tag", src, asTag)
}

// createContainer creates a container from an image without starting it.
// Returns the container ID. A dummy command is provided for images without
// CMD/ENTRYPOINT. The command doesn't need to exist since the container is
// never started.
func (c *dockerCmd) createContainer(image string) (string, error) {
	cmd := c.cmd("create", image, "unused")
	buf := new(bytes.Buffer)
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// copyFromContainer copies a file or directory from a container to the host.
func (c *dockerCmd) copyFromContainer(containerID, src, dst string) error {
	return c.run("cp", containerID+":"+src, dst)
}

// removeContainer removes a container quietly (no stdout).
func (c *dockerCmd) removeContainer(containerID string) error {
	cmd := exec.Command(c.bin, "rm", containerID)
	cmd.Env = c.envs
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// listContainerFiles lists all files in a container using docker export.
func (c *dockerCmd) listContainerFiles(containerID string) ([]string, error) {
	exportCmd := exec.Command(c.bin, "export", containerID)
	exportCmd.Env = c.envs

	stdout, err := exportCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := exportCmd.Start(); err != nil {
		return nil, fmt.Errorf("start docker export: %w", err)
	}

	var files []string
	tr := tar.NewReader(stdout)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			exportCmd.Process.Kill()
			return nil, fmt.Errorf("read tar stream: %w", err)
		}
		files = append(files, "/"+strings.TrimPrefix(header.Name, "/"))
	}

	if err := exportCmd.Wait(); err != nil {
		return nil, fmt.Errorf("docker export: %w", err)
	}

	return files, nil
}

func (c *dockerCmd) build(in *buildInput, core *buildInputCore, hints *buildInputHints) error {
	if hints == nil {
		hints = newBuildInputHints(nil, nil)
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
	// TODO(aslonnie): maybe recheck all the IDs of the from images?

	// Build the image.
	var args []string
	args = append(args, "build")
	if !c.useLegacyEngine {
		args = append(args, "--progress=plain")
	}
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

	log.Printf("docker %s", strings.Join(args, " "))

	buildCmd := c.cmd(args...)
	if in.context != nil {
		buildCmd.Stdin = newWriterToReader(in.context)
	}

	return buildCmd.Run()
}
