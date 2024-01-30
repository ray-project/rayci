package raycicmd

import (
	"testing"

	"reflect"
	"sort"
)

type testDepNode struct {
	d []string
	m bool
}

func newTestDepNode(deps []string) *testDepNode {
	return &testDepNode{d: deps}
}
func (n *testDepNode) deps() []string { return n.d }
func (n *testDepNode) mark()          { n.m = true }

type testDepGraph struct{ g map[string]*testDepNode }

func newTestDepGraph(g map[string][]string) *testDepGraph {
	nodes := make(map[string]*testDepNode)
	for k, deps := range g {
		nodes[k] = newTestDepNode(deps)
	}
	return &testDepGraph{g: nodes}
}

func (g *testDepGraph) depNode(id string) depNode {
	n, ok := g.g[id]
	if !ok {
		return nil
	}
	return n
}

func (g *testDepGraph) marked() []string {
	var marked []string
	for k, node := range g.g {
		if node.m {
			marked = append(marked, k)
		}
	}
	sort.Strings(marked)
	return marked
}

func TestMarkDeps(t *testing.T) {
	for _, test := range []struct {
		g    map[string][]string
		s    []string
		want []string
	}{{
		g:    map[string][]string{},
		s:    []string{},
		want: nil,
	}, {
		g: map[string][]string{
			"a": {"b"},
			"b": {},
		},
		s:    []string{"a"},
		want: []string{"a", "b"},
	}, {
		g: map[string][]string{
			"a": {"b"},
			"b": {},
		},
		s:    []string{"b"},
		want: []string{"b"},
	}, {
		g: map[string][]string{
			"a": {"b"},
			"b": {},
		},
		s:    nil,
		want: nil,
	}, {
		g: map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"d"},
			"d": {},
		},
		s:    []string{"a"},
		want: []string{"a", "b", "c", "d"},
	}, {
		g: map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"d"},
		},
		s:    []string{"a", "e"},
		want: []string{"a", "b", "c"},
	}, {
		g: map[string][]string{
			"a": {"a"},
		},
		s:    []string{"a"},
		want: []string{"a"},
	}, {
		g: map[string][]string{
			"a": {"b"},
			"b": {"a"},
		},
		s:    []string{"a"},
		want: []string{"a", "b"},
	}, {
		g: map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"a"},
			"d": {"e"},
		},
		s:    []string{"a"},
		want: []string{"a", "b", "c"},
	}} {
		g := newTestDepGraph(test.g)
		set := make(map[string]struct{})
		for _, id := range test.s {
			set[id] = struct{}{}
		}
		markDeps(g, set)
		got := g.marked()
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"mark graph %v with %v, got %v, want %v",
				test.g, test.s, got, test.want,
			)
		}
	}
}
