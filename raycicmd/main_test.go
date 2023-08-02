package raycicmd

import (
	"testing"

	"os"
	"path/filepath"

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
