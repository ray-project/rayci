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
	"path/filepath"
	"runtime"
	"strings"

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

		if hdr.Typeflag == tar.TypeSymlink {
			files[hdr.Name] = hdr.Linkname
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
	symlinkPath := "testdata/src/link-to-foo.h"
	symlinkTarget := "foo.h"
	if err := os.Symlink(symlinkTarget, symlinkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	defer os.Remove(symlinkPath)

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
		t.Fatalf("got %d layers, want 1", len(layers))
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

	if got, ok := files["src/link-to-foo.h"]; !ok {
		t.Error("symlink src/link-to-foo.h not found in image")
	} else if got != symlinkTarget {
		t.Errorf("symlink target: got %q, want %q", got, symlinkTarget)
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

	const tag = "cr.ray.io/rayproject/hello-hint"

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

	if err := Build("testdata/hello-test.wanda.yaml", config); err != nil {
		t.Fatalf("build: %v", err)
	}

	const tag = "cr.ray.io/rayproject/hello-test"

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

	const tag = "cr.ray.io/rayproject/hello-nocache"

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

func TestForge_noCacheAfterExpandVar(t *testing.T) {
	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
		BuildID:    "nocache-expand-test",
	}

	forge, err := NewForge(config)
	if err != nil {
		t.Fatalf("make forge: %v", err)
	}

	spec, err := parseSpecFile("testdata/hello-nocache.wanda.yaml")
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}

	// Simulate what Build() does: expand variables before building.
	// This is a regression test to ensure DisableCaching is preserved
	// through expandVar.
	spec = spec.expandVar(os.LookupEnv)

	if !spec.DisableCaching {
		t.Fatal("DisableCaching should be true after expandVar")
	}

	if err := forge.Build(spec); err != nil {
		t.Fatalf("first build: %v", err)
	}

	// Build again - should have 0 cache hits since caching is disabled.
	if err := forge.Build(spec); err != nil {
		t.Fatalf("second build: %v", err)
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

	if err := Build("testdata/hello-test.wanda.yaml", config); err != nil {
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

	helloSpec, err := parseSpecFile("testdata/hello-test.wanda.yaml")
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

	hello := fmt.Sprintf("%s/work:def456-hello-test", crAddr)
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

func TestBuild_WithDeps(t *testing.T) {
	// Test: dep-top -> dep-middle -> dep-base
	// Build should build in order: dep-base, dep-middle, dep-top

	// Create a wandaspecs file pointing to testdata directory.
	wandaSpecs := filepath.Join(t.TempDir(), ".wandaspecs")
	absTestdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("abs testdata: %v", err)
	}
	if err := os.WriteFile(wandaSpecs, []byte(absTestdata), 0644); err != nil {
		t.Fatalf("write wandaspecs: %v", err)
	}

	config := &ForgeConfig{
		WorkDir:        "testdata",
		NamePrefix:     "cr.ray.io/rayproject/",
		WandaSpecsFile: wandaSpecs,
	}

	if err := Build("testdata/dep-top.wanda.yaml", config); err != nil {
		t.Fatalf("build with deps: %v", err)
	}

	// Verify dep-top was built and can be read
	ref, err := name.ParseReference("cr.ray.io/rayproject/dep-top")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	img, err := daemon.Image(ref)
	if err != nil {
		t.Fatalf("read dep-top image: %v", err)
	}

	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("read layers: %v", err)
	}

	// Should have 3 layers: dep-base, dep-middle, dep-top
	if got, want := len(layers), 3; got != want {
		t.Errorf("got %d layers, want %d", got, want)
	}
}

func TestBuild_NoDeps(t *testing.T) {
	// Test backward compatibility: a spec with no deps should work
	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
	}

	if err := Build("testdata/hello-test.wanda.yaml", config); err != nil {
		t.Fatalf("build with deps: %v", err)
	}

	ref, err := name.ParseReference("cr.ray.io/rayproject/hello-test")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	img, err := daemon.Image(ref)
	if err != nil {
		t.Fatalf("read hello image: %v", err)
	}

	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("read layers: %v", err)
	}

	if got, want := len(layers), 1; got != want {
		t.Errorf("got %d layers, want %d", got, want)
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

	if err := Build("testdata/hello-test.wanda.yaml", config); err != nil {
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

	helloSpec, err := parseSpecFile("testdata/hello-test.wanda.yaml")
	if err != nil {
		t.Fatalf("parse hello spec: %v", err)
	}

	if err := forge.Build(helloSpec); err != nil {
		t.Fatalf("rebuild hello: %v", err)
	}

	if hit := forge.cacheHit(); hit != 1 {
		t.Errorf("got %d cache hits, want 1", hit)
	}

	hello := "localhost:5000/rayci-work:abc123-hello-test"
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

func TestBuild_WithEnvfile(t *testing.T) {
	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
		EnvFile:    "testdata/test.env",
	}

	if err := Build("testdata/env-file-test.wanda.yaml", config); err != nil {
		t.Fatalf("build with envfile: %v", err)
	}

	ref, err := name.ParseReference("cr.ray.io/rayproject/env-file-test")
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	img, err := daemon.Image(ref)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}

	imgConfig, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	// Verify envfile values were expanded into build args
	msgLabel := imgConfig.Config.Labels["io.ray.wanda.message"]
	if msgLabel != "from-envfile" {
		t.Errorf("message label = %q, want %q", msgLabel, "from-envfile")
	}

	versionLabel := imgConfig.Config.Labels["io.ray.wanda.version"]
	if versionLabel != "1.0.0" {
		t.Errorf("version label = %q, want %q", versionLabel, "1.0.0")
	}
}

func TestBuild_EnvfileMissing(t *testing.T) {
	config := &ForgeConfig{
		WorkDir:    "testdata",
		NamePrefix: "cr.ray.io/rayproject/",
		EnvFile:    "testdata/nonexistent.env",
	}

	err := Build("testdata/env-file-missing.wanda.yaml", config)
	if err == nil {
		t.Fatal("expected error for missing envfile, got nil")
	}

	if !strings.Contains(err.Error(), "nonexistent.env") {
		t.Errorf("error should mention envfile name, got: %v", err)
	}
}

func TestBuild_EnvfileCacheInvalidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Use random values to avoid cache hits from previous test runs
	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		t.Fatalf("read random: %v", err)
	}
	randSuffix := fmt.Sprintf("%x", randBytes)

	// Create envfile with random suffix to ensure unique cache key
	envfilePath := filepath.Join(tmpDir, "cache-test.env")
	if err := os.WriteFile(envfilePath, []byte(fmt.Sprintf("VERSION=1.0.0-%s\n", randSuffix)), 0644); err != nil {
		t.Fatalf("write envfile: %v", err)
	}

	// Create Dockerfile
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile.cache")
	dockerfile := "FROM scratch\nARG VERSION=0.0.0\nLABEL version=${VERSION}\n"
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		t.Fatalf("write dockerfile: %v", err)
	}

	// Create spec (no envfile field - it's now passed via config)
	specPath := filepath.Join(tmpDir, "cache-test.wanda.yaml")
	spec := "name: cache-test\ndockerfile: Dockerfile.cache\nbuild_args:\n  - VERSION=$VERSION\n"
	if err := os.WriteFile(specPath, []byte(spec), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	config := &ForgeConfig{
		WorkDir:    tmpDir,
		NamePrefix: "cr.ray.io/rayproject/",
		EnvFile:    envfilePath,
	}

	// First build
	if err := Build(specPath, config); err != nil {
		t.Fatalf("first build: %v", err)
	}

	// Second build with same envfile - should cache hit
	forge1, err := NewForge(config)
	if err != nil {
		t.Fatalf("new forge: %v", err)
	}

	envMap1, err := ParseEnvFile(envfilePath)
	if err != nil {
		t.Fatalf("parse envfile: %v", err)
	}
	lookup1 := func(key string) (string, bool) {
		if v, ok := envMap1[key]; ok {
			return v, true
		}
		return os.LookupEnv(key)
	}

	parsedSpec1, err := parseSpecFile(specPath)
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	expandedSpec1 := parsedSpec1.expandVar(lookup1)

	if err := forge1.Build(expandedSpec1); err != nil {
		t.Fatalf("second build: %v", err)
	}
	if forge1.cacheHit() != 1 {
		t.Errorf("expected cache hit on unchanged envfile, got %d hits", forge1.cacheHit())
	}

	// Update envfile with a different version
	if err := os.WriteFile(envfilePath, []byte(fmt.Sprintf("VERSION=2.0.0-%s\n", randSuffix)), 0644); err != nil {
		t.Fatalf("update envfile: %v", err)
	}

	// Third build with changed envfile - should cache miss
	forge2, err := NewForge(config)
	if err != nil {
		t.Fatalf("new forge: %v", err)
	}

	envMap2, err := ParseEnvFile(envfilePath)
	if err != nil {
		t.Fatalf("parse envfile: %v", err)
	}
	lookup2 := func(key string) (string, bool) {
		if v, ok := envMap2[key]; ok {
			return v, true
		}
		return os.LookupEnv(key)
	}

	parsedSpec2, err := parseSpecFile(specPath)
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	expandedSpec2 := parsedSpec2.expandVar(lookup2)

	if err := forge2.Build(expandedSpec2); err != nil {
		t.Fatalf("third build: %v", err)
	}
	if forge2.cacheHit() != 0 {
		t.Errorf("expected cache miss on changed envfile, got %d hits", forge2.cacheHit())
	}
}

