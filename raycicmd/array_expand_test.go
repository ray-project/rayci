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

func TestExpandArraySteps_SelectorDependsOn(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{
			{
				"label":    "Build {{array.python}} {{array.cuda}}",
				"key":      "build-step",
				"commands": []any{"echo {{array.python}} {{array.cuda}}"},
				"array": map[string]any{
					"python": []any{"3.10", "3.11"},
					"cuda":   []any{"12.1.1", "12.8.1"},
				},
			},
			{
				"key":      "test-311-only",
				"commands": []any{"echo test"},
				"depends_on": []any{
					map[string]any{
						"build-step": map[string]any{
							"python": "3.11",
							"cuda":   "12.8.1",
						},
					},
				},
			},
			{
				"key":      "test-all-python311",
				"commands": []any{"echo test"},
				"depends_on": []any{
					map[string]any{
						"build-step": map[string]any{
							"python": "3.11",
						},
					},
				},
			},
		},
	}}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	// 4 expanded steps + 2 downstream steps = 6 total
	rs := groups[0].resolvedSteps
	if len(rs) != 6 {
		t.Fatalf("got %d resolvedSteps, want 6", len(rs))
	}

	// Exact match: python=3.11, cuda=12.8.1 -> 1 dep
	exactDeps := rs[4].resolvedDependsOn
	if len(exactDeps) != 1 {
		t.Fatalf("exact selector: got %d deps, want 1: %v", len(exactDeps), exactDeps)
	}
	if exactDeps[0] != "build-step--cuda1281-python311" {
		t.Errorf("exact selector: got %q, want %q", exactDeps[0], "build-step--cuda1281-python311")
	}

	// Partial match: python=3.11 (any cuda) -> 2 deps
	partialDeps := rs[5].resolvedDependsOn
	if len(partialDeps) != 2 {
		t.Fatalf("partial selector: got %d deps, want 2: %v", len(partialDeps), partialDeps)
	}
	if partialDeps[0] != "build-step--cuda1211-python311" || partialDeps[1] != "build-step--cuda1281-python311" {
		t.Errorf("partial selector: got %v, want [build-step--cuda1211-python311, build-step--cuda1281-python311]", partialDeps)
	}
}

func TestExpandArraySteps_BaseKeyDependsOn(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{
			{
				"label":    "Build {{array.python}}",
				"key":      "build-step",
				"commands": []any{"echo build"},
				"array": map[string]any{
					"python": []any{"3.10", "3.11"},
				},
			},
			{
				"key":        "test-step",
				"commands":   []any{"echo test"},
				"depends_on": "build-step",
			},
		},
	}}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	rs := groups[0].resolvedSteps
	// 2 expanded + test-step = index 2
	resolved := rs[2].resolvedDependsOn
	if len(resolved) != 2 {
		t.Fatalf("got %d deps, want 2: %v", len(resolved), resolved)
	}
	if resolved[0] != "build-step--python310" || resolved[1] != "build-step--python311" {
		t.Errorf("resolved deps: got %v, want [build-step--python310, build-step--python311]", resolved)
	}
}

func TestExpandArraySteps_SelectorOnNonArrayStep(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"key":      "test-step",
			"commands": []any{"echo test"},
			"depends_on": []any{
				map[string]any{
					"plain-step": map[string]any{"python": "3.11"},
				},
			},
		}},
	}}

	err := expandArraySteps(groups)
	if err == nil {
		t.Fatal("expected error for array selector on non-array step, got nil")
	}
	if !strings.Contains(err.Error(), "non-array step") {
		t.Errorf("error = %q, want to contain \"non-array step\"", err.Error())
	}
}

