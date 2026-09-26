package raycicmd

import (
	"testing"

	"github.com/ray-project/rayci/wanda"
)

func TestBuildID(t *testing.T) {
	t.Run("custom build ID", func(t *testing.T) {
		env := newEnvsMap(map[string]string{"RAYCI_BUILD_ID": "myid"})
		got, err := makeBuildID(env)
		if err != nil {
			t.Fatalf("makeBuildID: %v", err)
		}
		if want := "myid"; got != want {
			t.Errorf("makeBuildID: got %q, want %q", got, want)
		}
	})

	t.Run("buildkite build ID", func(t *testing.T) {
		env1 := newEnvsMap(map[string]string{"BUILDKITE_BUILD_ID": "id1"})
		got1, err := makeBuildID(env1)
		if err != nil {
			t.Fatalf("makeBuildID 1: %v", err)
		}

		env2 := newEnvsMap(map[string]string{"BUILDKITE_BUILD_ID": "id2"})
		got2, err := makeBuildID(env2)
		if err != nil {
			t.Fatalf("makeBuildID 2: %v", err)
		}
		if got1 == got2 {
			t.Errorf("got same build ID %q, want different build IDs", got1)
		}
	})
}

func TestGitCommit(t *testing.T) {
	env := newEnvsMap(map[string]string{"BUILDKITE_COMMIT": "123abcdefg"})
	got := gitCommit(env)
	if want := "123abcdefg"; got != want {
		t.Errorf("gitCommit: got %q, want %q", got, want)
	}
}

func TestMakeCacheEpoch(t *testing.T) {
	t.Run("override", func(t *testing.T) {
		env := newEnvsMap(map[string]string{"RAYCI_CACHE_EPOCH": "202533b"})
		if got := makeCacheEpoch(env); got != "202533b" {
			t.Errorf("makeCacheEpoch() = %q, want `202533b`", got)
		}
	})

	t.Run("default matches wanda", func(t *testing.T) {
		env := newEnvsMap(map[string]string{})
		got := makeCacheEpoch(env)
		if want := wanda.DefaultCacheEpoch(); got != want {
			t.Errorf("makeCacheEpoch() = %q, want %q", got, want)
		}
	})

	t.Run("empty override falls back", func(t *testing.T) {
		env := newEnvsMap(map[string]string{"RAYCI_CACHE_EPOCH": ""})
		if got := makeCacheEpoch(env); got == "" {
			t.Error("makeCacheEpoch() = \"\", want a resolved epoch")
		}
	})
}