func TestForgeConfigArtifactsDir(t *testing.T) {
	config := &ForgeConfig{ArtifactsDir: "/custom/artifacts", WorkDir: "/work"}
	if got := config.ArtifactsDir; got != "/custom/artifacts" {
		t.Errorf("ArtifactsDir = %q, want %q", got, "/custom/artifacts")
	}
}

func TestBuild_WithArtifacts_exact(t *testing.T) {
	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      "testdata",
		NamePrefix:   "cr.ray.io/rayproject/",
		ArtifactsDir: artifactsDir,
	}

	if err := Build("testdata/artifact-exact.wanda.yaml", config); err != nil {
		t.Fatalf("build: %v", err)
	}

	extractedFile := filepath.Join(artifactsDir, "bin/myapp")
	content, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}

	want := "binary-content\n"
	if got := string(content); got != want {
		t.Errorf("extracted content = %q, want %q", got, want)
	}
}

func TestBuild_WithArtifacts_optional(t *testing.T) {
	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      "testdata",
		NamePrefix:   "cr.ray.io/rayproject/",
		ArtifactsDir: artifactsDir,
	}

	// Build should succeed even though the optional artifact doesn't exist
	if err := Build("testdata/artifact-optional.wanda.yaml", config); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Required artifact should be extracted
	extractedFile := filepath.Join(artifactsDir, "bin/myapp")
	content, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}

	want := "binary-content\n"
	if got := string(content); got != want {
		t.Errorf("extracted content = %q, want %q", got, want)
	}

	// Optional artifact should not exist (since source doesn't exist)
	optionalFile := filepath.Join(artifactsDir, "optional.txt")
	if _, err := os.Stat(optionalFile); !os.IsNotExist(err) {
		t.Errorf("optional file should not exist, but got err: %v", err)
	}
}

