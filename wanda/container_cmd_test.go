package wanda

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
)

// imageTestInfo contains image information for testing.
type imageTestInfo struct {
	LayerCount int
	Env        []string
	Labels     map[string]string
}

// containerRuntimeTest represents a container runtime for testing.
type containerRuntimeTest struct {
	name      string
	available func() bool
	runtime   ContainerRuntime
	tagPrefix string
	bin       string
}

func dockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func podmanAvailable() bool {
	_, err := exec.LookPath("podman")
	return err == nil
}

var containerRuntimes = []containerRuntimeTest{
	{
		name:      "docker",
		available: dockerAvailable,
		runtime:   RuntimeDocker,
		tagPrefix: "cr.ray.io/rayproject/",
		bin:       "docker",
	},
	{
		name:      "podman",
		available: podmanAvailable,
		runtime:   RuntimePodman,
		tagPrefix: "localhost/rayproject/",
		bin:       "podman",
	},
}

// newCmd creates a ContainerCmd for the given runtime.
func (rt *containerRuntimeTest) newCmd() ContainerCmd {
	config := &ForgeConfig{ContainerRuntime: rt.runtime}
	return config.newContainerCmd()
}

// inspectLabel gets an image label using the runtime's inspect command.
func (rt *containerRuntimeTest) inspectLabel(tag, label string) (string, error) {
	out, err := exec.Command(rt.bin, "inspect", "--format", "{{index .Config.Labels \""+label+"\"}}", tag).Output()
	return strings.TrimSpace(string(out)), err
}

// inspectImage gets detailed image information for testing.
func (rt *containerRuntimeTest) inspectImage(tag string) (*imageTestInfo, error) {
	if rt.runtime == RuntimeDocker {
		return rt.inspectImageDocker(tag)
	}
	return rt.inspectImagePodman(tag)
}

func (rt *containerRuntimeTest) inspectImageDocker(tag string) (*imageTestInfo, error) {
	ref, err := name.ParseReference(tag)
	if err != nil {
		return nil, err
	}

	img, err := daemon.Image(ref)
	if err != nil {
		return nil, err
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}

	config, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}

	return &imageTestInfo{
		LayerCount: len(layers),
		Env:        config.Config.Env,
		Labels:     config.Config.Labels,
	}, nil
}

