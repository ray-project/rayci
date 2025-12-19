package wanda

import (
	"testing"
)

func TestNewPodmanCmd(t *testing.T) {
	c := NewPodmanCmd(&PodmanCmdConfig{})
	cmd := c.(*podmanCmd)

	if cmd.bin != "podman" {
		t.Errorf("bin: got %q, want %q", cmd.bin, "podman")
	}
}

func TestNewPodmanCmd_customBin(t *testing.T) {
	c := NewPodmanCmd(&PodmanCmdConfig{Bin: "/usr/local/bin/podman"})
	cmd := c.(*podmanCmd)

	if cmd.bin != "/usr/local/bin/podman" {
		t.Errorf("bin: got %q, want %q", cmd.bin, "/usr/local/bin/podman")
	}
}
