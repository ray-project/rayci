package wanda

import (
	"testing"

	"archive/tar"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"runtime"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	cranev1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func filesInLayer(layer cranev1.Layer) (map[string]string, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("uncompress layer: %w", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)

	files := make(map[string]string)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar header: %w", err)
		}

		if hdr.FileInfo().IsDir() {
			continue
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read tar content: %w", err)
		}

		files[hdr.Name] = string(content)
	}

	return files, nil
}

const worldDotTxt = "This is my world!"

func TestForgeLocal_noNamePrefix(t *testing.T) {
	config := &ForgeConfig{WorkDir: "testdata"}

	if err := Build("testdata/localbase.wanda.yaml", config); err != nil {
		t.Fatalf("build base: %v", err)
	}

	if err := Build("testdata/local.wanda.yaml", config); err != nil {
		t.Fatalf("build: %v", err)
	}

	const resultRef = "cr.ray.io/rayproject/local"
	ref, err := name.ParseReference(resultRef)
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
		t.Fatalf("got %d layers, want 2", len(layers))
	}

	files, err := filesInLayer(layers[0])
	if err != nil {
		t.Fatalf("read layer: %v", err)
	}

	if _, ok := files["opt/app/Dockerfile"]; !ok {
		t.Errorf("Dockerfile not in image")
	}
}

func TestForge_globFiles(t *testing.T) {
	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
	}

	if err := Build("testdata/glob.wanda.yaml", config); err != nil {
		t.Fatalf("build: %v", err)
	}

	const resultRef = "cr.ray.io/rayproject/glob"
	ref, err := name.ParseReference(resultRef)
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
		t.Fatalf("got %d layers, want 2", len(layers))
	}

	files, err := filesInLayer(layers[0])
	if err != nil {
		t.Fatalf("read layer: %v", err)
	}

	for _, file := range []string{
		"src/foo.cpp",
		"src/foo.h",
		"world.txt",
	} {
		if _, ok := files[file]; !ok {
			t.Errorf("%q not in image", file)
		}
	}
}

func TestForge_withHints(t *testing.T) {
	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
		Rebuild:    true,
	}

	if err := Build("testdata/hello-hint.wanda.yaml", config); err != nil {
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
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}

	imgConfig, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	// Check message env value, this is set by the build args.
	t.Log(imgConfig.Config.Cmd)
	t.Log(imgConfig.Config.Labels)

	labelGot := imgConfig.Config.Labels["io.ray.wanda.message"]
	const labelWant = "hint message"
	if labelGot != labelWant {
		t.Errorf("label got %v, want %v", labelGot, labelWant)
	}
}

func TestForge(t *testing.T) {
	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
	}

	if err := Build("testdata/hello.wanda.yaml", config); err != nil {
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

	if err := Build("testdata/world.wanda.yaml", config); err != nil {
		t.Fatalf("build world: %v", err)
	}

	world := "cr.ray.io/rayproject/world"
	ref2, err := name.ParseReference(world)
	if err != nil {
		t.Fatalf("parse world reference: %v", err)
	}

	img2, err := daemon.Image(ref2)
	if err != nil {
		t.Fatalf("read world image: %v", err)
	}

	layers2, err := img2.Layers()
	if err != nil {
		t.Fatalf("read world layers: %v", err)
	}

	if len(layers2) != 2 {
		t.Fatalf("got %d world layers, want 1", len(layers2))
	}

	files, err := filesInLayer(layers2[1])
	if err != nil {
		t.Fatalf("read world layer files: %v", err)
	}

	if got := files["opt/app/world.txt"]; got != worldDotTxt {
		t.Errorf("world.txt in image, got %q, want %q", got, worldDotTxt)
	}
}

func TestForge_noCache(t *testing.T) {
	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
	}

	if err := Build("testdata/hello-nocache.wanda.yaml", config); err != nil {
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

	config.BuildID = "abc123"
	forge, err := NewForge(config)
	if err != nil {
		t.Fatalf("make new forge: %v", err)
	}

	helloSpec, err := parseSpecFile("testdata/hello-nocache.wanda.yaml")
	if err != nil {
		t.Fatalf("parse hello spec: %v", err)
	}

	if err := forge.Build(helloSpec); err != nil {
		t.Fatalf("rebuild hello: %v", err)
	}

	if hit := forge.cacheHit(); hit != 0 {
		t.Errorf("got %d cache hits, want 0", hit)
	}
}

