package wanda

import (
	"testing"

	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

func TestSpecMarshalLoopback(t *testing.T) {
	spec := &Spec{
		Name:       "hello",
		Froms:      []string{"ubuntu:22.04"},
		Dockerfile: "ci/docker/hello.Dockerfile",
		Tags:       []string{"cr.ray.io/rayproject/hello"},
		BuildArgs:  []string{"RAYCI_BUILDID"},

		BuildHintArgs: []string{"REMOTE_CACHE_URL"},
	}

	bs, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	loopback := new(Spec)
	if err := yaml.Unmarshal(bs, loopback); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(spec, loopback) {
		t.Fatalf("got %+v, want %+v", loopback, spec)
	}
}

func TestExpandVar(t *testing.T) {
	envs := map[string]string{
		"NAME":      "RAY",
		"Name":      "Ray",
		"name":      "ray",
		"name0":     "ray0",
		"NAME_NAME": "RAY_RAY",
		"0":         "invalid env key, won't capture",
		"0A":        "invalid env key, won't capture",
	}

	for _, test := range []struct {
		in   string
		want string
	}{
		{"$NAME", "RAY"},
		{"My name is $Name", "My name is Ray"},
		{"my name is $name!", "my name is ray!"},
		{"my $$name0 is $name0~", "my $name0 is ray0~"},
		{"", ""},
		{"$", "$"},
		{"$$", "$"},
		{"$0", "$0"},
		{"$0A", "$0A"},
		{"$NAME-$NAME", "RAY-RAY"},
		{"$NAME$NAME", "RAYRAY"},
		{"$NAME_NAME", "RAY_RAY"},
	} {
		got := expandVar(test.in, func(k string) (string, bool) {
			v, ok := envs[k]
			return v, ok
		})
		if got != test.want {
			t.Errorf("expandVar(%q) got %q, want %q", test.in, got, test.want)
		}
	}
}

func TestSpecExpand(t *testing.T) {
	spec := &Spec{
		Name:       "$NAME",
		Froms:      []string{"ubuntu:$UBUNTU_VERSION"},
		Dockerfile: "ci/docker/hello.Dockerfile",
		Tags:       []string{"cr.ray.io/rayproject/hello"},
		BuildArgs: []string{
			"RAYCI_BUILDID=$$RAYCI_BUILDID",
			"UBUNTU_VERSION=$UBUNTU_VERSION",
		},
		BuildHintArgs: []string{
			"REMOTE_CACHE_URL=$REMOTE_CACHE_URL",
		},
	}

	envs := map[string]string{
		"NAME":             "hello",
		"UBUNTU_VERSION":   "22.04",
		"RAYCI_BUILDID":    "abc123",
		"REMOTE_CACHE_URL": "http://localhost:5000",
	}

	expanded := spec.expandVar(func(k string) (string, bool) {
		v, ok := envs[k]
		return v, ok
	})

	want := &Spec{
		Name:       "hello",
		Froms:      []string{"ubuntu:22.04"},
		Dockerfile: "ci/docker/hello.Dockerfile",
		Tags:       []string{"cr.ray.io/rayproject/hello"},
		BuildArgs: []string{
			"RAYCI_BUILDID=$RAYCI_BUILDID",
			"UBUNTU_VERSION=22.04",
		},
		BuildHintArgs: []string{
			"REMOTE_CACHE_URL=http://localhost:5000",
		},
	}

	if !reflect.DeepEqual(expanded, want) {
		t.Errorf("got %+v, want %+v", expanded, want)
	}
}

func TestParseSpecFile(t *testing.T) {
	tmpDir := t.TempDir()

	specFile := filepath.Join(tmpDir, "spec.yaml")
	spec := strings.Join([]string{
		"name: hello",
		"froms: [ubuntu:22.04]",
		"dockerfile: ci/docker/hello.Dockerfile",
		"tags: [cr.ray.io/rayproject/hello]",
		`build_args: ["RAYCI_BUILDID", "UBUNTU_VERSION"]`,
		`build_hint_args: ["REMOTE_CACHE_URL"]`,
	}, "\n") + "\n"

	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	got, err := parseSpecFile(specFile)
	if err != nil {
		t.Fatalf("parse spec file: %v", err)
	}

	want := &Spec{
		Name:          "hello",
		Froms:         []string{"ubuntu:22.04"},
		Dockerfile:    "ci/docker/hello.Dockerfile",
		Tags:          []string{"cr.ray.io/rayproject/hello"},
		BuildArgs:     []string{"RAYCI_BUILDID", "UBUNTU_VERSION"},
		BuildHintArgs: []string{"REMOTE_CACHE_URL"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestExtractVarNames(t *testing.T) {
	for _, test := range []struct {
		in   string
		want []string
	}{
		{"$NAME", []string{"NAME"}},
		{"$NAME$VERSION", []string{"NAME", "VERSION"}},
		{"foo$BAR$BAZ", []string{"BAR", "BAZ"}},
		{"no vars", nil},
		{"$$escaped", nil},
		{"$", nil},
		{"$0invalid", nil},
		{"$NAME_VAR", []string{"NAME_VAR"}},
		{"prefix$VAR1-$VAR2suffix", []string{"VAR1", "VAR2suffix"}},
	} {
		got := extractVarNames(test.in)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("extractVarNames(%q) = %v, want %v", test.in, got, test.want)
		}
	}
}

func TestExpandVarWithParams(t *testing.T) {
	for _, test := range []struct {
		name   string
		s      string
		params map[string][]string
		want   []string
	}{
		{
			name:   "no vars",
			s:      "hello",
			params: nil,
			want:   []string{"hello"},
		},
		{
			name:   "single var single value",
			s:      "foo$PY",
			params: map[string][]string{"PY": {"3.10"}},
			want:   []string{"foo3.10"},
		},
		{
			name:   "single var multiple values",
			s:      "foo$PY",
			params: map[string][]string{"PY": {"3.10", "3.11", "3.12"}},
			want:   []string{"foo3.10", "foo3.11", "foo3.12"},
		},
		{
			name:   "multiple vars cartesian product",
			s:      "$PY-$ARCH",
			params: map[string][]string{"PY": {"3.10", "3.11"}, "ARCH": {"x86", "arm"}},
			want:   []string{"3.10-x86", "3.10-arm", "3.11-x86", "3.11-arm"},
		},
		{
			name:   "var without params preserved",
			s:      "$PY-$UNKNOWN",
			params: map[string][]string{"PY": {"3.10"}},
			want:   []string{"3.10-$UNKNOWN"},
		},
		{
			name:   "no params for any var",
			s:      "$FOO$BAR",
			params: map[string][]string{},
			want:   []string{"$FOO$BAR"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := expandVarWithParams(test.s, test.params)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("expandVarWithParams(%q) = %v, want %v", test.s, got, test.want)
			}
		})
	}
}

func TestSpecExpandedNames(t *testing.T) {
	spec := &Spec{
		Name:   "myimage$PY_VERSION",
		Params: map[string][]string{"PY_VERSION": {"3.10", "3.11", "3.12"}},
	}

	got := spec.ExpandedNames()
	want := []string{"myimage3.10", "myimage3.11", "myimage3.12"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandedNames() = %v, want %v", got, want)
	}
}

func TestSpecExpandedFroms(t *testing.T) {
	spec := &Spec{
		Name: "myimage$PY_VERSION",
		Froms: []string{
			"cr.ray.io/rayproject/base$PY_VERSION",
			"ubuntu:22.04",
		},
		Params: map[string][]string{"PY_VERSION": {"3.10", "3.11"}},
	}

	got := spec.ExpandedFroms()
	want := []string{
		"cr.ray.io/rayproject/base3.10",
		"cr.ray.io/rayproject/base3.11",
		"ubuntu:22.04",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandedFroms() = %v, want %v", got, want)
	}
}

func TestSpecExpandedFromsDedup(t *testing.T) {
	spec := &Spec{
		Froms: []string{
			"base$PY",
			"base$PY",
		},
		Params: map[string][]string{"PY": {"3.10"}},
	}

	got := spec.ExpandedFroms()
	want := []string{"base3.10"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandedFroms() = %v, want %v (should be deduplicated)", got, want)
	}
}

func TestValidateParams(t *testing.T) {
	spec := &Spec{
		Name:   "myimage$PY_VERSION",
		Params: map[string][]string{"PY_VERSION": {"3.10", "3.11", "3.12"}},
	}

	t.Run("valid value", func(t *testing.T) {
		lookup := func(k string) (string, bool) {
			if k == "PY_VERSION" {
				return "3.11", true
			}
			return "", false
		}
		if err := spec.ValidateParams(lookup); err != nil {
			t.Errorf("ValidateParams() unexpected error: %v", err)
		}
	})

	t.Run("invalid value", func(t *testing.T) {
		lookup := func(k string) (string, bool) {
			if k == "PY_VERSION" {
				return "3.9", true
			}
			return "", false
		}
		err := spec.ValidateParams(lookup)
		if err == nil {
			t.Error("ValidateParams() expected error for invalid value")
		}
		if !strings.Contains(err.Error(), "3.9") {
			t.Errorf("error should mention invalid value '3.9': %v", err)
		}
	})

	t.Run("unset var", func(t *testing.T) {
		lookup := func(k string) (string, bool) {
			return "", false
		}
		if err := spec.ValidateParams(lookup); err != nil {
			t.Errorf("ValidateParams() should not error on unset vars: %v", err)
		}
	})
}

func TestParseSpecFileWithParams(t *testing.T) {
	tmpDir := t.TempDir()

	specFile := filepath.Join(tmpDir, "spec.yaml")
	spec := strings.Join([]string{
		"name: rayml$PY_VERSION",
		"params:",
		"  PY_VERSION:",
		"    - '3.10'",
		"    - '3.11'",
		"    - '3.12'",
		"froms: [cr.ray.io/rayproject/base$PY_VERSION]",
		"dockerfile: rayml.Dockerfile",
	}, "\n") + "\n"

	if err := os.WriteFile(specFile, []byte(spec), 0644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	got, err := parseSpecFile(specFile)
	if err != nil {
		t.Fatalf("parse spec file: %v", err)
	}

	want := &Spec{
		Name:       "rayml$PY_VERSION",
		Params:     map[string][]string{"PY_VERSION": {"3.10", "3.11", "3.12"}},
		Froms:      []string{"cr.ray.io/rayproject/base$PY_VERSION"},
		Dockerfile: "rayml.Dockerfile",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// Verify ExpandedFroms works on parsed spec
	gotFroms := got.ExpandedFroms()
	wantFroms := []string{
		"cr.ray.io/rayproject/base3.10",
		"cr.ray.io/rayproject/base3.11",
		"cr.ray.io/rayproject/base3.12",
	}
	if !reflect.DeepEqual(gotFroms, wantFroms) {
		t.Errorf("ExpandedFroms() = %v, want %v", gotFroms, wantFroms)
	}
}
