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
	ts.addFile("Dockerfile.hello", nil, "testdata/Dockerfile.hello")

	in := newBuildInput(ts, []string{"MESSAGE=test=msg"})
	in.addTag("myimage")

	core, err := in.makeCore("Dockerfile.hello")
	if err != nil {
		t.Fatalf("make build input core: %v", err)
	}

	if core.Dockerfile != "Dockerfile.hello" {
		t.Errorf("got %q, want Dockerfile.hello", core.Dockerfile)
	}
	if got := core.BuildArgs["MESSAGE"]; got != "test=msg" {
		t.Errorf("build args MESSAGE got %q, want `test=msg`", got)
	}

	digest, err := core.digest()
	if err != nil {
		t.Fatalf("compute digest: %v", err)
	}

	core.Dockerfile = "Dockerfile2"
	digest2, err := core.digest()
	if err != nil {
		t.Fatalf("compute digest for the second time: %v", err)
	}
	if digest == digest2 {
		t.Errorf("same digest after change: %q vs %q", digest, digest2)
	}

}
