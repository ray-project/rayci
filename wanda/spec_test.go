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
