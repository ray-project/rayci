package wanda

import (
	"testing"

	"archive/tar"
	"fmt"
	"io"
	"net/http/httptest"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

func TestForgeWithWorkRepo(t *testing.T) {
	if os.Getenv("BUILDKITE") == "true" {
		t.Log("does not work when the daemon cannot reach the local registry")
		t.Skip()
		return
	}

	cr := registry.New()
	server := httptest.NewServer(cr)
	defer server.Close()

	addr := server.Listener.Addr().String()

	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
		WorkRepo:   fmt.Sprintf("%s/work", addr),
		BuildID:    "abc123",
	}

	if err := Build("testdata/hello.spec.yaml", config); err != nil {
		t.Fatalf("build hello: %v", err)
	}

	if err := Build("testdata/world.spec.yaml", config); err != nil {
		t.Fatalf("build world: %v", err)
	}

	world := fmt.Sprintf("%s/work:abc123-world", addr)
	ref, err := name.ParseReference(world)
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	img, err := remote.Image(ref)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}

	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("read layers: %v", err)
	}

	if len(layers) != 2 {
		t.Fatalf("got %d layers, want 2", len(layers))
	}

	layer, err := layers[1].Uncompressed()
	if err != nil {
		t.Fatalf("uncompress layer: %v", err)
	}

	tr := tar.NewReader(layer)

	files := make(map[string]string)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar header: %v", err)
		}

		if hdr.FileInfo().IsDir() {
			continue
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read tar content: %v", err)
		}

		t.Log(hdr.Name)
		files[hdr.Name] = string(content)
	}

	if got, want := files["opt/app/world.txt"], "This is my world!"; got != want {
		t.Errorf("world.txt in image, got %q, want %q", got, want)
	}
}