func TestBuild_WithArtifacts_rootOnly(t *testing.T) {
	wandaSpecs := filepath.Join(t.TempDir(), ".wandaspecs")
	absTestdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("abs testdata: %v", err)
	}
	if err := os.WriteFile(wandaSpecs, []byte(absTestdata), 0644); err != nil {
		t.Fatalf("write wandaspecs: %v", err)
	}

	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:        "testdata",
		NamePrefix:     "cr.ray.io/rayproject/",
		WandaSpecsFile: wandaSpecs,
		ArtifactsDir:   artifactsDir,
	}

	if err := Build("testdata/artifact-dep-top.wanda.yaml", config); err != nil {
		t.Fatalf("build with deps: %v", err)
	}

	topFile := filepath.Join(artifactsDir, "top.txt")
	if _, err := os.Stat(topFile); os.IsNotExist(err) {
		t.Error("root spec artifact should have been extracted")
	}

	depDocsFile := filepath.Join(artifactsDir, "docs/readme.md")
	if _, err := os.Stat(depDocsFile); !os.IsNotExist(err) {
		t.Error("dependency artifact should NOT have been extracted")
	}
}

func TestBuild_WithArtifacts_cacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	artifactsDir1 := filepath.Join(tmpDir, "artifacts1")
	artifactsDir2 := filepath.Join(tmpDir, "artifacts2")

	config := &ForgeConfig{
		WorkDir:      "testdata",
		NamePrefix:   "cr.ray.io/rayproject/",
		ArtifactsDir: artifactsDir1,
	}

	if err := Build("testdata/artifact-exact.wanda.yaml", config); err != nil {
		t.Fatalf("first build: %v", err)
	}

	extractedFile1 := filepath.Join(artifactsDir1, "bin/myapp")
	if _, err := os.Stat(extractedFile1); os.IsNotExist(err) {
		t.Fatal("first build should have extracted artifact")
	}

	config.ArtifactsDir = artifactsDir2

	if err := Build("testdata/artifact-exact.wanda.yaml", config); err != nil {
		t.Fatalf("second build: %v", err)
	}

	extractedFile2 := filepath.Join(artifactsDir2, "bin/myapp")
	if _, err := os.Stat(extractedFile2); os.IsNotExist(err) {
		t.Error("artifact should be extracted even on cache hit")
	}
}

