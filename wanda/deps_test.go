package wanda

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLocalDeps(t *testing.T) {
	const prefix = "cr.ray.io/rayproject/"

	tests := []struct {
		name  string
		froms []string
		want  []string
	}{
		{
			name:  "no deps",
			froms: []string{"ubuntu:22.04"},
			want:  nil,
		},
		{
			name:  "single local dep",
			froms: []string{"cr.ray.io/rayproject/base"},
			want:  []string{"base"},
		},
		{
			name:  "mixed deps",
			froms: []string{"ubuntu:22.04", "cr.ray.io/rayproject/base", "gcr.io/other/hello", "cr.ray.io/rayproject/other"},
			want:  []string{"base", "other"},
		},
		{
			name:  "at-ref not a wanda dep",
			froms: []string{"@localimage", "cr.ray.io/rayproject/base"},
			want:  []string{"base"}, // @localimage is a docker ref, not a wanda dep
		},
		{
			name:  "empty",
			froms: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &Spec{Froms: tt.froms}
			got := localDeps(spec, prefix)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("localDeps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func writeSpec(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
	return path
}

const testWandaSpecsFile = ".wandaspecs"

func writeWandaSpecs(t *testing.T, dir string, dirs []string) string {
	t.Helper()
	path := filepath.Join(dir, testWandaSpecsFile)
	content := strings.Join(dirs, "\n")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", testWandaSpecsFile, err)
	}
	return path
}

const testPrefix = "cr.ray.io/rayproject/"

func TestBuildDepGraph_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "base.wanda.yaml", strings.Join([]string{
		"name: base",
		"dockerfile: Dockerfile",
	}, "\n"))

	specsFile := filepath.Join(tmpDir, testWandaSpecsFile) // non-existent, no discovery
	graph, err := buildDepGraph(filepath.Join(tmpDir, "base.wanda.yaml"), noopLookup, testPrefix, specsFile)
	if err != nil {
		t.Fatalf("buildDepGraph: %v", err)
	}

	if graph.Root != "base" {
		t.Errorf("Root() = %q, want %q", graph.Root, "base")
	}

	if len(graph.Order) != 1 || graph.Order[0] != "base" {
		t.Errorf("Order() = %v, want [base]", graph.Order)
	}
}