func TestExpandArraySteps_NonArrayDependsOn(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"key":        "test-step",
			"commands":   []any{"echo test"},
			"depends_on": "plain-step",
		}},
	}}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	rs := groups[0].resolvedSteps[0]
	resolved := rs.resolvedDependsOn
	if len(resolved) != 1 || resolved[0] != "plain-step" {
		t.Errorf("resolvedDependsOn = %v, want [\"plain-step\"]", resolved)
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

func TestResolveArraySelector(t *testing.T) {
	cfg := &arrayConfig{
		dims: map[string][]string{
			"python": {"3.10", "3.11"},
			"cuda":   {"12.1.1", "12.8.1"},
		},
	}
	cfg.elements = cfg.expand()

	tests := []struct {
		name    string
		sel     *arraySelector
		wantLen int
		wantErr bool
	}{
		{
			name: "partial match - python only",
			sel: &arraySelector{
				key:    "ray-build",
				filter: map[string][]string{"python": {"3.11"}},
			},
			wantLen: 2,
		},
		{
			name: "partial match - cuda only",
			sel: &arraySelector{
				key:    "ray-build",
				filter: map[string][]string{"cuda": {"12.1.1"}},
			},
			wantLen: 2,
		},
		{
			name: "exact match",
			sel: &arraySelector{
				key:    "ray-build",
				filter: map[string][]string{"python": {"3.11"}, "cuda": {"12.1.1"}},
			},
			wantLen: 1,
		},
		{
			name: "invalid dimension",
			sel: &arraySelector{
				key:    "ray-build",
				filter: map[string][]string{"invalid": {"value"}},
			},
			wantErr: true,
		},
		{
			name: "no match",
			sel: &arraySelector{
				key:    "ray-build",
				filter: map[string][]string{"python": {"3.12"}},
			},
			wantErr: true,
		},
		{
			name: "multi-value match",
			sel: &arraySelector{
				key:    "ray-build",
				filter: map[string][]string{"python": {"3.10", "3.11"}},
			},
			wantLen: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveArraySelector(tt.sel, cfg)
			if tt.wantErr {
				if err == nil {
					t.Error("resolveArraySelector() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveArraySelector() error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("resolveArraySelector() returned %d matches, want %d: %v", len(got), tt.wantLen, got)
			}
		})
	}
}

func TestResolveArraySelectorKeyFormat(t *testing.T) {
	cfg := &arrayConfig{
		dims: map[string][]string{
			"python": {"3.11"},
			"cuda":   {"12.1.1"},
		},
	}
	cfg.elements = cfg.expand()

	sel := &arraySelector{
		key:    "ray-build",
		filter: map[string][]string{"python": {"3.11"}, "cuda": {"12.1.1"}},
	}

	got, err := resolveArraySelector(sel, cfg)
	if err != nil {
		t.Fatalf("resolveArraySelector() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("resolveArraySelector() returned %d matches, want 1", len(got))
	}

	if !strings.HasPrefix(got[0], "ray-build--") {
		t.Errorf("key = %q, want prefix \"ray-build--\"", got[0])
	}
}

func TestExpandArraySteps_SkipAdjustment(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build {{array.os}} {{array.arch}}",
			"key":      "build-step",
			"commands": []any{"echo {{array.os}} {{array.arch}}"},
			"array": map[string]any{
				"os":   []any{"windows", "linux"},
				"arch": []any{"amd64", "arm64"},
				"adjustments": []any{
					map[string]any{
						"with": map[string]any{"os": "windows", "arch": "arm64"},
						"skip": true,
					},
				},
			},
		}},
	}}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	rs := groups[0].resolvedSteps
	if len(rs) != 3 {
		t.Fatalf("got %d resolvedSteps, want 3", len(rs))
	}

	for _, r := range rs {
		key := r.src["key"].(string)
		if key == "build-step--archarm64-oswindows" {
			t.Errorf("skipped combination should not appear: %q", key)
		}
	}
}

func TestExpandArraySteps_AddAdjustment(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build {{array.os}} {{array.arch}}",
			"key":      "build-step",
			"commands": []any{"echo {{array.os}} {{array.arch}}"},
			"array": map[string]any{
				"os":   []any{"windows", "linux"},
				"arch": []any{"amd64"},
				"adjustments": []any{
					map[string]any{
						"with": map[string]any{"os": "Plan 9", "arch": "arm64"},
					},
				},
			},
		}},
	}}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	rs := groups[0].resolvedSteps
	// 2 original + 1 added = 3
	if len(rs) != 3 {
		t.Fatalf("got %d resolvedSteps, want 3", len(rs))
	}

	lastKey := rs[2].src["key"].(string)
	if lastKey != "build-step--archarm64-osPlan9" {
		t.Errorf("added step key = %q, want %q", lastKey, "build-step--archarm64-osPlan9")
	}
}

