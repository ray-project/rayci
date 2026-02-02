package wanda

import (
	"os"
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

func TestDockerCmdRunExtract(t *testing.T) {
	cmd := newDockerCmd(&dockerCmdConfig{})

	const testImage = "alpine:latest"

	if err := cmd.run("pull", testImage); err != nil {
		t.Fatalf("pull image: %v", err)
	}

	tmpDir := t.TempDir()

	script := strings.Join([]string{
		"mkdir -p /artifacts/etc",
		"cp /etc/alpine-release /artifacts/etc/ || echo 'warning: not found'",
		"cp /etc/*.conf /artifacts/etc/ || echo 'warning: not found'",
	}, "\n")

	if err := cmd.runExtract(testImage, tmpDir, script); err != nil {
		t.Fatalf("runExtract: %v", err)
	}

	if _, err := os.Stat(tmpDir + "/etc/alpine-release"); os.IsNotExist(err) {
		t.Error("alpine-release was not copied")
	}

	entries, err := os.ReadDir(tmpDir + "/etc")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	foundConf := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".conf") {
			foundConf = true
			break
		}
	}

	if !foundConf {
		t.Error("no .conf files were copied")
	}
}

func TestDockerCmdRunExtract_bestEffort(t *testing.T) {
	cmd := newDockerCmd(&dockerCmdConfig{})

	const testImage = "alpine:latest"

	if err := cmd.run("pull", testImage); err != nil {
		t.Fatalf("pull image: %v", err)
	}

	tmpDir := t.TempDir()

	// Missing files should not fail - extraction is best-effort
	script := strings.Join([]string{
		"mkdir -p /artifacts/etc",
		"cp /etc/alpine-release /artifacts/etc/ || echo 'warning: not found'",
		"cp /nonexistent/file /artifacts/ || echo 'warning: not found'",
	}, "\n")

	if err := cmd.runExtract(testImage, tmpDir, script); err != nil {
		t.Fatalf("runExtract should not fail (best-effort): %v", err)
	}

	if _, err := os.Stat(tmpDir + "/etc/alpine-release"); os.IsNotExist(err) {
		t.Error("alpine-release was not copied")
	}
}
