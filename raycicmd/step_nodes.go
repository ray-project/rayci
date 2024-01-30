package raycicmd

import (
	"fmt"
)

type stepNodeSet struct {
	nodes   map[string]*stepNode
	nameMap map[string]*stepNode
}

func newStepNodeSet() *stepNodeSet {
	return &stepNodeSet{
		nodes:   make(map[string]*stepNode),
		nameMap: make(map[string]*stepNode),
	}
}

func (g *stepNodeSet) add(node *stepNode) {
	g.nodes[node.id] = node
}

func (g *stepNodeSet) addName(id, name string) error {
	if _, ok := g.nameMap[name]; ok {
		return fmt.Errorf("duplicate node key %q", name)
	}
	g.nameMap[name] = g.nodes[id]
	return nil
}

func (g *stepNodeSet) byName(name string) (*stepNode, bool) {
	node, ok := g.nameMap[name]
	return node, ok
}

func (g *stepNodeSet) depNode(id string) depNode {
	n := g.nodes[id]
	if n == nil {
		return nil // For interface casting
	}
	return n
}

func (g *stepNodeSet) markDeps(starts map[string]struct{}) {
	markDeps(g, starts)
}