func (rt *containerRuntimeTest) inspectImagePodman(tag string) (*imageTestInfo, error) {
	// Use podman inspect to get image details as JSON.
	out, err := exec.Command("podman", "inspect", tag).Output()
	if err != nil {
		return nil, err
	}

	var inspectResult []struct {
		RootFS struct {
			Layers []string `json:"Layers"`
		} `json:"RootFS"`
		Config struct {
			Env    []string          `json:"Env"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}

	if err := json.Unmarshal(out, &inspectResult); err != nil {
		return nil, err
	}

	if len(inspectResult) == 0 {
		return nil, nil
	}

	return &imageTestInfo{
		LayerCount: len(inspectResult[0].RootFS.Layers),
		Env:        inspectResult[0].Config.Env,
		Labels:     inspectResult[0].Config.Labels,
	}, nil
}

func TestContainerCmd_Build(t *testing.T) {
	for _, rt := range containerRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			if !rt.available() {
				t.Skipf("%s not available", rt.name)
			}

			cmd := rt.newCmd()

			ts := newTarStream()
			ts.addFile("Dockerfile.hello", nil, "testdata/Dockerfile.hello")

			tag := rt.tagPrefix + "wanda-build-test"

			buildArgs := []string{"MESSAGE=test message from " + rt.name}
			input := newBuildInput(ts, buildArgs)
			input.addTag(tag)

			core, err := input.makeCore("Dockerfile.hello")
			if err != nil {
				t.Fatalf("make build input core: %v", err)
			}

			hints := newBuildInputHints([]string{
				"REMOTE_CACHE_URL=http://localhost:5000",
				"MESSAGE=does not matter", // will be shadowed by the build args
			})

			if err := cmd.build(input, core, hints); err != nil {
				t.Fatalf("build: %v", err)
			}

			// Verify the image was built.
			info, err := cmd.inspectImage(tag)
			if err != nil {
				t.Fatalf("inspect image: %v", err)
			}
			if info == nil {
				t.Fatal("image not found after build")
			}

			// Verify the build args were applied by checking the image labels.
			label, err := rt.inspectLabel(tag, "io.ray.wanda.message")
			if err != nil {
				t.Fatalf("inspect label: %v", err)
			}

			want := "test message from " + rt.name
			if label != want {
				t.Errorf("label got %q, want %q", label, want)
			}

			// Clean up.
			_ = cmd.run("rmi", tag)
		})
	}
}

// TestContainerCmd_Build_Full tests container build with full image verification
// including layers and env variables for both docker and podman.
func TestContainerCmd_Build_Full(t *testing.T) {
	for _, rt := range containerRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			if !rt.available() {
				t.Skipf("%s not available", rt.name)
			}

			cmd := rt.newCmd()

			ts := newTarStream()
			ts.addFile("Dockerfile.hello", nil, "testdata/Dockerfile.hello")

			tag := rt.tagPrefix + "wanda-full-test"

			buildArgs := []string{"MESSAGE=test message"}
			input := newBuildInput(ts, buildArgs)
			input.addTag(tag)

			core, err := input.makeCore("Dockerfile.hello")
			if err != nil {
				t.Fatalf("make build input core: %v", err)
			}

			hints := newBuildInputHints([]string{
				"REMOTE_CACHE_URL=http://localhost:5000",
				"MESSAGE=does not matter", // will be shadowed by the build args
			})

			if err := cmd.build(input, core, hints); err != nil {
				t.Fatalf("build: %v", err)
			}

			// Get full image info.
			imgInfo, err := rt.inspectImage(tag)
			if err != nil {
				t.Fatalf("inspect image: %v", err)
			}
			if imgInfo == nil {
				t.Fatal("image not found after build")
			}

			// Verify layer count.
			if imgInfo.LayerCount != 1 {
				t.Errorf("expected 1 layer, got %d", imgInfo.LayerCount)
			}

			// Check message env value, this is set by the build args.
			messageEnv := ""
			t.Log(imgInfo.Env)
			for _, env := range imgInfo.Env {
				if strings.HasPrefix(env, "MESSAGE=") {
					messageEnv = env
					break
				}
			}

			if messageEnv != "MESSAGE=test message" {
				t.Errorf("MESSAGE env got %q, want `MESSAGE=test message`", messageEnv)
			}

			// Clean up.
			_ = cmd.run("rmi", tag)
		})
	}
}

// TestContainerCmd_Build_Full_withHints tests container build with hints (no build args)
// with full image verification for both docker and podman.
func TestContainerCmd_Build_Full_withHints(t *testing.T) {
	for _, rt := range containerRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			if !rt.available() {
				t.Skipf("%s not available", rt.name)
			}

			cmd := rt.newCmd()

			ts := newTarStream()
			ts.addFile("Dockerfile.hello", nil, "testdata/Dockerfile.hello")

			tag := rt.tagPrefix + "wanda-full-hints-test"

			input := newBuildInput(ts, nil)
			input.addTag(tag)

			core, err := input.makeCore("Dockerfile.hello")
			if err != nil {
				t.Fatalf("make build input core: %v", err)
			}

			hints := newBuildInputHints([]string{
				"REMOTE_CACHE_URL=http://localhost:5000",
				"MESSAGE=hint message",
			})

			if err := cmd.build(input, core, hints); err != nil {
				t.Fatalf("build: %v", err)
			}

			// Get full image info.
			imgInfo, err := rt.inspectImage(tag)
			if err != nil {
				t.Fatalf("inspect image: %v", err)
			}
			if imgInfo == nil {
				t.Fatal("image not found after build")
			}

			// Verify layer count.
			if imgInfo.LayerCount != 1 {
				t.Errorf("expected 1 layer, got %d", imgInfo.LayerCount)
			}

			// Check message env value, this is set by the hint args.
			messageEnv := ""
			t.Log(imgInfo.Env)
			for _, env := range imgInfo.Env {
				if strings.HasPrefix(env, "MESSAGE=") {
					messageEnv = env
					break
				}
			}

			if messageEnv != "MESSAGE=hint message" {
				t.Errorf("MESSAGE env got %q, want `MESSAGE=hint message`", messageEnv)
			}

			// Clean up.
			_ = cmd.run("rmi", tag)
		})
	}
}

func TestContainerCmd_Build_withHints(t *testing.T) {
	for _, rt := range containerRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			if !rt.available() {
				t.Skipf("%s not available", rt.name)
			}

			cmd := rt.newCmd()

			ts := newTarStream()
			ts.addFile("Dockerfile.hello", nil, "testdata/Dockerfile.hello")

			tag := rt.tagPrefix + "wanda-hints-test"

			input := newBuildInput(ts, nil) // no build args, only hints
			input.addTag(tag)

			core, err := input.makeCore("Dockerfile.hello")
			if err != nil {
				t.Fatalf("make build input core: %v", err)
			}

			hints := newBuildInputHints([]string{
				"REMOTE_CACHE_URL=http://localhost:5000",
				"MESSAGE=hint message for " + rt.name,
			})

			if err := cmd.build(input, core, hints); err != nil {
				t.Fatalf("build: %v", err)
			}

			// Verify the image was built.
			info, err := cmd.inspectImage(tag)
			if err != nil {
				t.Fatalf("inspect image: %v", err)
			}
			if info == nil {
				t.Fatal("image not found after build")
			}

			// Verify the hint args were applied by checking the image labels.
			label, err := rt.inspectLabel(tag, "io.ray.wanda.message")
			if err != nil {
				t.Fatalf("inspect label: %v", err)
			}

			want := "hint message for " + rt.name
			if label != want {
				t.Errorf("label got %q, want %q", label, want)
			}

			// Clean up.
			_ = cmd.run("rmi", tag)
		})
	}
}

func TestForge_Build(t *testing.T) {
	for _, rt := range containerRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			if !rt.available() {
				t.Skipf("%s not available", rt.name)
			}

			tmpDir := t.TempDir()

			// Create a simple file.
			if err := os.WriteFile(tmpDir+"/hello.txt", []byte("hello"), 0644); err != nil {
				t.Fatalf("write file: %v", err)
			}

			// Create a Dockerfile.
			dockerfile := `FROM scratch
COPY hello.txt /hello.txt
CMD ["cat", "/hello.txt"]
`
			if err := os.WriteFile(tmpDir+"/Dockerfile", []byte(dockerfile), 0644); err != nil {
				t.Fatalf("write Dockerfile: %v", err)
			}

			spec := &Spec{
				Name:       "forge-build-test",
				Dockerfile: "Dockerfile",
				Srcs:       []string{"hello.txt"},
			}

			config := &ForgeConfig{
				WorkDir:          tmpDir,
				NamePrefix:       "localhost/test/",
				ContainerRuntime: rt.runtime,
			}

			forge, err := NewForge(config)
			if err != nil {
				t.Fatalf("create forge: %v", err)
			}

			if err := forge.Build(spec); err != nil {
				t.Fatalf("build: %v", err)
			}

			// Verify the image was built.
			tag := "localhost/test/forge-build-test"
			cmd := rt.newCmd()
			info, err := cmd.inspectImage(tag)
			if err != nil {
				t.Fatalf("inspect image: %v", err)
			}
			if info == nil {
				t.Fatal("image not found after build")
			}

			// Clean up.
			_ = cmd.run("rmi", tag)
		})
	}
}

func TestForge_BuildWithSymlink(t *testing.T) {
	for _, rt := range containerRuntimes {
		t.Run(rt.name, func(t *testing.T) {
			if !rt.available() {
				t.Skipf("%s not available", rt.name)
			}

			tmpDir := t.TempDir()

			// Create a target file.
			if err := os.WriteFile(tmpDir+"/target.txt", []byte("target content"), 0644); err != nil {
				t.Fatalf("write target: %v", err)
			}

			// Create a symlink.
			if err := os.Symlink("target.txt", tmpDir+"/link.txt"); err != nil {
				t.Fatalf("create symlink: %v", err)
			}

			// Create a Dockerfile.
			dockerfile := `FROM scratch
COPY . /app/
CMD ["cat", "/app/link.txt"]
`
			if err := os.WriteFile(tmpDir+"/Dockerfile", []byte(dockerfile), 0644); err != nil {
				t.Fatalf("write Dockerfile: %v", err)
			}

			spec := &Spec{
				Name:       "forge-symlink-test",
				Dockerfile: "Dockerfile",
				Srcs:       []string{"target.txt", "link.txt"},
			}

			config := &ForgeConfig{
				WorkDir:          tmpDir,
				NamePrefix:       "localhost/test/",
				ContainerRuntime: rt.runtime,
			}

			forge, err := NewForge(config)
			if err != nil {
				t.Fatalf("create forge: %v", err)
			}

			if err := forge.Build(spec); err != nil {
				t.Fatalf("build: %v", err)
			}

			// Verify the image was built.
			tag := "localhost/test/forge-symlink-test"
			cmd := rt.newCmd()
			info, err := cmd.inspectImage(tag)
			if err != nil {
				t.Fatalf("inspect image: %v", err)
			}
			if info == nil {
				t.Fatal("image not found after build")
			}

			// Clean up.
			_ = cmd.run("rmi", tag)
		})
	}
}