func TestForgeWithRemoteWorkRepo(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping test on non-linux")
		return
	}

	cr := registry.New()

	var server *httptest.Server
	var crAddr string
	if os.Getenv("WANDA_TEST_CR_PORT") == "" {
		server = httptest.NewServer(cr)
		crAddr = server.Listener.Addr().String()
	} else {
		server = httptest.NewUnstartedServer(cr)
		listenAddr := fmt.Sprintf(":%s", os.Getenv("WANDA_TEST_CR_PORT"))
		listener, err := net.Listen("tcp4", listenAddr)
		if err != nil {
			t.Fatal("listen error:", err)
		}
		server.Listener = listener

		port := listener.Addr().(*net.TCPAddr).Port
		crAddr = fmt.Sprintf("localhost:%d", port)

		server.Start()
	}
	defer server.Close()

	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
		WorkRepo:   fmt.Sprintf("%s/work", crAddr),
		BuildID:    "abc123",
		RayCI:      true,
		Epoch:      "1",
	}

	if err := Build("testdata/hello.wanda.yaml", config); err != nil {
		t.Fatalf("build hello: %v", err)
	}

	if err := Build("testdata/world.wanda.yaml", config); err != nil {
		t.Fatalf("build world: %v", err)
	}

	world := fmt.Sprintf("%s/work:abc123-world", crAddr)
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
	files, err := filesInLayer(layers[1])
	if err != nil {
		t.Fatalf("read layer: %v", err)
	}
	if got := files["opt/app/world.txt"]; got != worldDotTxt {
		t.Errorf("world.txt in image, got %q, want %q", got, worldDotTxt)
	}

	// Now test caching, on anthoer forge, with a different build ID.

	config.BuildID = "def456"
	forge, err := NewForge(config)
	if err != nil {
		t.Fatalf("make new forge: %v", err)
	}

	helloSpec, err := parseSpecFile("testdata/hello.wanda.yaml")
	if err != nil {
		t.Fatalf("parse hello spec: %v", err)
	}
	// Apply a hint, and it should still be cache hit.
	helloSpec.BuildHintArgs = []string{"REMOTE_CACHE_URL=http://localhost:5000"}

	if err := forge.Build(helloSpec); err != nil {
		t.Fatalf("rebuild hello: %v", err)
	}

	if hit := forge.cacheHit(); hit != 1 {
		t.Errorf("got %d cache hits, want 1", hit)
	}

	hello := fmt.Sprintf("%s/work:def456-hello", crAddr)
	helloRef, err := name.ParseReference(hello)
	if err != nil {
		t.Fatalf("parse hello reference: %v", err)
	}

	helloImg, err := remote.Image(helloRef)
	if err != nil {
		t.Fatalf("read hello image: %v", err)
	}

	helloLayers, err := helloImg.Layers()
	if err != nil {
		t.Fatalf("read hello layers: %v", err)
	}

	if len(helloLayers) != 1 {
		t.Fatalf("got hello %d layers, want 2", len(layers))
	}

	config.Epoch = "2"
	forge2, err := NewForge(config)
	if err != nil {
		t.Fatalf("make forge for new epoch: %v", err)
	}

	if err := forge2.Build(helloSpec); err != nil {
		t.Fatalf("rebuild hello: %v", err)
	}

	if hit := forge2.cacheHit(); hit != 0 {
		t.Errorf("got %d cache hits, want 0", hit)
	}
}

func TestForgeLocal_withNamePrefix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping test on non-linux")
		return
	}

	randomEpoch := func() string {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			t.Fatalf("read random: %v", err)
		}
		return fmt.Sprintf("%x", b)
	}

	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
		Epoch:      randomEpoch(),
	}

	if err := Build("testdata/hello.wanda.yaml", config); err != nil {
		t.Fatalf("build hello: %v", err)
	}

	if err := Build("testdata/world.wanda.yaml", config); err != nil {
		t.Fatalf("build world: %v", err)
	}

	world := "localhost:5000/rayci-work:world"
	ref, err := name.ParseReference(world)
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

	if len(layers) != 2 {
		t.Fatalf("got %d layers, want 2", len(layers))
	}
	files, err := filesInLayer(layers[1])
	if err != nil {
		t.Fatalf("read layer: %v", err)
	}
	if got := files["opt/app/world.txt"]; got != worldDotTxt {
		t.Errorf("world.txt in image, got %q, want %q", got, worldDotTxt)
	}

	// Now test caching, on anthoer forge, with a different build ID.

	config.BuildID = "abc123"
	forge, err := NewForge(config)
	if err != nil {
		t.Fatalf("make new forge: %v", err)
	}

	helloSpec, err := parseSpecFile("testdata/hello.wanda.yaml")
	if err != nil {
		t.Fatalf("parse hello spec: %v", err)
	}

	if err := forge.Build(helloSpec); err != nil {
		t.Fatalf("rebuild hello: %v", err)
	}

	if hit := forge.cacheHit(); hit != 1 {
		t.Errorf("got %d cache hits, want 1", hit)
	}

	hello := "localhost:5000/rayci-work:abc123-hello"
	helloRef, err := name.ParseReference(hello)
	if err != nil {
		t.Fatalf("parse hello reference: %v", err)
	}

	helloImg, err := daemon.Image(helloRef)
	if err != nil {
		t.Fatalf("read hello image: %v", err)
	}

	helloLayers, err := helloImg.Layers()
	if err != nil {
		t.Fatalf("read hello layers: %v", err)
	}

	if len(helloLayers) != 1 {
		t.Fatalf("got hello %d layers, want 2", len(layers))
	}

	config.Epoch = randomEpoch()
	forge2, err := NewForge(config)
	if err != nil {
		t.Fatalf("make forge for new epoch: %v", err)
	}

	if err := forge2.Build(helloSpec); err != nil {
		t.Fatalf("rebuild hello: %v", err)
	}

	if hit := forge2.cacheHit(); hit != 0 {
		t.Errorf("got %d cache hits, want 0", hit)
	}
}