func TestBuild_WithArtifacts_glob(t *testing.T) {
	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      "testdata",
		NamePrefix:   "cr.ray.io/rayproject/",
		ArtifactsDir: artifactsDir,
	}

	if err := Build("testdata/artifact-glob.wanda.yaml", config); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Check that both wheel files were extracted
	wheels := []string{"mypackage-1.0.0.whl", "mypackage-1.0.1.whl"}
	for _, wheel := range wheels {
		extractedFile := filepath.Join(artifactsDir, "wheels", wheel)
		content, err := os.ReadFile(extractedFile)
		if err != nil {
			t.Errorf("read extracted file %s: %v", wheel, err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("extracted file %s is empty", wheel)
		}
	}
}

func TestBuild_WithArtifacts_globNoTrailingSlash(t *testing.T) {
	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      "testdata",
		NamePrefix:   "cr.ray.io/rayproject/",
		ArtifactsDir: artifactsDir,
	}

	// This spec has dst: "wheels" (no trailing slash) but glob matches multiple files.
	// The code should automatically treat dst as a directory.
	if err := Build("testdata/artifact-glob-notrail.wanda.yaml", config); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Check that both wheel files were extracted into the wheels directory
	wheels := []string{"mypackage-1.0.0.whl", "mypackage-1.0.1.whl"}
	for _, wheel := range wheels {
		extractedFile := filepath.Join(artifactsDir, "wheels", wheel)
		content, err := os.ReadFile(extractedFile)
		if err != nil {
			t.Errorf("read extracted file %s: %v", wheel, err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("extracted file %s is empty", wheel)
		}
	}
}

func TestBuild_WithArtifacts_noCmdImage(t *testing.T) {
	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")

	config := &ForgeConfig{
		WorkDir:      "testdata",
		NamePrefix:   "cr.ray.io/rayproject/",
		ArtifactsDir: artifactsDir,
	}

	if err := Build("testdata/artifact-nocmd.wanda.yaml", config); err != nil {
		t.Fatalf("build: %v", err)
	}

	extractedFile := filepath.Join(artifactsDir, "output.txt")
	content, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}

	want := "test-content\n"
	if got := string(content); got != want {
		t.Errorf("extracted content = %q, want %q", got, want)
	}
}
