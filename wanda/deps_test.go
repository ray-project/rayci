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

const testPrefix = "cr.ray.io/rayproject/"

func TestBuildDepGraph_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	writeSpec(t, tmpDir, "base.wanda.yaml", strings.Join([]string{
		"name: base",
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := buildDepGraph(filepath.Join(tmpDir, "base.wanda.yaml"), noopLookup, testPrefix)
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

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Linear chain: A depends on B, B depends on C
	//
	//   a
	//   │
	//   ▼
	//   b
	//   │
	//   ▼
	//   c
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

	graph, err := buildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup, testPrefix)
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

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

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

	graph, err := buildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup, testPrefix)
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

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

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

	_, err := buildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup, testPrefix)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}

	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestBuildDepGraph_VariableExpansion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

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

	graph, err := buildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), lookup, testPrefix)
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

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	writeSpec(t, tmpDir, "app.wanda.yaml", strings.Join([]string{
		"name: app-$VERSION",
		"dockerfile: Dockerfile",
	}, "\n"))

	_, err := buildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), noopLookup, testPrefix)
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

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	writeSpec(t, tmpDir, "app.wanda.yaml", strings.Join([]string{
		"name: app-$VERSION",
		`froms: ["base-$PYTHON_VERSION"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	_, err := buildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), noopLookup, testPrefix)
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

func TestFindRepoRoot(t *testing.T) {
	// Create a fake repo structure
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(tmpDir, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// From subdir, should find repo root
	got := findRepoRoot(subDir)
	if got != tmpDir {
		t.Errorf("findRepoRoot(%q) = %q, want %q", subDir, got, tmpDir)
	}

	// From repo root itself
	got = findRepoRoot(tmpDir)
	if got != tmpDir {
		t.Errorf("findRepoRoot(%q) = %q, want %q", tmpDir, got, tmpDir)
	}
}

func TestFindRepoRoot_NoGit(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// No .git found, should return start dir
	got := findRepoRoot(subDir)
	if got != subDir {
		t.Errorf("findRepoRoot(%q) = %q, want %q (no .git)", subDir, got, subDir)
	}
}

func TestDiscoverSpecs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create specs in different directories
	baseDir := filepath.Join(tmpDir, "base")
	appDir := filepath.Join(tmpDir, "app")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeSpec(t, baseDir, "base.wanda.yaml", strings.Join([]string{
		"name: base-image",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, appDir, "app.wanda.yaml", strings.Join([]string{
		"name: app-image",
		"dockerfile: Dockerfile",
	}, "\n"))

	index, err := discoverSpecs(tmpDir, noopLookup)
	if err != nil {
		t.Fatalf("discoverSpecs: %v", err)
	}

	if len(index) != 2 {
		t.Errorf("index has %d entries, want 2", len(index))
	}

	if path, ok := index["base-image"]; !ok {
		t.Error("index missing base-image")
	} else if !strings.HasSuffix(path, "base.wanda.yaml") {
		t.Errorf("base-image path = %q, want suffix base.wanda.yaml", path)
	}

	if path, ok := index["app-image"]; !ok {
		t.Error("index missing app-image")
	} else if !strings.HasSuffix(path, "app.wanda.yaml") {
		t.Errorf("app-image path = %q, want suffix app.wanda.yaml", path)
	}
}

func TestDiscoverSpecs_NameCollision(t *testing.T) {
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	// Two specs with same name
	writeSpec(t, dir1, "a.wanda.yaml", strings.Join([]string{
		"name: same-name",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, dir2, "b.wanda.yaml", strings.Join([]string{
		"name: same-name",
		"dockerfile: Dockerfile",
	}, "\n"))

	_, err := discoverSpecs(tmpDir, noopLookup)
	if err == nil {
		t.Fatal("expected error for name collision, got nil")
	}

	if !strings.Contains(err.Error(), "same-name") {
		t.Errorf("error should mention conflicting name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "multiple") {
		t.Errorf("error should mention 'multiple', got: %v", err)
	}
}

func TestDiscoverSpecs_WithVariables(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "base.wanda.yaml", strings.Join([]string{
		"name: base-$VERSION",
		"dockerfile: Dockerfile",
	}, "\n"))

	lookup := func(key string) (string, bool) {
		if key == "VERSION" {
			return "1.0", true
		}
		return "", false
	}

	index, err := discoverSpecs(tmpDir, lookup)
	if err != nil {
		t.Fatalf("discoverSpecs: %v", err)
	}

	if _, ok := index["base-1.0"]; !ok {
		t.Errorf("index missing expanded name base-1.0, got: %v", index)
	}
}

func TestDiscoverSpecs_WithParams(t *testing.T) {
	tmpDir := t.TempDir()

	// Spec with params - should be indexed under all expanded names
	writeSpec(t, tmpDir, "base.wanda.yaml", strings.Join([]string{
		"name: base$PY",
		"params:",
		"  PY:",
		"    - '3.10'",
		"    - '3.11'",
		"    - '3.12'",
		"dockerfile: Dockerfile",
	}, "\n"))

	// No env vars needed - params provide the values
	index, err := discoverSpecs(tmpDir, noopLookup)
	if err != nil {
		t.Fatalf("discoverSpecs: %v", err)
	}

	// All three expanded names should be indexed
	for _, name := range []string{"base3.10", "base3.11", "base3.12"} {
		if _, ok := index[name]; !ok {
			t.Errorf("index missing %q, got: %v", name, index)
		}
	}

	// All should point to the same spec file
	path := index["base3.10"]
	if index["base3.11"] != path || index["base3.12"] != path {
		t.Errorf("all names should map to same path, got: %v", index)
	}
}

func TestDiscoverSpecs_ParamsAndEnvFallback(t *testing.T) {
	tmpDir := t.TempDir()

	// Spec with partial params - one var has params, one needs env
	writeSpec(t, tmpDir, "base.wanda.yaml", strings.Join([]string{
		"name: base$PY-$ARCH",
		"params:",
		"  PY:",
		"    - '3.10'",
		"dockerfile: Dockerfile",
	}, "\n"))

	lookup := func(key string) (string, bool) {
		if key == "ARCH" {
			return "amd64", true
		}
		return "", false
	}

	index, err := discoverSpecs(tmpDir, lookup)
	if err != nil {
		t.Fatalf("discoverSpecs: %v", err)
	}

	// Should be indexed as base3.10-amd64
	if _, ok := index["base3.10-amd64"]; !ok {
		t.Errorf("index missing base3.10-amd64, got: %v", index)
	}
}

func TestBuildDepGraph_Discovery(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

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

	graph, err := buildDepGraph(filepath.Join(appDir, "app.wanda.yaml"), noopLookup, testPrefix)
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

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// App references an image with namePrefix but no wanda spec - treated as external
	writeSpec(t, tmpDir, "app.wanda.yaml", strings.Join([]string{
		"name: app",
		`froms: ["cr.ray.io/rayproject/external-base"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	g, err := buildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), noopLookup, testPrefix)
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

	// validateDeps should also pass - external deps are skipped
	if err := g.validateDeps(); err != nil {
		t.Errorf("validateDeps() unexpected error: %v", err)
	}
}

func TestBuildDepGraph_TransitiveDeps(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .git to mark repo root
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

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

	graph, err := buildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup, testPrefix)
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

func TestBuildDepGraph_ParamsValidation(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "spec.wanda.yaml", strings.Join([]string{
		"name: myimage$PY_VERSION",
		"params:",
		"  PY_VERSION:",
		"    - '3.10'",
		"    - '3.11'",
		"dockerfile: Dockerfile",
	}, "\n"))

	t.Run("valid param value", func(t *testing.T) {
		lookup := func(k string) (string, bool) {
			if k == "PY_VERSION" {
				return "3.10", true
			}
			return "", false
		}
		_, err := buildDepGraph(filepath.Join(tmpDir, "spec.wanda.yaml"), lookup, "")
		if err != nil {
			t.Errorf("unexpected error with valid param: %v", err)
		}
	})

	t.Run("invalid param value", func(t *testing.T) {
		lookup := func(k string) (string, bool) {
			if k == "PY_VERSION" {
				return "3.9", true
			}
			return "", false
		}
		_, err := buildDepGraph(filepath.Join(tmpDir, "spec.wanda.yaml"), lookup, "")
		if err == nil {
			t.Error("expected error for invalid param value")
		}
		if !strings.Contains(err.Error(), "3.9") {
			t.Errorf("error should mention invalid value '3.9': %v", err)
		}
	})
}

