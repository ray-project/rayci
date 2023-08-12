package wanda

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type buildInput struct {
	Dockerfile   string            // Name of the Dockerfile to use.
	Froms        map[string]string // Map from image names to image digests.
	BuildContext string            // Digests of the build context.
	BuildArgs    map[string]string // Resolved build args.
}

func (i *buildInput) digest() (string, error) {
	bs, err := json.Marshal(i)
	if err != nil {
		return "", fmt.Errorf("marshal build input: %w", err)
	}
	return sha256Digest(bs), nil
}

func dockerCmdEnvs() []string {
	var envs []string
	for _, k := range []string{
		"HOME",
		"USER",
	} {
		if v, ok := os.LookupEnv(k); ok {
			envs = append(envs, fmt.Sprintf("%s=%s", k, v))
		}
	}
	envs = append(envs, "DOCKER_BUILDKIT=1")

	return envs
}

type dockerCmd struct {
	bin string

	envs []string
}

func newDockerCmd(bin string) *dockerCmd {
	if bin == "" {
		bin = "docker"
	}
	envs := dockerCmdEnvs()
	return &dockerCmd{bin: bin, envs: envs}
}

func (c *dockerCmd) cmd(args ...string) *exec.Cmd {
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = c.envs
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
		if err := c.run("tag", src, asTag); err != nil {
			return fmt.Errorf("tag %s %s: %w", src, asTag, err)
		}
	}

	return nil
}

func (c *dockerCmd) build(in *buildInput, context *tarStream, tags []string) error {
	// Pull down the required images, and tag them properly.
	var froms []string
	for from := range in.Froms {
		if strings.HasPrefix(from, "@") {
			// A local image, no need to pull.
			continue
		}
		froms = append(froms, from)
	}
	sort.Strings(froms)
	for _, ref := range froms {
		srcRef := in.Froms[ref]
		if srcRef == "" {
			srcRef = ref
		}
		if err := c.pull(srcRef, ref); err != nil {
			return fmt.Errorf("pull %s(%s): %w", ref, srcRef, err)
		}
	}

	// Build the image.
	var args []string

	args = append(args, "build", "--progress=plain")
	args = append(args, "-f", in.Dockerfile)

	for _, t := range tags {
		args = append(args, "-t", t)
	}

	var buildArgKeys []string
	for k := range in.BuildArgs {
		buildArgKeys = append(buildArgKeys, k)
	}
	sort.Strings(buildArgKeys)
	for _, k := range buildArgKeys {
		v := in.BuildArgs[k]
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, "-") // read context from stdin

	log.Printf("docker %s", strings.Join(args, " "))

	buildCmd := c.cmd(args...)
	buildCmd.Stdin = newWriterToReader(context)

	return buildCmd.Run()
}
