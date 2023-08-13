package raycicmd

import (
	"testing"

	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
)

func TestForgeName(t *testing.T) {
	for _, test := range []struct {
		file, want string
	}{
		{file: "Dockerfile.forge", want: "forge"},
		{file: "Dockerfile.wheel-forge", want: "wheel-forge"},
	} {
		got, ok := forgeNameFromDockerfile(test.file)
		if !ok {
			t.Errorf("forgeNameFromDockerfile(%q): got !ok, want ok", test.file)
			continue
		}
		if got != test.want {
			t.Errorf(
				"forgeNameFromDockerfile(%q): got %q, want %q",
				test.file, got, test.want,
			)
		}
	}

	for _, file := range []string{
		"Dockerfile",
		"Dockerfile.",
		"Dockerfil",
		"other",
		".",
		"",
	} {
		if _, ok := forgeNameFromDockerfile(file); ok {
			t.Errorf("forgeNameFromDockerfile(%q): got ok, want !ok", file)
		}
	}
}

func jsonString(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestMakeForgeGroup(t *testing.T) {
	config := &config{
		name: "test",

		ArtifactsBucket: "rayci-artifacts",

		CITemp:     "s3://ci-temp",
		CITempRepo: rayCIECR + "/rayci_test",

		BuilderQueues: map[string]string{
			"builder": "builder_queue",
		},
		BuilderPriority: 1,

		ForgeDirs: []string{
			"ci/forge",
			"civ2/forge",
		},
	}

	buildID := "fakebuild"

	t.Run("no forge", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "ci/forge"), 0700); err != nil {
			t.Fatalf("make forge dir: %v", err)
		}
		g, err := makeForgeGroup(root, buildID, config, nil)
		if err != nil {
			t.Fatalf("make forge group: %v", err)
		}

		if len(g.Steps) != 0 {
			t.Errorf("makeForgeGroup(): got %d steps, want 0", len(g.Steps))
		}
	})

	t.Run("one forge dir ", func(t *testing.T) {
		root := t.TempDir()

		if err := os.MkdirAll(filepath.Join(root, "ci/forge"), 0700); err != nil {
			t.Fatalf("make forge dir: %v", err)
		}

		p := filepath.Join(root, "ci/forge/Dockerfile.forge")
		content := []byte(`FROM ubuntu:latest`)
		if err := os.WriteFile(p, content, 0600); err != nil {
			t.Fatalf("write forge file: %v", err)
		}

		p = filepath.Join(root, "ci/forge/Dockerfile.wheel-forge")
		content = []byte(`FROM manylinux2014`)
		if err := os.WriteFile(p, content, 0600); err != nil {
			t.Fatalf("write wheel-forge file: %v", err)
		}

		envMap := map[string]string{
			"RAYCI_BUILD_ID": buildID,
			"RAYCI_TMP_REPO": config.CITempRepo,
		}

		g, err := makeForgeGroup(root, buildID, config, envMap)
		if err != nil {
			t.Fatalf("make forge group: %v", err)
		}

		if g.Group != "forge" {
			t.Errorf("got group %q, want %q", g.Group, "forge")
		}
		if g.Key != "all-forges" {
			t.Errorf("got key %q, want %q", g.Key, "all-forges")
		}
		if len(g.Steps) != 2 {
			t.Fatalf("got %d steps, want 1", len(g.Steps))
		}

		step := g.Steps[0].(map[string]any)
		want := map[string]any{
			"label":    "forge",
			"key":      "forge",
			"commands": []string{forgeBuilderCommand},
			"env": map[string]string{
				"RAYCI_BUILD_ID":         buildID,
				"RAYCI_TMP_REPO":         config.CITempRepo,
				"RAYCI_FORGE_DOCKERFILE": "ci/forge/Dockerfile.forge",
				"RAYCI_FORGE_NAME":       "forge",
			},
			"agents":   map[string]any{"queue": "builder_queue"},
			"priority": 1,
		}
		if !reflect.DeepEqual(step, want) {
			t.Errorf(
				"got step 1 %s\n, want %s",
				jsonString(step),
				jsonString(want),
			)
		}

		step = g.Steps[1].(map[string]any)
		want = map[string]any{
			"label":    "wheel-forge",
			"key":      "wheel-forge",
			"commands": []string{forgeBuilderCommand},
			"env": map[string]string{
				"RAYCI_BUILD_ID":         buildID,
				"RAYCI_TMP_REPO":         config.CITempRepo,
				"RAYCI_FORGE_DOCKERFILE": "ci/forge/Dockerfile.wheel-forge",
				"RAYCI_FORGE_NAME":       "wheel-forge",
			},
			"agents":   map[string]any{"queue": "builder_queue"},
			"priority": 1,
		}
		if !reflect.DeepEqual(step, want) {
			t.Errorf(
				"got step 2 %s\n, want %s",
				jsonString(step),
				jsonString(want),
			)
		}

	})

	// For having forges from two different directories.
	t.Run("two forge dirs", func(t *testing.T) {
		root := t.TempDir()

		if err := os.MkdirAll(filepath.Join(root, "ci/forge"), 0700); err != nil {
			t.Fatalf("make forge dir: %v", err)
		}

		p := filepath.Join(root, "ci/forge/Dockerfile.forge")
		content := []byte(`FROM ubuntu:latest`)
		if err := os.WriteFile(p, content, 0600); err != nil {
			t.Fatalf("write forge file: %v", err)
		}

		if err := os.MkdirAll(filepath.Join(root, "civ2/forge"), 0700); err != nil {
			t.Fatalf("make v2 forge dir: %v", err)
		}

		p = filepath.Join(root, "civ2/forge/Dockerfile.forgev2")
		if err := os.WriteFile(p, content, 0600); err != nil {
			t.Fatalf("write v2 forge file: %v", err)
		}

		envMap := map[string]string{"RAYCI_BUILD_ID": buildID}

		g, err := makeForgeGroup(root, buildID, config, envMap)
		if err != nil {
			t.Fatalf("make forge group: %v", err)
		}

		if g.Group != "forge" {
			t.Errorf("got group %q, want %q", g.Group, "forge")
		}
		if g.Key != "all-forges" {
			t.Errorf("got key %q, want %q", g.Key, "all-forges")
		}
		if len(g.Steps) != 2 {
			t.Fatalf("got %d steps, want 1", len(g.Steps))
		}

		step := g.Steps[0].(map[string]any)
		want := map[string]any{
			"label":    "forge",
			"key":      "forge",
			"commands": []string{forgeBuilderCommand},
			"env": map[string]string{
				"RAYCI_BUILD_ID":         buildID,
				"RAYCI_FORGE_DOCKERFILE": "ci/forge/Dockerfile.forge",
				"RAYCI_FORGE_NAME":       "forge",
			},
			"agents":   map[string]any{"queue": "builder_queue"},
			"priority": 1,
		}
		if !reflect.DeepEqual(step, want) {
			t.Errorf(
				"got step 1 %s\n, want %s",
				jsonString(step),
				jsonString(want),
			)
		}

		step = g.Steps[1].(map[string]any)
		want = map[string]any{
			"label":    "forgev2",
			"key":      "forgev2",
			"commands": []string{forgeBuilderCommand},
			"env": map[string]string{
				"RAYCI_BUILD_ID":         buildID,
				"RAYCI_FORGE_DOCKERFILE": "civ2/forge/Dockerfile.forgev2",
				"RAYCI_FORGE_NAME":       "forgev2",
			},
			"agents":   map[string]any{"queue": "builder_queue"},
			"priority": 1,
		}
		if !reflect.DeepEqual(step, want) {
			t.Errorf(
				"got step 2 %s\n, want %s",
				jsonString(step),
				jsonString(want),
			)
		}
	})
}
