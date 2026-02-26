package raycicmd

import (
	"strings"
	"testing"
)

func TestExpandArraySteps(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build {{array.python}}",
			"key":      "build-step",
			"commands": []any{"echo {{array.python}}"},
			"array": map[string]any{
				"python": []any{"3.10", "3.11"},
			},
		}},
	}}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	rs := groups[0].resolvedSteps
	if len(rs) != 2 {
		t.Fatalf("got %d resolvedSteps, want 2", len(rs))
	}

	if got := rs[0].src["key"]; got != "build-step--python310" {
		t.Errorf("resolvedSteps[0].src[key] = %q, want %q", got, "build-step--python310")
	}
	if got := rs[1].src["key"]; got != "build-step--python311" {
		t.Errorf("resolvedSteps[1].src[key] = %q, want %q", got, "build-step--python311")
	}
}

func TestExpandArraySteps_DoesNotMutateSrc(t *testing.T) {
	origStep := map[string]any{
		"label":    "Build {{array.python}}",
		"key":      "build-step",
		"commands": []any{"echo {{array.python}}"},
		"array": map[string]any{
			"python": []any{"3.10", "3.11"},
		},
	}
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{origStep},
	}}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	// g.Steps must be unchanged.
	if len(groups[0].Steps) != 1 {
		t.Fatalf("Steps length = %d, want 1", len(groups[0].Steps))
	}
	if got := groups[0].Steps[0]["key"]; got != "build-step" {
		t.Errorf("Steps[0][key] = %q, want %q", got, "build-step")
	}
	if _, ok := groups[0].Steps[0]["array"]; !ok {
		t.Error("Steps[0] lost 'array' key")
	}
}

func TestExpandArraySteps_LabelPlaceholderRequired(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build step",
			"key":      "build-step",
			"commands": []any{"echo build"},
			"array": map[string]any{
				"python": []any{"3.10", "3.11"},
			},
		}},
	}}

	err := expandArraySteps(groups)
	if err == nil {
		t.Fatal("expected error for missing placeholder, got nil")
	}
	if !strings.Contains(err.Error(), "placeholder") {
		t.Errorf("error = %q, want to contain \"placeholder\"", err.Error())
	}
}

func TestExpandArraySteps_MatrixAndArrayMutuallyExclusive(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build {{array.python}}",
			"key":      "build-step",
			"commands": []any{"echo build"},
			"matrix": map[string]any{
				"setup": map[string]any{
					"python": []any{"3.10", "3.11"},
				},
			},
			"array": map[string]any{
				"python": []any{"3.10", "3.11"},
			},
		}},
	}}

	err := expandArraySteps(groups)
	if err == nil {
		t.Fatal("expected error for both matrix and array, got nil")
	}
	if !strings.Contains(err.Error(), "both") {
		t.Errorf("error = %q, want to contain \"both\"", err.Error())
	}
}