func TestBuildDepGraph_LinearChain(t *testing.T) {
	tmpDir := t.TempDir()

	specsFile := writeWandaSpecs(t, tmpDir, []string{"."})

	// Linear chain: A depends on B, B depends on C
	//
	//   a --> b --> c
	//
	// Build order: c, b, a
	writeSpec(t, tmpDir, "c.wanda.yaml", strings.Join([]string{
		"name: c",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "b.wanda.yaml", strings.Join([]string{
		"name: b",
		`froms: ["cr.ray.io/rayproject/c"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		`froms: ["cr.ray.io/rayproject/b"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := buildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup, testPrefix, specsFile)
	if err != nil {
		t.Fatalf("buildDepGraph: %v", err)
	}

	if graph.Root != "a" {
		t.Errorf("Root() = %q, want %q", graph.Root, "a")
	}

	// C must come before B, B must come before A
	cIdx, bIdx, aIdx := -1, -1, -1
	for i, name := range graph.Order {
		switch name {
		case "c":
			cIdx = i
		case "b":
			bIdx = i
		case "a":
			aIdx = i
		}
	}

	if cIdx == -1 || bIdx == -1 || aIdx == -1 {
		t.Fatalf("Order() = %v, missing expected specs", graph.Order)
	}

	if cIdx > bIdx {
		t.Errorf("c (idx %d) should come before b (idx %d)", cIdx, bIdx)
	}
	if bIdx > aIdx {
		t.Errorf("b (idx %d) should come before a (idx %d)", bIdx, aIdx)
	}
}

func TestBuildDepGraph_Diamond(t *testing.T) {
	tmpDir := t.TempDir()

	specsFile := writeWandaSpecs(t, tmpDir, []string{"."})

	// Diamond: A depends on B and C, both B and C depend on D
	//
	//       a
	//      / \
	//     ▼   ▼
	//     b   c
	//      \ /
	//       ▼
	//       d
	//
	// Build order: d, b, c, a (b and c after d, a last)
	writeSpec(t, tmpDir, "d.wanda.yaml", strings.Join([]string{
		"name: d",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "b.wanda.yaml", strings.Join([]string{
		"name: b",
		`froms: ["cr.ray.io/rayproject/d"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "c.wanda.yaml", strings.Join([]string{
		"name: c",
		`froms: ["cr.ray.io/rayproject/d"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		`froms: ["cr.ray.io/rayproject/b", "cr.ray.io/rayproject/c"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := buildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup, testPrefix, specsFile)
	if err != nil {
		t.Fatalf("buildDepGraph: %v", err)
	}

	if len(graph.Order) != 4 {
		t.Fatalf("Order() has %d items, want 4", len(graph.Order))
	}

	// Find indices
	indices := make(map[string]int)
	for i, name := range graph.Order {
		indices[name] = i
	}

	// D must come before B and C
	if indices["d"] > indices["b"] {
		t.Errorf("d should come before b")
	}
	if indices["d"] > indices["c"] {
		t.Errorf("d should come before c")
	}

	// B and C must come before A
	if indices["b"] > indices["a"] {
		t.Errorf("b should come before a")
	}
	if indices["c"] > indices["a"] {
		t.Errorf("c should come before a")
	}
}

func TestBuildDepGraph_CycleDetection(t *testing.T) {
	tmpDir := t.TempDir()

	specsFile := writeWandaSpecs(t, tmpDir, []string{"."})

	// Cycle: A depends on B, B depends on A
	//
	//   a ◄──┐
	//   │    │
	//   ▼    │
	//   b ───┘
	//
	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		`froms: ["cr.ray.io/rayproject/b"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "b.wanda.yaml", strings.Join([]string{
		"name: b",
		`froms: ["cr.ray.io/rayproject/a"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	_, err := buildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup, testPrefix, specsFile)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}

	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestBuildDepGraph_VariableExpansion(t *testing.T) {
	tmpDir := t.TempDir()

	specsFile := writeWandaSpecs(t, tmpDir, []string{"."})

	writeSpec(t, tmpDir, "base.wanda.yaml", strings.Join([]string{
		"name: base-$VERSION",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "app.wanda.yaml", strings.Join([]string{
		"name: app-$VERSION",
		`froms: ["cr.ray.io/rayproject/base-$VERSION"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	lookup := func(key string) (string, bool) {
		if key == "VERSION" {
			return "1.0", true
		}
		return "", false
	}

	graph, err := buildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), lookup, testPrefix, specsFile)
	if err != nil {
		t.Fatalf("buildDepGraph: %v", err)
	}

	if graph.Root != "app-1.0" {
		t.Errorf("Root() = %q, want %q", graph.Root, "app-1.0")
	}

	if graph.Specs["base-1.0"] == nil {
		t.Error("expected base-1.0 in graph")
	}
}

func TestBuildDepGraph_UnexpandedEnvVar(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "app.wanda.yaml", strings.Join([]string{
		"name: app-$VERSION",
		"dockerfile: Dockerfile",
	}, "\n"))

	specsFile := filepath.Join(tmpDir, testWandaSpecsFile)
	_, err := buildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), noopLookup, testPrefix, specsFile)
	if err == nil {
		t.Fatal("expected error for unexpanded env var, got nil")
	}

	if !strings.Contains(err.Error(), "$VERSION") {
		t.Errorf("error should mention $VERSION, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not set") {
		t.Errorf("error should mention 'not set', got: %v", err)
	}
}

func TestBuildDepGraph_MultipleUnexpandedEnvVars(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "app.wanda.yaml", strings.Join([]string{
		"name: app-$VERSION",
		`froms: ["base-$PYTHON_VERSION"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	specsFile := filepath.Join(tmpDir, testWandaSpecsFile)
	_, err := buildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), noopLookup, testPrefix, specsFile)
	if err == nil {
		t.Fatal("expected error for unexpanded env vars, got nil")
	}

	if !strings.Contains(err.Error(), "$VERSION") {
		t.Errorf("error should mention $VERSION, got: %v", err)
	}
	if !strings.Contains(err.Error(), "$PYTHON_VERSION") {
		t.Errorf("error should mention $PYTHON_VERSION, got: %v", err)
	}
}

func noopLookup(key string) (string, bool) {
	return "", false
}

func TestBuildDepGraph_Discovery(t *testing.T) {
	tmpDir := t.TempDir()

	specsFile := writeWandaSpecs(t, tmpDir, []string{"."})

	// Base spec in a subdirectory
	baseDir := filepath.Join(tmpDir, "base")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeSpec(t, baseDir, "base.wanda.yaml", strings.Join([]string{
		"name: base",
		"dockerfile: Dockerfile",
	}, "\n"))

	// App spec references base via prefix - discovered automatically
	appDir := filepath.Join(tmpDir, "app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeSpec(t, appDir, "app.wanda.yaml", strings.Join([]string{
		"name: app",
		`froms: ["cr.ray.io/rayproject/base"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := buildDepGraph(filepath.Join(appDir, "app.wanda.yaml"), noopLookup, testPrefix, specsFile)
	if err != nil {
		t.Fatalf("buildDepGraph: %v", err)
	}

	if graph.Root != "app" {
		t.Errorf("Root() = %q, want %q", graph.Root, "app")
	}

	if len(graph.Order) != 2 {
		t.Fatalf("Order() has %d items, want 2", len(graph.Order))
	}

	// Base should be built before app
	indices := make(map[string]int)
	for i, name := range graph.Order {
		indices[name] = i
	}

	if indices["base"] > indices["app"] {
		t.Errorf("base should come before app in build order")
	}

	// Verify both specs are in the graph
	if graph.Specs["base"] == nil {
		t.Error("expected base in graph")
	}
	if graph.Specs["app"] == nil {
		t.Error("expected app in graph")
	}
}

func TestBuildDepGraph_ExternalDep(t *testing.T) {
	tmpDir := t.TempDir()

	// App references an image with namePrefix but no wanda spec - treated as external
	writeSpec(t, tmpDir, "app.wanda.yaml", strings.Join([]string{
		"name: app",
		`froms: ["cr.ray.io/rayproject/external-base"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	specsFile := filepath.Join(tmpDir, testWandaSpecsFile)
	g, err := buildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), noopLookup, testPrefix, specsFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the root spec should be in the graph; external dep is skipped
	if len(g.Specs) != 1 {
		t.Errorf("got %d specs, want 1", len(g.Specs))
	}
	if _, ok := g.Specs["app"]; !ok {
		t.Error("expected app in graph")
	}
}

func TestBuildDepGraph_TransitiveDeps(t *testing.T) {
	tmpDir := t.TempDir()

	specsFile := writeWandaSpecs(t, tmpDir, []string{"."})

	// A -> B -> C (transitive discovery)
	writeSpec(t, tmpDir, "c.wanda.yaml", strings.Join([]string{
		"name: c",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "b.wanda.yaml", strings.Join([]string{
		"name: b",
		`froms: ["cr.ray.io/rayproject/c"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		`froms: ["cr.ray.io/rayproject/b"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := buildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup, testPrefix, specsFile)
	if err != nil {
		t.Fatalf("buildDepGraph: %v", err)
	}

	// All three should be in the graph
	if len(graph.Order) != 3 {
		t.Errorf("Order() has %d items, want 3", len(graph.Order))
	}

	if graph.Specs["c"] == nil {
		t.Error("expected c in graph (transitive dep)")
	}
}
