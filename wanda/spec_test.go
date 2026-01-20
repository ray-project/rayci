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

func TestSpecExpandDisableCaching(t *testing.T) {
	spec := &Spec{
		Name:           "$NAME",
		Dockerfile:     "Dockerfile",
		DisableCaching: true,
	}

	expanded := spec.expandVar(func(k string) (string, bool) {
		if k == "NAME" {
			return "hello", true
		}
		return "", false
	})

	if expanded.Name != "hello" {
		t.Errorf("Name = %q, want %q", expanded.Name, "hello")
	}
	if !expanded.DisableCaching {
		t.Errorf("DisableCaching = %v, want %v", expanded.DisableCaching, true)
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
