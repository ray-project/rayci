package raycicmd

import (
	"testing"
)

func TestMakeRayDockerPlugin_mountSSHAgent(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		config := &stepDockerPluginConfig{}
		m := makeRayDockerPlugin("test-image:latest", config)
		if _, ok := m["mount-ssh-agent"]; ok {
			t.Errorf(
				"makeRayDockerPlugin() mount-ssh-agent = %#v, want absent",
				m["mount-ssh-agent"],
			)
		}
	})

	t.Run("enabled when configured", func(t *testing.T) {
		config := &stepDockerPluginConfig{mountSSHAgent: true}
		m := makeRayDockerPlugin("test-image:latest", config)
		got, ok := m["mount-ssh-agent"]
		if !ok {
			t.Fatal("makeRayDockerPlugin() missing mount-ssh-agent key")
		}
		if val, ok := got.(bool); !ok || !val {
			t.Errorf(
				"makeRayDockerPlugin() mount-ssh-agent = %#v, want true",
				got,
			)
		}
	})
}

func TestBkPipelineTotalSteps(t *testing.T) {
	p := &bkPipeline{
		Steps: []*bkPipelineGroup{
			{
				Group: "group1",
				Steps: []any{
					map[string]any{"command": "echo step1"},
					map[string]any{"command": "echo step2"},
				},
			},
			{
				Group: "group2",
				Steps: []any{
					map[string]any{"command": "echo step3"},
				},
			},
		},
	}
	if total := p.totalSteps(); total != 3 {
		t.Errorf("totalSteps() = %d; want 3", total)
	}
}
