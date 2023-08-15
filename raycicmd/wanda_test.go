package raycicmd

import "testing"

func TestWandaStep(t *testing.T) {
	s := &wandaStep{
		name:    "forge",
		file:    "ci/forge.wanda.yaml",
		buildID: "abc123",

		dependsOn: "forge-deps",

		envs: map[string]string{"RAYCI_BRANCH": "stable"},

		ciConfig: &config{
			BuilderQueues:   map[string]string{"builder": "mybuilder"},
			BuilderPriority: 1,
		},
	}

	bk := s.buildkiteStep()

	key, ok := stringInMap(bk, "key")
	if !ok || key != "forge" {
		t.Errorf("got key %q, want `forge`", key)
	}

	dependsOn, ok := stringInMap(bk, "depends_on")
	if !ok || dependsOn != "forge-deps" {
		t.Errorf("got depends_on %q, want `forge-deps`", dependsOn)
	}

	if got := bk["priority"].(int); got != 1 {
		t.Errorf("got priority %d, want 1", got)
	}
	if got := bk["agents"].(map[string]any)["queue"].(string); got != "mybuilder" {
		t.Errorf("got agents queue %q, want `mybuilder`", got)
	}
}
