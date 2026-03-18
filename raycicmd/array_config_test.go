package raycicmd

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestParseArrayConfig(t *testing.T) {
	input := map[string]any{
		"python": []any{"3.10", "3.11"},
		"cuda":   []any{"12.1.1", "12.8.1"},
	}

	cfg, err := parseArrayConfig(input)
	if err != nil {
		t.Fatalf("parseArrayConfig() error = %v", err)
	}

	if got := cfg.dims["python"]; !reflect.DeepEqual(got, []string{"3.10", "3.11"}) {
		t.Errorf("dims[\"python\"] = %v, want [\"3.10\", \"3.11\"]", got)
	}
	if got := cfg.dims["cuda"]; !reflect.DeepEqual(got, []string{"12.1.1", "12.8.1"}) {
		t.Errorf("dims[\"cuda\"] = %v, want [\"12.1.1\", \"12.8.1\"]", got)
	}
}

func TestExpandArray(t *testing.T) {
	cfg := &arrayConfig{
		dims: map[string][]string{
			"python": {"3.10", "3.11"},
			"cuda":   {"12.1.1", "12.8.1"},
		},
	}

	elements := cfg.expand()

	if len(elements) != 4 {
		t.Fatalf("len(elements) = %d, want 4", len(elements))
	}

	var combos []string
	for _, elem := range elements {
		combos = append(combos, elem.values["python"]+"-"+elem.values["cuda"])
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

func TestGenerateArrayElementKey(t *testing.T) {
	elem := &arrayElement{values: map[string]string{"python": "3.11", "cuda": "12.1.1"}}

	got := elem.generateKey("ray-build")
	want := "ray-build--cuda1211-python311"
	if got != want {
		t.Errorf("generateKey() = %q, want %q", got, want)
	}
}

func TestSubstituteValues(t *testing.T) {
	tests := []struct {
		name  string
		input any
		elem  *arrayElement
		want  any
	}{
		{
			name:  "named dimensions",
			input: "python {{array.python}} cuda {{array.cuda}}",
			elem:  &arrayElement{values: map[string]string{"python": "3.11", "cuda": "12.1.1"}},
			want:  "python 3.11 cuda 12.1.1",
		},
		{
			name:  "nested map",
			input: map[string]any{"cmd": "./build.sh --python={{array.python}}"},
			elem:  &arrayElement{values: map[string]string{"python": "3.11"}},
			want:  map[string]any{"cmd": "./build.sh --python=3.11"},
		},
		{
			name:  "array",
			input: []any{"echo {{array.python}}", "echo {{array.cuda}}"},
			elem:  &arrayElement{values: map[string]string{"python": "3.11", "cuda": "12.1.1"}},
			want:  []any{"echo 3.11", "echo 12.1.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.elem.substituteValues(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("substituteValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasArrayPlaceholder(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"{{array.python}}", true},
		{"hello {{array.os}}", true},
		{"no placeholder", false},
		{"{{matrix}}", false},
		{"{{variations}}", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := hasArrayPlaceholder(tt.input); got != tt.want {
				t.Errorf("hasArrayPlaceholder(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseArrayAdjustments(t *testing.T) {
	input := []any{
		map[string]any{
			"with": map[string]any{"os": "windows", "arch": "arm64"},
			"skip": true,
		},
		map[string]any{
			"with": map[string]any{"os": "Plan 9", "arch": "arm64"},
		},
	}

	adjs, err := parseArrayAdjustments(input)
	if err != nil {
		t.Fatalf("parseArrayAdjustments() error = %v", err)
	}
	if len(adjs) != 2 {
		t.Fatalf("len(adjs) = %d, want 2", len(adjs))
	}

	if !adjs[0].skip {
		t.Error("adjs[0].skip = false, want true")
	}
	if adjs[0].with["os"] != "windows" || adjs[0].with["arch"] != "arm64" {
		t.Errorf("adjs[0].with = %v, want {os:windows, arch:arm64}", adjs[0].with)
	}

	if adjs[1].skip {
		t.Error("adjs[1] should be a pure addition, got skip=true")
	}
}

func TestParseArrayAdjustmentsErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{
			name:    "not an array",
			input:   "bad",
			wantErr: "must be an array",
		},
		{
			name:    "element not a map",
			input:   []any{"bad"},
			wantErr: "must be a map",
		},
		{
			name:    "missing with",
			input:   []any{map[string]any{"skip": true}},
			wantErr: "missing required \"with\"",
		},
		{
			name:    "with not a map",
			input:   []any{map[string]any{"with": "bad"}},
			wantErr: "\"with\" must be a map",
		},
		{
			name:    "empty with",
			input:   []any{map[string]any{"with": map[string]any{}}},
			wantErr: "\"with\" cannot be empty",
		},
		{
			name: "with value not a string",
			input: []any{map[string]any{
				"with": map[string]any{"os": 42},
			}},
			wantErr: "must be a string",
		},
		{
			name: "duplicate with",
			input: []any{
				map[string]any{
					"with": map[string]any{"os": "linux"},
					"skip": true,
				},
				map[string]any{
					"with": map[string]any{"os": "linux"},
				},
			},
			wantErr: "duplicate \"with\"",
		},
		{
			name: "unknown key",
			input: []any{map[string]any{
				"with": map[string]any{"os": "linux"},
				"skp":  true,
			}},
			wantErr: "unknown key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseArrayAdjustments(tt.input)
			if err == nil {
				t.Fatal("parseArrayAdjustments() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseArrayConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{
			name:    "invalid type",
			input:   42,
			wantErr: "must be a map",
		},
		{
			name:    "array not supported",
			input:   []any{"a", "b"},
			wantErr: "must be a map",
		},
		{
			name:    "empty map",
			input:   map[string]any{},
			wantErr: "cannot be empty",
		},
		{
			name:    "dimension not an array",
			input:   map[string]any{"python": "3.11"},
			wantErr: "must be an array",
		},
		{
			name:    "empty dimension",
			input:   map[string]any{"python": []any{}},
			wantErr: "cannot be empty",
		},
		{
			name:    "only adjustments key",
			input:   map[string]any{"adjustments": []any{}},
			wantErr: "at least one dimension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseArrayConfig(tt.input)
			if err == nil {
				t.Fatal("parseArrayConfig() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseArrayConfig() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
