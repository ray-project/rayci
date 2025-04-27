package raycicmd

import (
	"testing"

	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"

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

func TestLoadConfig_fromFile(t *testing.T) {
	tmp := t.TempDir()
	envs := newEnvsMap(map[string]string{})

	configFile := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(configFile, []byte(strings.Join([]string{
		"ci_temp: /tmp/rayci",
		"ci_work_repo: ray-project/rayci",
	}, "\n")), 0644); err != nil {
		t.Fatal("write config file: ", err)
	}

	c, err := loadConfig(configFile, "", envs)
	if err != nil {
		t.Fatal("load config: ", err)
	}

	want := &config{
		CIWorkRepo: "ray-project/rayci",
		CITemp:     "/tmp/rayci",
	}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("got %+v, want %+v", c, want)
	}
}

func TestLoadConfig_customBuildkiteDirs(t *testing.T) {
	envs := newEnvsMap(map[string]string{})
	c, err := loadConfig("", ".buildkite/premerge:.buildkite/common", envs)
	if err != nil {
		t.Fatal("load config: ", err)
	}

	want := []string{
		".buildkite/premerge",
		".buildkite/common",
	}
	if !reflect.DeepEqual(c.BuildkiteDirs, want) {
		t.Errorf("got %v, want %v", c.BuildkiteDirs, want)
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
		"RAYCI_BUILD_ID":                "fake-build-id",
		"BUILDKITE_COMMIT":              "abc123",
		"BUILDKITE_BUILD_CREATOR_EMAIL": "reef@anyscale.com",
		"RAYCI_BRANCH":                  "foobar",
		"RAYCI_SELECT":                  "foo,bar,taz",
	})

	info, err := makeBuildInfo(flags, envs)
	if err != nil {
		t.Fatal("make build info: ", err)
	}

	want := &buildInfo{
		buildID:          "fake-build-id",
		buildAuthorEmail: "reef@anyscale.com",
		launcherBranch:   "foobar",
		gitCommit:        "abc123",
		selects:          []string{"foo", "bar", "taz"},
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
		buildID:          "fake-build-id",
		buildAuthorEmail: "reef@anyscale.com",
		launcherBranch:   "foobar",
		gitCommit:        "abc123",
		selects:          []string{"gee", "goo"},
	}
	if !reflect.DeepEqual(info, want) {
		t.Errorf("got %+v, want %+v", info, want)
	}
}
