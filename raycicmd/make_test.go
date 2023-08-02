package raycicmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIsRayCIYaml(t *testing.T) {
	for _, f := range []string{
		"foo.rayci.yaml",
		"foo.rayci.yml",
		"dir/foo.rayci.yml",
	} {
		if !isRayCIYaml(f) {
			t.Errorf("want %q to be a rayci yaml", f)
		}
	}

	for _, f := range []string{
		"rayci.yaml",
		"pipeline.build.yaml",
		"pipeline.tests.yml",
	} {
		if isRayCIYaml(f) {
			t.Errorf("want %q to not be a rayci yaml", f)
		}
	}
}

func TestListCIYamlFiles(t *testing.T) {
	tmp := t.TempDir()

	for _, f := range []string{
		"foo.rayci.yaml",
		"bar.rayci.yaml",
		"foo.rayci.yml",
		"dir/foo.rayci.yml",
		"pipeline.build.yaml",
	} {
		dir := filepath.Join(tmp, filepath.Dir(f))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir for %q: %v", f, err)
		}

		if err := os.WriteFile(filepath.Join(tmp, f), nil, 0o600); err != nil {
			t.Fatalf("write file %q: %v", f, err)
		}
	}

	got, err := listCIYamlFiles(tmp)
	if err != nil {
		t.Fatalf("listCIYamlFiles: %v", err)
	}

	want := []string{
		"bar.rayci.yaml",
		"foo.rayci.yaml",
		"foo.rayci.yml",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
