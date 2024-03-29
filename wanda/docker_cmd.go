package wanda

import (
	"fmt"
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

func (c *dockerCmd) tag(src, asTag string) error {
	return c.run("tag", src, asTag)
}

func (c *dockerCmd) build(in *buildInput, core *buildInputCore) error {
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

	var buildArgKeys []string
	for k := range core.BuildArgs {
		buildArgKeys = append(buildArgKeys, k)
	}
	sort.Strings(buildArgKeys)
	for _, k := range buildArgKeys {
		v := core.BuildArgs[k]
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	if in.context != nil {
		args = append(args, "-") // read context from stdin
	} else {
		// copy everything
		args = append(args, ".")
	}

	log.Printf("docker %s", strings.Join(args, " "))

	buildCmd := c.cmd(args...)
	if in.context != nil {
		buildCmd.Stdin = newWriterToReader(in.context)
	}

	return buildCmd.Run()
}
