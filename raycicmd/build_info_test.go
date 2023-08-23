package raycicmd

import (
	"testing"
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

func TestGitDiff(t *testing.T) {
	env := newEnvsMap(map[string]string{})
	got := gitDiff(env)
	if len(got) != 0 {
		t.Errorf("gitDiff: got %v, want empty", got)
	}
}
