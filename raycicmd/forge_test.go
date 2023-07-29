package raycicmd

import (
	"testing"
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
