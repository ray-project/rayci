package raycicmd

import (
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func mustParseYAML(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := yaml.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	return v
}

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

func TestParseArrayDependsOnString(t *testing.T) {
	selectors, err := parseArrayDependsOn("ray-build")
	if err != nil {
		t.Fatalf("parseArrayDependsOn() error = %v", err)
	}

	if len(selectors) != 1 {
		t.Fatalf("len(selectors) = %d, want 1", len(selectors))
	}
	if selectors[0].key != "ray-build" {
		t.Errorf("selectors[0].key = %q, want \"ray-build\"", selectors[0].key)
	}
	if selectors[0].filter != nil {
		t.Errorf("selectors[0].filter = %v, want nil", selectors[0].filter)
	}
}

func TestParseArrayDependsOnArray(t *testing.T) {
	input := []any{"step-a", "step-b"}

	selectors, err := parseArrayDependsOn(input)
	if err != nil {
		t.Fatalf("parseArrayDependsOn() error = %v", err)
	}

	if len(selectors) != 2 {
		t.Fatalf("len(selectors) = %d, want 2", len(selectors))
	}
	if selectors[0].key != "step-a" {
		t.Errorf("selectors[0].key = %q, want \"step-a\"", selectors[0].key)
	}
	if selectors[1].key != "step-b" {
		t.Errorf("selectors[1].key = %q, want \"step-b\"", selectors[1].key)
	}
}

func TestParseArrayDependsOnSelector(t *testing.T) {
	input := mustParseYAML(t, strings.Join([]string{
		"- ray-build:",
		`    python: "3.11"`,
	}, "\n"))

	selectors, err := parseArrayDependsOn(input)
	if err != nil {
		t.Fatalf("parseArrayDependsOn() error = %v", err)
	}

	if len(selectors) != 1 {
		t.Fatalf("len(selectors) = %d, want 1", len(selectors))
	}
	if selectors[0].key != "ray-build" {
		t.Errorf("selectors[0].key = %q, want \"ray-build\"", selectors[0].key)
	}
	got := selectors[0].filter["python"]
	if len(got) != 1 || got[0] != "3.11" {
		t.Errorf("selectors[0].filter[\"python\"] = %v, want [\"3.11\"]", got)
	}
}

func TestParseArrayDependsOnSelectorArray(t *testing.T) {
	input := mustParseYAML(t, strings.Join([]string{
		"- ray-build:",
		"    python:",
		`      - "3.10"`,
		`      - "3.11"`,
	}, "\n"))

	selectors, err := parseArrayDependsOn(input)
	if err != nil {
		t.Fatalf("parseArrayDependsOn() error = %v", err)
	}

	if len(selectors) != 1 {
		t.Fatalf("len(selectors) = %d, want 1", len(selectors))
	}
	got := selectors[0].filter["python"]
	if len(got) != 2 || got[0] != "3.10" || got[1] != "3.11" {
		t.Errorf("selectors[0].filter[\"python\"] = %v, want [\"3.10\", \"3.11\"]", got)
	}
}

func TestParseArrayDependsOnMixed(t *testing.T) {
	input := mustParseYAML(t, strings.Join([]string{
		`- array-step:`,
		`    os: [linux, macos]`,
		`    variant: "1"`,
		`- forge`,
	}, "\n"))

	selectors, err := parseArrayDependsOn(input)
	if err != nil {
		t.Fatalf("parseArrayDependsOn() error = %v", err)
	}

	if len(selectors) != 2 {
		t.Fatalf("len(selectors) = %d, want 2", len(selectors))
	}

	if selectors[0].key != "array-step" {
		t.Errorf("selectors[0].key = %q, want \"array-step\"", selectors[0].key)
	}
	gotOS := selectors[0].filter["os"]
	if len(gotOS) != 2 || gotOS[0] != "linux" || gotOS[1] != "macos" {
		t.Errorf("selectors[0].filter[\"os\"] = %v, want [\"linux\", \"macos\"]", gotOS)
	}
	gotVariant := selectors[0].filter["variant"]
	if len(gotVariant) != 1 || gotVariant[0] != "1" {
		t.Errorf("selectors[0].filter[\"variant\"] = %v, want [\"1\"]", gotVariant)
	}

	if selectors[1].key != "forge" {
		t.Errorf("selectors[1].key = %q, want \"forge\"", selectors[1].key)
	}
	if selectors[1].filter != nil {
		t.Errorf("selectors[1].filter = %v, want nil", selectors[1].filter)
	}
}

func TestParseArrayDependsOnErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{
			name:    "invalid type",
			input:   42,
			wantErr: "must be string or array",
		},
		{
			name:    "selector with multiple keys",
			input:   []any{map[string]any{"step-a": map[string]any{}, "step-b": map[string]any{}}},
			wantErr: "exactly one key",
		},
		{
			name:    "selector filter not a map",
			input:   []any{map[string]any{"ray-build": "invalid"}},
			wantErr: "filter must be a map",
		},
		{
			name:    "invalid value in array filter",
			input:   []any{map[string]any{"ray-build": map[string]any{"python": 123}}},
			wantErr: "in selector for \"ray-build\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseArrayDependsOn(tt.input)
			if err == nil {
				t.Fatal("parseArrayDependsOn() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
