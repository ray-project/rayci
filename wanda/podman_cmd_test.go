package wanda

import (
	"strings"
	"testing"
)

func TestNewPodmanCmd(t *testing.T) {
	testCases := []struct {
		name    string
		config  *PodmanCmdConfig
		wantBin string
	}{
		{
			name:    "default",
			config:  &PodmanCmdConfig{},
			wantBin: "podman",
		},
		{
			name:    "custom bin",
			config:  &PodmanCmdConfig{Bin: "/usr/local/bin/podman"},
			wantBin: "/usr/local/bin/podman",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewPodmanCmd(tc.config)
			cmd := c.(*podmanCmd)

			if cmd.bin != tc.wantBin {
				t.Errorf("bin: got %q, want %q", cmd.bin, tc.wantBin)
			}

			// Check that at least one env var is propagated, e.g. PATH.
			foundEnv := false
			for _, env := range cmd.envs {
				if strings.HasPrefix(env, "PATH=") {
					foundEnv = true
					break
				}
			}
			if !foundEnv {
				t.Errorf("expected env vars to be propagated, but PATH was not found")
			}
		})
	}
}
