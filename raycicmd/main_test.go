package raycicmd

import (
	"testing"

	"bytes"
	"os"
	"path/filepath"
	"reflect"

	yaml "gopkg.in/yaml.v3"
)

func TestMainFunction(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "pipeline.yaml")

	envs := newEnvsMap(map[string]string{
		"BUILDKITE":          "true",
		"BUILDKITE_BUILD_ID": "fake-id",
	})
	args := []string{
		"rayci",
		"-repo", dir,
		"-output", output,
	}

	if err := Main(args, envs); err != nil {
		t.Fatal(err)
	}

	bs, err := os.ReadFile(output)
	if err != nil {
		t.Fatal("read output: ", err)
	}

	bk := &bkPipeline{}
	if err := yaml.Unmarshal(bs, bk); err != nil {
		t.Fatal("output is not a valid buildkite pipeline: ", err)
		t.Log(bs)
	}
}

func TestExecWithInput(t *testing.T) {
	out := new(bytes.Buffer)
	if err := execWithInput(
		"cat", []string{"-"},
		[]byte("hello"), out,
	); err != nil {
		t.Fatal(err)
	}

	if got, want := out.String(), "hello"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMakeBuildInfo(t *testing.T) {
	flags := &Flags{}
	envs := newEnvsMap(map[string]string{
		"RAYCI_BUILD_ID":   "fake-build-id",
		"BUILDKITE_COMMIT": "abc123",
		"RAYCI_BRANCH":     "foobar",
		"RAYCI_SELECT":     "foo,bar,taz",
	})

	info, err := makeBuildInfo(flags, envs)
	if err != nil {
		t.Fatal("make build info: ", err)
	}

	want := &buildInfo{
		buildID:        "fake-build-id",
		launcherBranch: "foobar",
		gitCommit:      "abc123",
		selects:        []string{"foo", "bar", "taz"},
	}
	if !reflect.DeepEqual(info, want) {
		t.Errorf("got %+v, want %+v", info, want)
	}

	flags.Select = "gee,goo" // overwrites env var
	info, err = makeBuildInfo(flags, envs)
	if err != nil {
		t.Fatal("make build info with selects overwrite: ", err)
	}
	want = &buildInfo{
		buildID:        "fake-build-id",
		launcherBranch: "foobar",
		gitCommit:      "abc123",
		selects:        []string{"gee", "goo"},
	}
	if !reflect.DeepEqual(info, want) {
		t.Errorf("got %+v, want %+v", info, want)
	}
}
