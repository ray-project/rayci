package wanda

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func sha256Sum(bs []byte) string {
	h := sha256.New()
	h.Write(bs)
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

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
	return sha256Sum(bs), nil
}

func builderEnvs() []string {
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

func dockerCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func pullDocker(src, asTag string) error {
	cmd := dockerCmd("pull", src)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker pull %s: %w", src, err)
	}

	if src != asTag {
		cmd := dockerCmd("tag", src, asTag)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker tag %s %s: %w", src, asTag, err)
		}
	}

	return nil
}

func buildDocker(in *buildInput, context *tarStream, tags []string) error {
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
		if err := pullDocker(srcRef, ref); err != nil {
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

	cmd := dockerCmd(args...)
	cmd.Stdin = newWriterToReader(context)
	cmd.Env = builderEnvs()

	return cmd.Run()
}
