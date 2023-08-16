package wanda

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
)

func TestForge(t *testing.T) {
	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
	}

	if err := Build("testdata/hello.spec.yaml", config); err != nil {
		t.Fatalf("build: %v", err)
	}

	const tag = "cr.ray.io/rayproject/hello"

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
		t.Fatalf("got %d layers, want 1", len(layers))
	}
}
