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

	instances := cfg.expand()

	if len(instances) != 4 {
		t.Fatalf("len(instances) = %d, want 4", len(instances))
	}

	var combos []string
	for _, inst := range instances {
		combos = append(combos, inst.values["python"]+"-"+inst.values["cuda"])
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

func TestGenerateArrayInstanceKey(t *testing.T) {
	inst := &arrayInstance{values: map[string]string{"python": "3.11", "cuda": "12.1.1"}}

	got := inst.generateKey("ray-build")
	want := "ray-build--cuda1211-python311"
	if got != want {
		t.Errorf("generateKey() = %q, want %q", got, want)
	}
}

func TestSubstituteValues(t *testing.T) {
	tests := []struct {
		name  string
		input any
		inst  *arrayInstance
		want  any
	}{
		{
			name:  "named dimensions",
			input: "python {{array.python}} cuda {{array.cuda}}",
			inst:  &arrayInstance{values: map[string]string{"python": "3.11", "cuda": "12.1.1"}},
			want:  "python 3.11 cuda 12.1.1",
		},
		{
			name:  "nested map",
			input: map[string]any{"cmd": "./build.sh --python={{array.python}}"},
			inst:  &arrayInstance{values: map[string]string{"python": "3.11"}},
			want:  map[string]any{"cmd": "./build.sh --python=3.11"},
		},
		{
			name:  "array",
			input: []any{"echo {{array.python}}", "echo {{array.cuda}}"},
			inst:  &arrayInstance{values: map[string]string{"python": "3.11", "cuda": "12.1.1"}},
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
