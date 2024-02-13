package raycicmd

import (
	"testing"

	"reflect"
	"sort"
	"strings"
)

func TestStepNodeSet(t *testing.T) {
	set := newStepNodeSet()

	set.add(&stepNode{id: "node1"})
	set.add(&stepNode{id: "node2", key: "test-foo"})
	set.add(&stepNode{id: "node3", key: "test-bar"})

	got, ok := set.byID("node1")
	if !ok || got.id != "node1" {
		t.Errorf("byID(node1), got %s, ok=%v", got, ok)
	}

	got, ok = set.byID("non-exist")
	if ok || got != nil {
		t.Errorf("byID(non-exist), got %s, ok=%v", got, ok)
	}

	if err := set.buildIndex(); err != nil {
		t.Fatalf("buildIndex, got %v", err)
	}

	got, ok = set.byKey("test-foo")
	if !ok || got.id != "node2" {
		t.Errorf("byKey(test-foo), got %s, ok=%v", got, ok)
	}

	got, ok = set.byKey("test-bar")
	if !ok || got.id != "node3" {
		t.Errorf("byKey(test-bar), got %s, ok=%v", got, ok)
	}
}

func TestStepNodeSet_keyConflict(t *testing.T) {
	set := newStepNodeSet()

	set.add(&stepNode{id: "node0", key: "best"})
	set.add(&stepNode{id: "node1", key: "best"})

	if err := set.buildIndex(); err == nil {
		t.Fatalf("buildIndex got no error, expected error")
	}
}

func TestStepNodeSet_deps(t *testing.T) {
	makeSet := func(ids ...string) map[string]struct{} {
		set := make(map[string]struct{})
		for _, id := range ids {
			set[id] = struct{}{}
		}
		return set
	}

	for i, test := range []struct {
		graph    map[string]string
		includes string
		excludes string

		wantHits string
	}{{
		graph: map[string]string{
			"a": "",
			"b": "a",
		},
		includes: "a",
		wantHits: "a",
	}, {
		graph: map[string]string{
			"a": "",
			"b": "a",
		},
		includes: "b",
		wantHits: "a b",
	}, {
		graph: map[string]string{
			"a": "",
			"b": "a",
			"c": "b",
			"d": "a",
			"e": "d",
		},
		includes: "c e",
		wantHits: "a b c d e",
	}, {
		graph: map[string]string{
			"a": "",
			"b": "a",
			"c": "b",
			"d": "a",
			"e": "d",
		},
		includes: "c e",
		wantHits: "a b c d e",
	}, {
		graph: map[string]string{
			"a": "",
			"b": "a",
			"c": "b",
			"d": "a",
			"e": "d",
		},
		includes: "c e",
		excludes: "b", // c depends on b
		wantHits: "a d e",
	}, {
		graph: map[string]string{
			"a": "",
			"b": "a",
			"c": "b",
			"d": "a",
			"e": "d",
		},
		includes: "c e",
		excludes: "a",
		wantHits: "",
	}, {
		// This is a weird case. The target include is not really included
		// in the graph. The graph calculation is correct, there should be
		// extra checks on if the requested include nodes are picked.
		graph: map[string]string{
			"a": "",
			"b": "a",
			"c": "a",
			"d": "b c",
		},
		includes: "d",
		excludes: "b",
		wantHits: "a c",
	}} {
		set := newStepNodeSet()

		for id := range test.graph {
			set.add(&stepNode{id: id})
		}

		for id, deps := range test.graph {
			depsIDs := strings.Fields(deps)
			for _, dep := range depsIDs {
				set.addDep(id, dep)
			}
		}

		if err := set.buildIndex(); err != nil {
			t.Fatalf("buildIndex: %v", err)
		}

		set.markDeps(makeSet(strings.Fields(test.includes)...))
		set.rejectDeps(makeSet(strings.Fields(test.excludes)...))

		var hits []string
		for id := range test.graph {
			node, ok := set.byID(id)
			if !ok {
				t.Fatalf("case %d, byID(%s) failed", i, id)
			}
			if node.hit() {
				hits = append(hits, id)
			}
		}
		sort.Strings(hits)

		var want []string
		if test.wantHits != "" {
			want = strings.Fields(test.wantHits)
		}
		if !reflect.DeepEqual(hits, want) {
			t.Errorf("case %d, hits: got %v, want %v", i, hits, want)
		}
	}
}
