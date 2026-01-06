package wanda

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLocalDeps(t *testing.T) {
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
			froms: []string{"@base"},
			want:  []string{"base"},
		},
		{
			name:  "mixed deps",
			froms: []string{"ubuntu:22.04", "@base", "cr.ray.io/rayproject/hello", "@other"},
			want:  []string{"base", "other"},
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
			got := localDeps(spec)
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

func TestBuildDepGraph_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "base.wanda.yaml", strings.Join([]string{
		"name: base",
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := BuildDepGraph(filepath.Join(tmpDir, "base.wanda.yaml"), noopLookup)
	if err != nil {
		t.Fatalf("BuildDepGraph: %v", err)
	}

	if graph.Root() != "base" {
		t.Errorf("Root() = %q, want %q", graph.Root(), "base")
	}

	order := graph.Order()
	if len(order) != 1 || order[0] != "base" {
		t.Errorf("Order() = %v, want [base]", order)
	}
}

func TestBuildDepGraph_LinearChain(t *testing.T) {
	tmpDir := t.TempDir()

	// A -> B -> C (A depends on B, B depends on C)
	writeSpec(t, tmpDir, "c.wanda.yaml", strings.Join([]string{
		"name: c",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "b.wanda.yaml", strings.Join([]string{
		"name: b",
		"deps: [c.wanda.yaml]",
		`froms: ["@c"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		"deps: [b.wanda.yaml]",
		`froms: ["@b"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := BuildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup)
	if err != nil {
		t.Fatalf("BuildDepGraph: %v", err)
	}

	if graph.Root() != "a" {
		t.Errorf("Root() = %q, want %q", graph.Root(), "a")
	}

	order := graph.Order()
	// C must come before B, B must come before A
	cIdx, bIdx, aIdx := -1, -1, -1
	for i, name := range order {
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
		t.Fatalf("Order() = %v, missing expected specs", order)
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

	// Diamond: A -> B, A -> C, B -> D, C -> D
	writeSpec(t, tmpDir, "d.wanda.yaml", strings.Join([]string{
		"name: d",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "b.wanda.yaml", strings.Join([]string{
		"name: b",
		"deps: [d.wanda.yaml]",
		`froms: ["@d"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "c.wanda.yaml", strings.Join([]string{
		"name: c",
		"deps: [d.wanda.yaml]",
		`froms: ["@d"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		"deps: [b.wanda.yaml, c.wanda.yaml]",
		`froms: ["@b", "@c"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := BuildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup)
	if err != nil {
		t.Fatalf("BuildDepGraph: %v", err)
	}

	order := graph.Order()
	if len(order) != 4 {
		t.Fatalf("Order() has %d items, want 4", len(order))
	}

	// Find indices
	indices := make(map[string]int)
	for i, name := range order {
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

	// Cycle: A -> B -> A
	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		"deps: [b.wanda.yaml]",
		`froms: ["@b"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "b.wanda.yaml", strings.Join([]string{
		"name: b",
		"deps: [a.wanda.yaml]",
		`froms: ["@a"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	_, err := BuildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}

	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestBuildDepGraph_MissingDepFile(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		"deps: [nonexistent.wanda.yaml]",
		"dockerfile: Dockerfile",
	}, "\n"))

	_, err := BuildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup)
	if err == nil {
		t.Fatal("expected error for missing dep file, got nil")
	}
}

func TestBuildDepGraph_VariableExpansion(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "base.wanda.yaml", strings.Join([]string{
		"name: base-$VERSION",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "app.wanda.yaml", strings.Join([]string{
		"name: app-$VERSION",
		"deps: [base.wanda.yaml]",
		`froms: ["@base-$VERSION"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	lookup := func(key string) (string, bool) {
		if key == "VERSION" {
			return "1.0", true
		}
		return "", false
	}

	graph, err := BuildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), lookup)
	if err != nil {
		t.Fatalf("BuildDepGraph: %v", err)
	}

	if graph.Root() != "app-1.0" {
		t.Errorf("Root() = %q, want %q", graph.Root(), "app-1.0")
	}

	if graph.Get("base-1.0") == nil {
		t.Error("expected base-1.0 in graph")
	}
}

func TestDepGraph_ValidateDeps(t *testing.T) {
	tmpDir := t.TempDir()

	// A references @missing which is not in deps
	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		`froms: ["@missing"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := BuildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup)
	if err != nil {
		t.Fatalf("BuildDepGraph: %v", err)
	}

	err = graph.ValidateDeps()
	if err == nil {
		t.Fatal("expected validation error for missing dep, got nil")
	}

	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should mention missing dep, got: %v", err)
	}
}

func TestBuildDepGraph_TransitiveDeps(t *testing.T) {
	tmpDir := t.TempDir()

	// A -> B -> C, A only lists B in deps (not C)
	// C should still be discovered transitively
	writeSpec(t, tmpDir, "c.wanda.yaml", strings.Join([]string{
		"name: c",
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "b.wanda.yaml", strings.Join([]string{
		"name: b",
		"deps: [c.wanda.yaml]",
		`froms: ["@c"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	writeSpec(t, tmpDir, "a.wanda.yaml", strings.Join([]string{
		"name: a",
		"deps: [b.wanda.yaml]",
		`froms: ["@b"]`,
		"dockerfile: Dockerfile",
	}, "\n"))

	graph, err := BuildDepGraph(filepath.Join(tmpDir, "a.wanda.yaml"), noopLookup)
	if err != nil {
		t.Fatalf("BuildDepGraph: %v", err)
	}

	// All three should be in the graph
	if len(graph.Order()) != 3 {
		t.Errorf("Order() has %d items, want 3", len(graph.Order()))
	}

	if graph.Get("c") == nil {
		t.Error("expected c in graph (transitive dep)")
	}
}

func noopLookup(key string) (string, bool) {
	return "", false
}

func TestBuildDepGraph_UnexpandedEnvVar(t *testing.T) {
	tmpDir := t.TempDir()

	writeSpec(t, tmpDir, "app.wanda.yaml", strings.Join([]string{
		"name: app-$VERSION",
		"dockerfile: Dockerfile",
	}, "\n"))

	_, err := BuildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), noopLookup)
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

	_, err := BuildDepGraph(filepath.Join(tmpDir, "app.wanda.yaml"), noopLookup)
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
