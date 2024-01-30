package raycicmd

import (
	"fmt"
)

type nodeGraph struct {
	nodes   map[string]*jobNode
	nameMap map[string]*jobNode

	includes map[string]struct{}
}

func newNodeGraph() *nodeGraph {
	return &nodeGraph{
		nodes:    make(map[string]*jobNode),
		nameMap:  make(map[string]*jobNode),
		includes: make(map[string]struct{}),
	}
}

func (g *nodeGraph) add(node *jobNode) {
	g.nodes[node.id] = node
}

func (g *nodeGraph) addName(id, name string) error {
	if _, ok := g.nameMap[name]; ok {
		return fmt.Errorf("duplicate node key %q", name)
	}
	g.nameMap[name] = g.nodes[id]
	return nil
}

func (g *nodeGraph) byName(name string) (*jobNode, bool) {
	node, ok := g.nameMap[name]
	return node, ok
}

func (g *nodeGraph) depNode(id string) depNode {
	n := g.nodes[id]
	if n == nil {
		return nil // For interface casting
	}
	return n
}

func (g *nodeGraph) markDeps(set map[string]struct{}) { markDeps(g, set) }
