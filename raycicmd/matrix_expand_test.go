package raycicmd

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestParseMatrixConfigSimple(t *testing.T) {
	input := []any{"3.10", "3.11", "3.12"}

	cfg, err := parseMatrixConfig(input)
	if err != nil {
		t.Fatalf("parseMatrixConfig() error = %v", err)
	}

	want := []variant{"3.10", "3.11", "3.12"}
	if got := cfg.Setup[""]; !reflect.DeepEqual(got, want) {
		t.Errorf("parseMatrixConfig() Setup[\"\"] = %v, want %v", got, want)
	}
}

func TestParseMatrixConfigSetup(t *testing.T) {
	input := map[string]any{
		"setup": map[string]any{
			"python": []any{"3.10", "3.11"},
			"cuda":   []any{"12.1.1", "12.8.1"},
		},
	}

	cfg, err := parseMatrixConfig(input)
	if err != nil {
		t.Fatalf("parseMatrixConfig() error = %v", err)
	}

	if got := cfg.Setup["python"]; !reflect.DeepEqual(got, []variant{"3.10", "3.11"}) {
		t.Errorf("Setup[\"python\"] = %v, want [\"3.10\", \"3.11\"]", got)
	}
	if got := cfg.Setup["cuda"]; !reflect.DeepEqual(got, []variant{"12.1.1", "12.8.1"}) {
		t.Errorf("Setup[\"cuda\"] = %v, want [\"12.1.1\", \"12.8.1\"]", got)
	}
}

func TestParseMatrixConfigAdjustmentsNotSupported(t *testing.T) {
	input := map[string]any{
		"setup": map[string]any{
			"python": []any{"3.10", "3.11"},
			"cuda":   []any{"12.1.1", "12.8.1"},
		},
		"adjustments": []any{
			map[string]any{
				"with": map[string]any{
					"python": "3.10",
					"cuda":   "12.8.1",
				},
				"skip": true,
			},
		},
	}

	_, err := parseMatrixConfig(input)
	if err == nil {
		t.Fatal("parseMatrixConfig() expected error for adjustments, got nil")
	}
	if !strings.Contains(err.Error(), "adjustments is not supported") {
		t.Errorf("error = %q, want to contain \"adjustments is not supported\"", err.Error())
	}
}

func TestExpandMatrixSimple(t *testing.T) {
	cfg := &matrixConfig{
		Setup: map[dimension][]variant{
			"": {"3.10", "3.11", "3.12"},
		},
	}

	instances := cfg.expand()

	if len(instances) != 3 {
		t.Fatalf("len(instances) = %d, want 3", len(instances))
	}

	values := make([]string, len(instances))
	for i, inst := range instances {
		values[i] = string(inst.Values[""])
	}
	sort.Strings(values)

	want := []string{"3.10", "3.11", "3.12"}
	if !reflect.DeepEqual(values, want) {
		t.Errorf("values = %v, want %v", values, want)
	}
}

func TestExpandMatrixMultiDimensional(t *testing.T) {
	cfg := &matrixConfig{
		Setup: map[dimension][]variant{
			"python": {"3.10", "3.11"},
			"cuda":   {"12.1.1", "12.8.1"},
		},
	}

	instances := cfg.expand()

	if len(instances) != 4 {
		t.Fatalf("len(instances) = %d, want 4", len(instances))
	}

	// Collect all combinations
	var combos []string
	for _, inst := range instances {
		combos = append(combos, string(inst.Values["python"])+"-"+string(inst.Values["cuda"]))
	}
	sort.Strings(combos)

	want := []string{
		"3.10-12.1.1",
		"3.10-12.8.1",
		"3.11-12.1.1",
		"3.11-12.8.1",
	}
	if !reflect.DeepEqual(combos, want) {
		t.Errorf("combos = %v, want %v", combos, want)
	}
}

