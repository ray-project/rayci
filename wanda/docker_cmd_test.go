package wanda

import (
	"testing"
)

func TestNewDockerCmd(t *testing.T) {
	c := NewDockerCmd(&DockerCmdConfig{})
	cmd := c.(*dockerCmd)

	if cmd.bin != "docker" {
		t.Errorf("bin: got %q, want %q", cmd.bin, "docker")
	}

	if cmd.useLegacyEngine {
		t.Error("useLegacyEngine should be false by default")
	}
}

func TestNewDockerCmd_customBin(t *testing.T) {
	c := NewDockerCmd(&DockerCmdConfig{Bin: "/usr/local/bin/docker"})
	cmd := c.(*dockerCmd)

	if cmd.bin != "/usr/local/bin/docker" {
		t.Errorf("bin: got %q, want %q", cmd.bin, "/usr/local/bin/docker")
	}
}

func TestNewDockerCmd_legacyEngine(t *testing.T) {
	c := NewDockerCmd(&DockerCmdConfig{UseLegacyEngine: true})
	cmd := c.(*dockerCmd)

	if !cmd.useLegacyEngine {
		t.Error("useLegacyEngine should be true when configured")
	}
}
