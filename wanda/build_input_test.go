package wanda

import (
	"testing"

	"reflect"
)

func TestBuildInputTags(t *testing.T) {
	ts := newTarStream()

	in := newBuildInput(ts, nil)

	in.addTag("myimage")
	in.addTag("cr.ray.io/rayproject/ray")
	in.addTag("myimage") // duplicate

	tagList := in.tagList()
	want := []string{
		"cr.ray.io/rayproject/ray",
		"myimage",
	}
	if !reflect.DeepEqual(tagList, tagList) {
		t.Errorf("got %v, want %v", tagList, want)
	}
}

func TestBuildInputCore(t *testing.T) {
	ts := newTarStream()
	ts.addFile("Dockerfile", nil, "testdata/Dockerfile")

	in := newBuildInput(ts, []string{"MESSAGE=test-msg"})
	in.addTag("myimage")

	core, err := in.makeCore("Dockerfile")
	if err != nil {
		t.Fatalf("make build input core: %v", err)
	}

	if core.Dockerfile != "Dockerfile" {
		t.Errorf("got %q, want Dockerfile", core.Dockerfile)
	}
	if got := core.BuildArgs["MESSAGE"]; got != "test-msg" {
		t.Errorf("build args MESSAGE got %q, want `test-msg`", got)
	}
}