func TestBuildDepGraph_UnexpandedWithParamsHint(t *testing.T) {
	tmpDir := t.TempDir()

	// Spec with params but env var not set
	writeSpec(t, tmpDir, "spec.wanda.yaml", strings.Join([]string{
		"name: myimage$PY_VERSION",
		"params:",
		"  PY_VERSION:",
		"    - '3.10'",
		"    - '3.11'",
		"dockerfile: Dockerfile",
	}, "\n"))

	// No env var set - should get helpful error with valid values
	_, err := buildDepGraph(filepath.Join(tmpDir, "spec.wanda.yaml"), noopLookup, "")
	if err == nil {
		t.Fatal("expected error for unexpanded env var")
	}

	// Error should mention valid values from params
	errStr := err.Error()
	if !strings.Contains(errStr, "PY_VERSION") {
		t.Errorf("error should mention PY_VERSION: %v", err)
	}
	if !strings.Contains(errStr, "valid values") {
		t.Errorf("error should mention valid values: %v", err)
	}
	if !strings.Contains(errStr, "3.10") || !strings.Contains(errStr, "3.11") {
		t.Errorf("error should list valid values 3.10, 3.11: %v", err)
	}
}

func TestBuildDepGraph_DiscoveryWithParams(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	// Base spec with params - discoverable via params, loadable with env var
	baseDir := filepath.Join(tmpDir, "base")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, baseDir, "base.wanda.yaml", strings.Join([]string{
		"name: base$PY",
		"params:",
		"  PY:",
		"    - '3.10'",
		"    - '3.11'",
		"dockerfile: Dockerfile",
	}, "\n"))

	// App spec depends on base3.10
	appDir := filepath.Join(tmpDir, "app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, appDir, "app.wanda.yaml", strings.Join([]string{
		"name: app",
		`froms: ["cr.ray.io/rayproject/base3.10"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	// Discovery finds base3.10 via params (no env var needed for discovery).
	// Loading the spec requires env var to be set for expansion.
	lookup := func(key string) (string, bool) {
		if key == "PY" {
			return "3.10", true
		}
		return "", false
	}

	graph, err := buildDepGraph(filepath.Join(appDir, "app.wanda.yaml"), lookup, testPrefix)
	if err != nil {
		t.Fatalf("buildDepGraph: %v", err)
	}

	// base3.10 was discovered via params and loaded with PY=3.10
	if graph.Specs["base3.10"] == nil {
		t.Error("expected base3.10 in graph")
	}

	if len(graph.Order) != 2 {
		t.Errorf("Order has %d items, want 2", len(graph.Order))
	}
}
