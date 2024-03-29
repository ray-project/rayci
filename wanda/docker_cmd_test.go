package wanda

import (
	"testing"

	"strings"

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

	core, err := input.makeCore("Dockerfile.hello")
	if err != nil {
		t.Fatalf("make build input core: %v", err)
	}

	if err := cmd.build(input, core); err != nil {
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

func TestDockerCmdBuild_copyEverything(t *testing.T) {
	cmd := newDockerCmd(&dockerCmdConfig{}) // uses real docker client

	cmd.setWorkDir("testdata")

	const tag = "cr.ray.io/rayproject/wanda-test"

	buildArgs := []string{"MESSAGE=test mesasge"}
	input := newBuildInput(nil, buildArgs)
	input.addTag(tag)

	core, err := input.makeCore("Dockerfile.hello")
	if err != nil {
		t.Fatalf("make build input core: %v", err)
	}

	if err := cmd.build(input, core); err != nil {
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
