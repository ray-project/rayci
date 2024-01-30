package raycicmd

import (
	"testing"

	"reflect"
	"sort"
)

type testDepNode struct {
	dependencies []string
	marked       bool
}

func newTestDepNode(deps []string) *testDepNode {
	return &testDepNode{dependencies: deps}
}
func (n *testDepNode) deps() []string { return n.dependencies }
func (n *testDepNode) mark()          { n.marked = true }

type testDepGraph struct{ nodes map[string]*testDepNode }

func newTestDepGraph(graph map[string][]string) *testDepGraph {
	nodes := make(map[string]*testDepNode)
	for name, deps := range graph {
		nodes[name] = newTestDepNode(deps)
	}
	return &testDepGraph{nodes: nodes}
}

func (g *testDepGraph) depNode(id string) depNode {
	n, ok := g.nodes[id]
	if !ok {
		return nil
	}
	return n
}

func (g *testDepGraph) marked() []string {
	var marked []string
	for name, node := range g.nodes {
		if node.marked {
			marked = append(marked, name)
		}
	}
	sort.Strings(marked)
	return marked
}

func TestMarkDeps(t *testing.T) {
	for _, test := range []struct {
		graph  map[string][]string
		starts []string
		want   []string
	}{{
		graph:  map[string][]string{},
		starts: []string{},
		want:   nil,
	}, {
		graph: map[string][]string{
			"a": {"b"},
			"b": {},
		},
		starts: []string{"a"},
		want:   []string{"a", "b"},
	}, {
		graph: map[string][]string{
			"a": {"b"},
			"b": {},
		},
		starts: []string{"b"},
		want:   []string{"b"},
	}, {
		graph: map[string][]string{
			"a": {"b"},
			"b": {},
		},
		starts: nil,
		want:   nil,
	}, {
		graph: map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"d"},
			"d": {},
		},
		starts: []string{"a"},
		want:   []string{"a", "b", "c", "d"},
	}, {
		graph: map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"d"},
		},
		starts: []string{"a", "e"},
		want:   []string{"a", "b", "c"},
	}, {
		graph: map[string][]string{
			"a": {"a"},
		},
		starts: []string{"a"},
		want:   []string{"a"},
	}, {
		graph: map[string][]string{
			"a": {"b"},
			"b": {"a"},
		},
		starts: []string{"a"},
		want:   []string{"a", "b"},
	}, {
		graph: map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"a"},
			"d": {"e"},
		},
		starts: []string{"a"},
		want:   []string{"a", "b", "c"},
	}} {
		graph := newTestDepGraph(test.graph)
		set := make(map[string]struct{})
		for _, name := range test.starts {
			set[name] = struct{}{}
		}
		markDeps(graph, set)
		got := graph.marked()
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"mark graph %v with %v, got %v, want %v",
				test.graph, test.starts, got, test.want,
			)
		}
	}
}
