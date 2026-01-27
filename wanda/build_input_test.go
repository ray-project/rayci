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

	core, err := in.makeCore("Dockerfile.hello", nil)
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

func TestResolveBuildArgsWithLookup(t *testing.T) {
	envfileVars := map[string]string{
		"MANYLINUX_VERSION": "260103.868e54c",
		"HOSTTYPE":          "x86_64",
	}
	lookup := func(key string) (string, bool) {
		v, ok := envfileVars[key]
		return v, ok
	}

	buildArgs := []string{
		"MANYLINUX_VERSION",     // no value, should come from lookup
		"HOSTTYPE",              // no value, should come from lookup
		"EXPLICIT_VAR=explicit", // has value, should use as-is
		"MISSING_VAR",           // no value, not in lookup or env
	}

	result := resolveBuildArgs(buildArgs, lookup)

	if got, want := result["MANYLINUX_VERSION"], "260103.868e54c"; got != want {
		t.Errorf("MANYLINUX_VERSION = %q, want %q", got, want)
	}
	if got, want := result["HOSTTYPE"], "x86_64"; got != want {
		t.Errorf("HOSTTYPE = %q, want %q", got, want)
	}
	if got, want := result["EXPLICIT_VAR"], "explicit"; got != want {
		t.Errorf("EXPLICIT_VAR = %q, want %q", got, want)
	}
	// MISSING_VAR should be empty (not in lookup or env)
	if got := result["MISSING_VAR"]; got != "" {
		t.Errorf("MISSING_VAR = %q, want empty string", got)
	}
}

func TestResolveBuildArgsWithNilLookup(t *testing.T) {
	// When lookup is nil, should fall back to os.Getenv.
	buildArgs := []string{
		"EXPLICIT_VAR=explicit",
	}

	result := resolveBuildArgs(buildArgs, nil)

	if got, want := result["EXPLICIT_VAR"], "explicit"; got != want {
		t.Errorf("EXPLICIT_VAR = %q, want %q", got, want)
	}
}
