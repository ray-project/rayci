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
				"key":        "test-311-only",
				"commands":   []any{"echo test"},
				"depends_on": "build-step(python=3.11, cuda=12.8.1)",
			},
			{
				"key":        "test-all-python311",
				"commands":   []any{"echo test"},
				"depends_on": "build-step(python=3.11)",
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

func TestExpandArraySteps_MatchAllDependsOn(t *testing.T) {
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
				"depends_on": "build-step(*)",
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
	for _, dep := range []string{
		"plain-step(python=3.11)",
		"plain-step(*)",
		"plain-step($)",
	} {
		t.Run(dep, func(t *testing.T) {
			groups := []*pipelineGroup{{
				Group: "build",
				Steps: []map[string]any{{
					"key":        "test-step",
					"commands":   []any{"echo test"},
					"depends_on": dep,
				}},
			}}

			err := expandArraySteps(groups)
			if err == nil {
				t.Fatal("expected error for array selector on non-array step, got nil")
			}
			if !strings.Contains(err.Error(), "non-array step") {
				t.Errorf("error = %q, want to contain \"non-array step\"", err.Error())
			}
		})
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
				"depends_on": "build-step(*)",
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
				"depends_on": "build-step(*)",
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
	wantDeps := []string{"build-step--oslinux", "build-step--osPlan9"}
	if len(deps) != len(wantDeps) {
		t.Fatalf("got deps %v, want %v", deps, wantDeps)
	}
	for i, want := range wantDeps {
		if deps[i] != want {
			t.Errorf("deps[%d] = %q, want %q", i, deps[i], want)
		}
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

func TestExpandArraySteps_GroupDependsOnArrayStep(t *testing.T) {
	groups := []*pipelineGroup{
		{
			Group: "build",
			Steps: []map[string]any{{
				"label":    "Build {{array.python}}",
				"name":     "ray-core-build",
				"commands": []any{"echo {{array.python}}"},
				"array": map[string]any{
					"python": []any{"3.10", "3.11", "3.12"},
				},
			}},
		},
		{
			Group:     "tests",
			DependsOn: []string{"forge", "ray-core-build"},
			Steps: []map[string]any{{
				"key":      "test-step",
				"commands": []any{"echo test"},
			}},
		},
	}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	// Group DependsOn should have "forge" unchanged, and
	// "ray-core-build" fanned out to all expanded keys.
	got := groups[1].DependsOn
	want := []string{
		"forge",
		"ray-core-build--python310",
		"ray-core-build--python311",
		"ray-core-build--python312",
	}
	if len(got) != len(want) {
		t.Fatalf("group DependsOn = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("group DependsOn[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExpandArraySteps_GroupDependsOnNonArrayStep(t *testing.T) {
	groups := []*pipelineGroup{
		{
			Group: "build",
			Steps: []map[string]any{{
				"key":      "plain-build",
				"commands": []any{"echo build"},
			}},
		},
		{
			Group:     "tests",
			DependsOn: []string{"plain-build", "forge"},
			Steps: []map[string]any{{
				"key":      "test-step",
				"commands": []any{"echo test"},
			}},
		},
	}

	if err := expandArraySteps(groups); err != nil {
		t.Fatalf("expandArraySteps() error = %v", err)
	}

	// Group DependsOn should be unchanged for non-array refs.
	got := groups[1].DependsOn
	want := []string{"plain-build", "forge"}
	if len(got) != len(want) {
		t.Fatalf("group DependsOn = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("group DependsOn[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExpandArraySteps_DuplicateBaseKey(t *testing.T) {
	groups := []*pipelineGroup{
		{
			Group: "build",
			Steps: []map[string]any{{
				"label":    "Build {{array.python}}",
				"name":     "shared-name",
				"commands": []any{"echo build"},
				"array": map[string]any{
					"python": []any{"3.10"},
				},
			}},
		},
		{
			Group: "test",
			Steps: []map[string]any{{
				"label":    "Test {{array.python}}",
				"name":     "shared-name",
				"commands": []any{"echo test"},
				"array": map[string]any{
					"python": []any{"3.11"},
				},
			}},
		},
	}

	err := expandArraySteps(groups)
	if err == nil {
		t.Fatal("expected error for duplicate base key, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate array step key") {
		t.Errorf(
			"error = %q, want to contain \"duplicate array step key\"",
			err.Error(),
		)
	}
}

func TestExpandArraySteps_DuplicateElementFromAdjustment(t *testing.T) {
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build {{array.python}}",
			"key":      "build-step",
			"commands": []any{"echo build"},
			"array": map[string]any{
				"python": []any{"3.10", "3.11"},
				"adjustments": []any{
					map[string]any{
						"with": map[string]any{"python": "3.10"},
					},
				},
			},
		}},
	}}

	err := expandArraySteps(groups)
	if err == nil {
		t.Fatal("expected error for duplicate element, got nil")
	}
	if !strings.Contains(err.Error(), "duplicates") {
		t.Errorf(
			"error = %q, want to contain \"duplicates\"",
			err.Error(),
		)
	}
}

func TestExpandArraySteps_KeyCollisionFromSanitization(t *testing.T) {
	// Values "1.2.1" and "121" both sanitize to "121", causing
	// a key collision.
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build {{array.ver}}",
			"key":      "build-step",
			"commands": []any{"echo build"},
			"array": map[string]any{
				"ver": []any{"1.2.1", "121"},
			},
		}},
	}}

	err := expandArraySteps(groups)
	if err == nil {
		t.Fatal("expected error for key collision, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate generated key") {
		t.Errorf(
			"error = %q, want to contain \"duplicate generated key\"",
			err.Error(),
		)
	}
}

func TestExpandArraySteps_PartialDimensionSkip(t *testing.T) {
	// Skip by one dimension on a two-dimension array removes all
	// matching combinations.
	groups := []*pipelineGroup{{
		Group: "build",
		Steps: []map[string]any{{
			"label":    "Build {{array.os}} {{array.arch}}",
			"key":      "build-step",
			"commands": []any{"echo {{array.os}} {{array.arch}}"},
			"array": map[string]any{
				"os":   []any{"linux", "windows"},
				"arch": []any{"amd64", "arm64"},
				"adjustments": []any{
					map[string]any{
						"with": map[string]any{"os": "windows"},
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
	// 4 total - 2 windows combos = 2 remaining (linux/amd64, linux/arm64)
	if len(rs) != 2 {
		t.Fatalf("got %d resolvedSteps, want 2", len(rs))
	}
	for _, r := range rs {
		key := r.src["key"].(string)
		if strings.Contains(key, "oswindows") {
			t.Errorf("skipped os=windows should not appear: %q", key)
		}
	}
}

func TestExpandArraySteps_PlainStringTargetsArrayStep(t *testing.T) {
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

	err := expandArraySteps(groups)
	if err == nil {
		t.Fatal("expected error for plain depends_on targeting array step, got nil")
	}
	if !strings.Contains(err.Error(), "plain depends_on") {
		t.Errorf("error = %q, want to contain \"plain depends_on\"", err.Error())
	}
	if !strings.Contains(err.Error(), "($)") {
		t.Errorf("error = %q, want to contain \"($)\"", err.Error())
	}
}
