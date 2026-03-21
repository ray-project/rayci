package raycicmd

import (
	"strings"
	"testing"
)

func TestParseSelector(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKey  string
		wantMode int
		wantFilt map[string][]string
	}{
		{
			name:     "literal",
			input:    "ray-build",
			wantKey:  "ray-build",
			wantMode: selectorLiteral,
		},
		{
			name:     "implicit",
			input:    "ray-build($)",
			wantKey:  "ray-build",
			wantMode: selectorImplicit,
		},
		{
			name:     "match all",
			input:    "ray-build(*)",
			wantKey:  "ray-build",
			wantMode: selectorMatchAll,
		},
		{
			name:     "explicit single",
			input:    "ray-build(python=3.11)",
			wantKey:  "ray-build",
			wantMode: selectorFilter,
			wantFilt: map[string][]string{
				"python": {"3.11"},
			},
		},
		{
			name:     "explicit multi-dim",
			input:    "ray-build(python=3.11, cuda=12.8.1)",
			wantKey:  "ray-build",
			wantMode: selectorFilter,
			wantFilt: map[string][]string{
				"python": {"3.11"},
				"cuda":   {"12.8.1"},
			},
		},
		{
			name:     "explicit multi-value same dim",
			input:    "ray-build(python=3.10, python=3.11)",
			wantKey:  "ray-build",
			wantMode: selectorFilter,
			wantFilt: map[string][]string{
				"python": {"3.10", "3.11"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sel, err := parseSelector(tt.input)
			if err != nil {
				t.Fatalf("parseSelector(%q) error = %v", tt.input, err)
			}
			if sel.key != tt.wantKey {
				t.Errorf("key = %q, want %q", sel.key, tt.wantKey)
			}
			if sel.mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", sel.mode, tt.wantMode)
			}
			if tt.wantFilt != nil {
				if len(sel.filter) != len(tt.wantFilt) {
					t.Fatalf(
						"filter has %d keys, want %d: %v",
						len(sel.filter), len(tt.wantFilt), sel.filter,
					)
				}
				for dim, wantVals := range tt.wantFilt {
					gotVals := sel.filter[dim]
					if len(gotVals) != len(wantVals) {
						t.Errorf("filter[%q] = %v, want %v", dim, gotVals, wantVals)
						continue
					}
					for i := range wantVals {
						if gotVals[i] != wantVals[i] {
							t.Errorf("filter[%q][%d] = %q, want %q", dim, i, gotVals[i], wantVals[i])
						}
					}
				}
			} else if sel.filter != nil {
				t.Errorf("filter = %v, want nil", sel.filter)
			}
		})
	}
}

func TestParseSelectorErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "empty name",
			input:   "($)",
			wantErr: "empty",
		},
		{
			name:    "missing closing paren",
			input:   "ray-build(python=3.11",
			wantErr: "missing closing",
		},
		{
			name:    "empty parentheses",
			input:   "ray-build()",
			wantErr: "empty parentheses",
		},
		{
			name:    "missing equals",
			input:   "ray-build(python)",
			wantErr: "expected key=value",
		},
		{
			name:    "empty dimension",
			input:   "ray-build(=3.11)",
			wantErr: "empty dimension",
		},
		{
			name:    "empty value",
			input:   "ray-build(python=)",
			wantErr: "empty value",
		},
		{
			name:    "trailing comma",
			input:   "ray-build(python=3.11,)",
			wantErr: "empty filter entry",
		},
		{
			name:    "duplicate key=value",
			input:   "ray-build(python=3.11, python=3.11)",
			wantErr: "duplicate filter entry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSelector(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseArrayDependsOnMixed(t *testing.T) {
	input := []any{
		"array-step(os=linux, os=macos, variant=1)",
		"forge",
	}

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
		name     string
		sel      *arraySelector
		wantKeys []string
		wantErr  bool
	}{
		{
			name: "partial match - python only",
			sel: &arraySelector{
				key: "ray-build", mode: selectorFilter,
				filter: map[string][]string{"python": {"3.11"}},
			},
			wantKeys: []string{
				"ray-build--cuda1211-python311",
				"ray-build--cuda1281-python311",
			},
		},
		{
			name: "partial match - cuda only",
			sel: &arraySelector{
				key: "ray-build", mode: selectorFilter,
				filter: map[string][]string{"cuda": {"12.1.1"}},
			},
			wantKeys: []string{
				"ray-build--cuda1211-python310",
				"ray-build--cuda1211-python311",
			},
		},
		{
			name: "exact match",
			sel: &arraySelector{
				key: "ray-build", mode: selectorFilter,
				filter: map[string][]string{
					"python": {"3.11"}, "cuda": {"12.1.1"},
				},
			},
			wantKeys: []string{
				"ray-build--cuda1211-python311",
			},
		},
		{
			name: "match all",
			sel: &arraySelector{
				key: "ray-build", mode: selectorMatchAll,
			},
			wantKeys: []string{
				"ray-build--cuda1211-python310",
				"ray-build--cuda1211-python311",
				"ray-build--cuda1281-python310",
				"ray-build--cuda1281-python311",
			},
		},
		{
			name: "invalid dimension",
			sel: &arraySelector{
				key: "ray-build", mode: selectorFilter,
				filter: map[string][]string{"invalid": {"value"}},
			},
			wantErr: true,
		},
		{
			name: "no match",
			sel: &arraySelector{
				key: "ray-build", mode: selectorFilter,
				filter: map[string][]string{"python": {"3.12"}},
			},
			wantErr: true,
		},
		{
			name: "multi-value match",
			sel: &arraySelector{
				key: "ray-build", mode: selectorFilter,
				filter: map[string][]string{
					"python": {"3.10", "3.11"},
				},
			},
			wantKeys: []string{
				"ray-build--cuda1211-python310",
				"ray-build--cuda1211-python311",
				"ray-build--cuda1281-python310",
				"ray-build--cuda1281-python311",
			},
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
			if len(got) != len(tt.wantKeys) {
				t.Fatalf("resolveArraySelector() = %v, want %v", got, tt.wantKeys)
			}
			for i, want := range tt.wantKeys {
				if got[i] != want {
					t.Errorf("got[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
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
			name:    "map in list rejected",
			input:   []any{map[string]any{"ray-build": "all"}},
			wantErr: "expected string",
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
