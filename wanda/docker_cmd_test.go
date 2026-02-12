package wanda

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
)

func TestDockerCmdBuild(t *testing.T) {
	cmd := newDockerCmd(&dockerCmdConfig{}) // uses real docker client

	ts := newTarStream()
	ts.addFile("Dockerfile.hello", nil, "testdata/Dockerfile.hello")

	const tag = "cr.ray.io/rayproject/wanda-test"

	buildArgs := []string{"MESSAGE=test mesasge"}
	input := newBuildInput(ts, buildArgs)
	input.addTag(tag)

	core, err := input.makeCore("Dockerfile.hello", nil)
	if err != nil {
		t.Fatalf("make build input core: %v", err)
	}

	hints := newBuildInputHints([]string{
		"REMOTE_CACHE_URL=http://localhost:5000",
		"MESSAGE=does not matter", // will be shadowed by the build args
	}, nil)

	if err := cmd.build(input, core, hints); err != nil {
		t.Fatalf("build: %v", err)
	}

	ref, err := name.ParseReference(tag)
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	img, err := daemon.Image(ref)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}

	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("read layers: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}

	config, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	// Check message env value, this is set by the build args.
	messageEnv := ""
	t.Log(config.Config.Env)
	for _, env := range config.Config.Env {
		if strings.HasPrefix(env, "MESSAGE=") {
			messageEnv = env
			break
		}
	}

	if messageEnv != "MESSAGE=test mesasge" {
		t.Errorf("MESSAGE env got %q, want `MESSAGE=test mesasge`", messageEnv)
	}
}

func TestDockerCmdBuild_withHints(t *testing.T) {
	cmd := newDockerCmd(&dockerCmdConfig{}) // uses real docker client

	ts := newTarStream()
	ts.addFile("Dockerfile.hello", nil, "testdata/Dockerfile.hello")

	const tag = "cr.ray.io/rayproject/wanda-test"

	input := newBuildInput(ts, nil)
	input.addTag(tag)

	core, err := input.makeCore("Dockerfile.hello", nil)
	if err != nil {
		t.Fatalf("make build input core: %v", err)
	}

	hints := newBuildInputHints([]string{
		"REMOTE_CACHE_URL=http://localhost:5000",
		"MESSAGE=hint message", // will be shadowed by the build args
	}, nil)

	if err := cmd.build(input, core, hints); err != nil {
		t.Fatalf("build: %v", err)
	}

	ref, err := name.ParseReference(tag)
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	img, err := daemon.Image(ref)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}

	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("read layers: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}

	config, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	// Check message env value, this is set by the build args.
	messageEnv := ""
	t.Log(config.Config.Env)
	for _, env := range config.Config.Env {
		if strings.HasPrefix(env, "MESSAGE=") {
			messageEnv = env
			break
		}
	}

	if messageEnv != "MESSAGE=hint message" {
		t.Errorf("MESSAGE env got %q, want `MESSAGE=hint message`", messageEnv)
	}
}

func TestDockerCmdCopyFromContainer(t *testing.T) {
	cmd := newDockerCmd(&dockerCmdConfig{})

	const testImage = "alpine:latest"

	if err := cmd.run("pull", testImage); err != nil {
		t.Fatalf("pull image: %v", err)
	}

	containerID, err := cmd.createContainer(testImage)
	if err != nil {
		t.Fatalf("createContainer: %v", err)
	}
	defer cmd.removeContainer(containerID)

	tmpDir := t.TempDir()

	// Copy a known file from the container
	if err := cmd.copyFromContainer(containerID, "/etc/alpine-release", filepath.Join(tmpDir, "alpine-release")); err != nil {
		t.Fatalf("copyFromContainer: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "alpine-release")); os.IsNotExist(err) {
		t.Error("alpine-release was not copied")
	}
}

func TestDockerCmdCopyFromContainer_directory(t *testing.T) {
	cmd := newDockerCmd(&dockerCmdConfig{})

	const testImage = "alpine:latest"

	if err := cmd.run("pull", testImage); err != nil {
		t.Fatalf("pull image: %v", err)
	}

	containerID, err := cmd.createContainer(testImage)
	if err != nil {
		t.Fatalf("createContainer: %v", err)
	}
	defer cmd.removeContainer(containerID)

	tmpDir := t.TempDir()

	// Copy a directory from the container
	if err := cmd.copyFromContainer(containerID, "/etc", filepath.Join(tmpDir, "etc")); err != nil {
		t.Fatalf("copyFromContainer: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "etc", "alpine-release")); os.IsNotExist(err) {
		t.Error("alpine-release was not copied from /etc directory")
	}
}

func TestDockerCmdCopyFromContainer_directoryContents(t *testing.T) {
	cmd := newDockerCmd(&dockerCmdConfig{})

	const testImage = "alpine:latest"

	if err := cmd.run("pull", testImage); err != nil {
		t.Fatalf("pull image: %v", err)
	}

	containerID, err := cmd.createContainer(testImage)
	if err != nil {
		t.Fatalf("createContainer: %v", err)
	}
	defer cmd.removeContainer(containerID)

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Trailing slash on src copies contents, not the directory itself.
	if err := cmd.copyFromContainer(containerID, "/etc/apk/", dst); err != nil {
		t.Fatalf("copyFromContainer: %v", err)
	}

	// keys/ should be directly in dst, not in dst/apk/.
	if _, err := os.Stat(filepath.Join(dst, "keys")); os.IsNotExist(err) {
		t.Error("keys/ should be directly in dst when src has trailing slash")
	}
	if _, err := os.Stat(filepath.Join(dst, "apk")); !os.IsNotExist(err) {
		t.Error("apk/ subdirectory should not exist when src has trailing slash")
	}
}

func TestDockerCmdCopyFromContainer_notFound(t *testing.T) {
	cmd := newDockerCmd(&dockerCmdConfig{})

	const testImage = "alpine:latest"

	if err := cmd.run("pull", testImage); err != nil {
		t.Fatalf("pull image: %v", err)
	}

	containerID, err := cmd.createContainer(testImage)
	if err != nil {
		t.Fatalf("createContainer: %v", err)
	}
	defer cmd.removeContainer(containerID)

	tmpDir := t.TempDir()

	// Copying a non-existent file should fail
	if err := cmd.copyFromContainer(containerID, "/nonexistent/file", filepath.Join(tmpDir, "file")); err == nil {
		t.Error("copyFromContainer should fail for non-existent file")
	}
}
