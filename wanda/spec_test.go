package wanda

import (
	"testing"

	"reflect"

	"gopkg.in/yaml.v3"
)

func TestParseSpecFile(t *testing.T) {
	spec := &Spec{
		Name:       "hello",
		Froms:      []string{"ubuntu:22.04"},
		Dockerfile: "ci/docker/hello.Dockerfile",
		Tags:       []string{"cr.ray.io/rayproject/hello"},
		BuildArgs:  []string{"RAYCI_BUILDID"},
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
	}

	envs := map[string]string{
		"NAME":           "hello",
		"UBUNTU_VERSION": "22.04",
		"RAYCI_BUILDID":  "abc123",
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
	}

	if !reflect.DeepEqual(expanded, want) {
		t.Errorf("got %+v, want %+v", expanded, want)
	}
}