func TestGenerateMatrixInstanceKey(t *testing.T) {
	tests := []struct {
		name    string
		baseKey string
		cfg     *matrixConfig
		inst    *matrixInstance
		want    string
	}{
		{
			name:    "simple",
			baseKey: "ray-build",
			cfg:     &matrixConfig{Setup: map[dimension][]variant{"": {"3.10"}}},
			inst:    &matrixInstance{Values: map[dimension]variant{"": "3.10"}},
			want:    "ray-build-310", // periods removed for valid Buildkite key
		},
		{
			name:    "multi-dimensional",
			baseKey: "ray-build",
			cfg: &matrixConfig{Setup: map[dimension][]variant{
				"python": {"3.11"},
				"cuda":   {"12.1.1"},
			}},
			inst: &matrixInstance{Values: map[dimension]variant{"python": "3.11", "cuda": "12.1.1"}},
			want: "ray-build-cuda1211-python311", // alphabetical order, periods removed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inst.generateKey(tt.baseKey, tt.cfg)
			if got != tt.want {
				t.Errorf("generateKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateMatrixTags(t *testing.T) {
	tests := []struct {
		name string
		inst *matrixInstance
		want []string
	}{
		{
			name: "simple",
			inst: &matrixInstance{Values: map[dimension]variant{"": "3.10"}},
			want: []string{"3.10"},
		},
		{
			name: "named dimensions",
			inst: &matrixInstance{Values: map[dimension]variant{"python": "3.11", "cuda": "12.1.1"}},
			want: []string{"cuda-12.1.1", "python-3.11"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inst.generateTags()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubstituteValues(t *testing.T) {
	tests := []struct {
		name  string
		input any
		inst  *matrixInstance
		want  any
	}{
		{
			name:  "simple string",
			input: "python {{matrix}}",
			inst:  &matrixInstance{Values: map[dimension]variant{"": "3.11"}},
			want:  "python 3.11",
		},
		{
			name:  "named dimension",
			input: "python {{matrix.python}} cuda {{matrix.cuda}}",
			inst:  &matrixInstance{Values: map[dimension]variant{"python": "3.11", "cuda": "12.1.1"}},
			want:  "python 3.11 cuda 12.1.1",
		},
		{
			name:  "nested map",
			input: map[string]any{"cmd": "./build.sh --python={{matrix.python}}"},
			inst:  &matrixInstance{Values: map[dimension]variant{"python": "3.11"}},
			want:  map[string]any{"cmd": "./build.sh --python=3.11"},
		},
		{
			name:  "array",
			input: []any{"echo {{matrix.python}}", "echo {{matrix.cuda}}"},
			inst:  &matrixInstance{Values: map[dimension]variant{"python": "3.11", "cuda": "12.1.1"}},
			want:  []any{"echo 3.11", "echo 12.1.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inst.substituteValues(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("substituteValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMatrixDependsOnString(t *testing.T) {
	selectors, err := parseMatrixDependsOn("ray-build")
	if err != nil {
		t.Fatalf("parseMatrixDependsOn() error = %v", err)
	}

	if len(selectors) != 1 {
		t.Fatalf("len(selectors) = %d, want 1", len(selectors))
	}
	if selectors[0].Key != "ray-build" {
		t.Errorf("selectors[0].Key = %q, want \"ray-build\"", selectors[0].Key)
	}
	if selectors[0].Matrix != nil {
		t.Errorf("selectors[0].Matrix = %v, want nil", selectors[0].Matrix)
	}
}

func TestParseMatrixDependsOnArray(t *testing.T) {
	input := []any{"step-a", "step-b"}

	selectors, err := parseMatrixDependsOn(input)
	if err != nil {
		t.Fatalf("parseMatrixDependsOn() error = %v", err)
	}

	if len(selectors) != 2 {
		t.Fatalf("len(selectors) = %d, want 2", len(selectors))
	}
	if selectors[0].Key != "step-a" {
		t.Errorf("selectors[0].Key = %q, want \"step-a\"", selectors[0].Key)
	}
	if selectors[1].Key != "step-b" {
		t.Errorf("selectors[1].Key = %q, want \"step-b\"", selectors[1].Key)
	}
}

func TestParseMatrixDependsOnSelector(t *testing.T) {
	// Test single string syntax
	input := []any{
		map[string]any{
			"key": "ray-build",
			"matrix": map[string]any{
				"python": "3.11", // single string, not array
			},
		},
	}

	selectors, err := parseMatrixDependsOn(input)
	if err != nil {
		t.Fatalf("parseMatrixDependsOn() error = %v", err)
	}

	if len(selectors) != 1 {
		t.Fatalf("len(selectors) = %d, want 1", len(selectors))
	}
	if selectors[0].Key != "ray-build" {
		t.Errorf("selectors[0].Key = %q, want \"ray-build\"", selectors[0].Key)
	}
	if got := selectors[0].Matrix["python"]; len(got) != 1 || got[0] != variant("3.11") {
		t.Errorf("selectors[0].Matrix[\"python\"] = %v, want [\"3.11\"]", got)
	}
}

func TestParseMatrixDependsOnSelectorArray(t *testing.T) {
	// Test array syntax
	input := []any{
		map[string]any{
			"key": "ray-build",
			"matrix": map[string]any{
				"python": []any{"3.10", "3.11"}, // array of values
			},
		},
	}

	selectors, err := parseMatrixDependsOn(input)
	if err != nil {
		t.Fatalf("parseMatrixDependsOn() error = %v", err)
	}

	if len(selectors) != 1 {
		t.Fatalf("len(selectors) = %d, want 1", len(selectors))
	}
	got := selectors[0].Matrix["python"]
	if len(got) != 2 || got[0] != variant("3.10") || got[1] != variant("3.11") {
		t.Errorf("selectors[0].Matrix[\"python\"] = %v, want [\"3.10\", \"3.11\"]", got)
	}
}

func TestExpandMatrixSelector(t *testing.T) {
	cfg := &matrixConfig{
		Setup: map[dimension][]variant{
			"python": {"3.10", "3.11"},
			"cuda":   {"12.1.1", "12.8.1"},
		},
	}

	stepKeyToConfig := map[string]*matrixConfig{
		"ray-build": cfg,
	}

	tests := []struct {
		name    string
		sel     *matrixSelector
		wantLen int
		wantErr bool
	}{
		{
			name: "partial match - python only",
			sel: &matrixSelector{
				Key:    "ray-build",
				Matrix: map[dimension][]variant{"python": {"3.11"}},
			},
			wantLen: 2, // 3.11 with both cuda versions
		},
		{
			name: "partial match - cuda only",
			sel: &matrixSelector{
				Key:    "ray-build",
				Matrix: map[dimension][]variant{"cuda": {"12.1.1"}},
			},
			wantLen: 2, // 12.1.1 with both python versions
		},
		{
			name: "exact match",
			sel: &matrixSelector{
				Key:    "ray-build",
				Matrix: map[dimension][]variant{"python": {"3.11"}, "cuda": {"12.1.1"}},
			},
			wantLen: 1,
		},
		{
			name: "invalid dimension",
			sel: &matrixSelector{
				Key:    "ray-build",
				Matrix: map[dimension][]variant{"invalid": {"value"}},
			},
			wantErr: true,
		},
		{
			name: "no match",
			sel: &matrixSelector{
				Key:    "ray-build",
				Matrix: map[dimension][]variant{"python": {"3.12"}}, // 3.12 doesn't exist
			},
			wantErr: true,
		},
		{
			name: "multi-value match",
			sel: &matrixSelector{
				Key:    "ray-build",
				Matrix: map[dimension][]variant{"python": {"3.10", "3.11"}},
			},
			wantLen: 4, // both python versions with both cuda versions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.sel.expand(stepKeyToConfig)
			if tt.wantErr {
				if err == nil {
					t.Error("expand() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expand() error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("expand() returned %d matches, want %d: %v", len(got), tt.wantLen, got)
			}
		})
	}
}

func TestHasMatrixPlaceholder(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello {{matrix}}", true},
		{"{{matrix.python}}", true},
		{"no placeholder", false},
		{"{{variations}}", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := hasMatrixPlaceholder(tt.input); got != tt.want {
				t.Errorf("hasMatrixPlaceholder(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseMatrixConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{
			name:    "invalid type",
			input:   42,
			wantErr: "must be an array or map",
		},
		{
			name:    "missing setup",
			input:   map[string]any{"adjustments": []any{}},
			wantErr: "must have 'setup' key",
		},
		{
			name:    "invalid setup type",
			input:   map[string]any{"setup": "invalid"},
			wantErr: "must be a map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseMatrixConfig(tt.input)
			if err == nil {
				t.Fatal("parseMatrixConfig() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseMatrixConfig() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