func TestExpandArraySteps_AddToSingleElementProduct(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build py{{array.python}} cu{{array.cuda}}",
			"key":      "cuda-build",
			"commands": []any{"echo {{array.python}} {{array.cuda}}"},
			"array": map[string]any{
				"python": []any{"3.12"},
				"cuda":   []any{"13.0.0-cudnn"},
				"adjustments": []any{
					map[string]any{
						"with": map[string]any{
							"python": "3.11",
							"cuda":   "12.8.1-cudnn",
						},
					},
				},
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

	got0 := rs[0].src["key"].(string)
	want0 := "cuda-build--cuda1300cudnn-python312"
	if got0 != want0 {
		t.Errorf("resolvedSteps[0] key = %q, want %q", got0, want0)
	}

	got1 := rs[1].src["key"].(string)
	want1 := "cuda-build--cuda1281cudnn-python311"
	if got1 != want1 {
		t.Errorf("resolvedSteps[1] key = %q, want %q", got1, want1)
	}
}

func TestExpandArraySteps_DependsOnExcludesSkipped(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{
			{
				"label":    "Build {{array.os}}",
				"key":      "build-step",
				"commands": []any{"echo {{array.os}}"},
				"array": map[string]any{
					"os": []any{"linux", "windows", "macos"},
					"adjustments": []any{
						map[string]any{
							"with": map[string]any{"os": "windows"},
							"skip": true,
						},
					},
				},
			},
			{
				"key":        "test-step",
				"commands":   []any{"echo test"},
				"depends_on": "build-step",
			},
		},
	}}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	// 2 expanded (linux, macos) + test-step = 3 total
	rs := groups[0].resolvedSteps
	if len(rs) != 3 {
		t.Fatalf("got %d resolvedSteps, want 3", len(rs))
	}

	deps := rs[2].resolvedDependsOn
	if len(deps) != 2 {
		t.Fatalf("got %d deps, want 2: %v", len(deps), deps)
	}
	for _, dep := range deps {
		if dep == "build-step--oswindows" {
			t.Error("depends_on should not include skipped element")
		}
	}
}

func TestExpandArraySteps_DependsOnIncludesAdded(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{
			{
				"label":    "Build {{array.os}}",
				"key":      "build-step",
				"commands": []any{"echo {{array.os}}"},
				"array": map[string]any{
					"os": []any{"linux"},
					"adjustments": []any{
						map[string]any{
							"with": map[string]any{"os": "Plan 9"},
						},
					},
				},
			},
			{
				"key":        "test-step",
				"commands":   []any{"echo test"},
				"depends_on": "build-step",
			},
		},
	}}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	rs := groups[0].resolvedSteps
	// 2 expanded (linux + Plan 9) + test-step = 3
	if len(rs) != 3 {
		t.Fatalf("got %d resolvedSteps, want 3", len(rs))
	}

	deps := rs[2].resolvedDependsOn
	if len(deps) != 2 {
		t.Fatalf("got %d deps, want 2: %v", len(deps), deps)
	}
}

func TestExpandArraySteps_SkipNoMatch(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build {{array.os}}",
			"key":      "build-step",
			"commands": []any{"echo {{array.os}}"},
			"array": map[string]any{
				"os": []any{"linux", "windows"},
				"adjustments": []any{
					map[string]any{
						"with": map[string]any{"os": "macos"},
						"skip": true,
					},
				},
			},
		}},
	}}

	err := expandArraySteps(groups)
	if err == nil {
		t.Fatal("expected error for skip with no match, got nil")
	}
	if !strings.Contains(err.Error(), "matches no element") {
		t.Errorf("error = %q, want to contain \"matches no element\"", err.Error())
	}
}

func TestExpandArraySteps_AddMissingDimension(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build {{array.os}} {{array.arch}}",
			"key":      "build-step",
			"commands": []any{"echo {{array.os}} {{array.arch}}"},
			"array": map[string]any{
				"os":   []any{"linux"},
				"arch": []any{"amd64"},
				"adjustments": []any{
					map[string]any{
						"with": map[string]any{"os": "Plan 9"},
					},
				},
			},
		}},
	}}

	err := expandArraySteps(groups)
	if err == nil {
		t.Fatal("expected error for addition missing dimension, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %q, want to contain \"missing\"", err.Error())
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
