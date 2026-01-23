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

func TestWandaStep_skip(t *testing.T) {
	s := &wandaStep{
		name:    "forge",
		file:    "ci/forge.wanda.yaml",
		buildID: "abc123",

		envs:         map[string]string{"RAYCI_BRANCH": "stable"},
		instanceType: "builder-arm64",

		ciConfig: &config{
			BuilderQueues: map[string]string{"builder-arm64": "~"},
		},
	}

	bk := s.buildkiteStep()

	key, ok := stringInMap(bk, "key")
	if !ok || key != "forge" {
		t.Errorf("got key %q, want `forge`", key)
	}

	if got := bk["skip"].(bool); !got {
		t.Errorf("got skip %v, want true", got)
	}
	if _, ok := bk["agent"]; ok {
		t.Errorf("got agent %v, want nil", bk["agent"])
	}
}

func TestWandaStep_priority(t *testing.T) {
	priority := 5
	s := &wandaStep{
		name:     "forge",
		file:     "ci/forge.wanda.yaml",
		buildID:  "abc123",
		priority: &priority,

		envs: map[string]string{"RAYCI_BRANCH": "stable"},

		ciConfig: &config{
			BuilderQueues:   map[string]string{"builder": "mybuilder"},
			BuilderPriority: 1, // This should be overridden by step-level priority
		},
	}

	bk := s.buildkiteStep()

	if got := bk["priority"].(int); got != 5 {
		t.Errorf("got priority %d, want 5", got)
	}
}

func TestWandaStep_envfile(t *testing.T) {
	tests := []struct {
		name          string
		envFile       string
		wantSet       bool
		expectedValue string
	}{
		{
			name:          "with envfile",
			envFile:       "ci/build.env",
			wantSet:       true,
			expectedValue: "ci/build.env",
		},
		{
			name:    "without envfile",
			envFile: "",
			wantSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &wandaStep{
				name:    "forge",
				file:    "ci/forge.wanda.yaml",
				envFile: tt.envFile,
				buildID: "abc123",
				envs:    map[string]string{"RAYCI_BRANCH": "stable"},
				ciConfig: &config{
					BuilderQueues: map[string]string{"builder": "mybuilder"},
				},
			}

			bk := s.buildkiteStep()
			envs := bk["env"].(map[string]string)

			got, ok := envs["RAYCI_ENV_FILE"]
			if tt.wantSet {
				if !ok {
					t.Fatal("expected RAYCI_ENV_FILE to be set, but it was not")
				}
				if got != tt.expectedValue {
					t.Errorf("RAYCI_ENV_FILE = %q, want %q", got, tt.expectedValue)
				}
			} else {
				if ok {
					t.Errorf("expected RAYCI_ENV_FILE to not be set, but it was set to %q", got)
				}
			}
		})
	}
}
