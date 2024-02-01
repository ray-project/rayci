package raycicmd

import (
	"fmt"
)

type reverseStepNode struct {
	node *stepNode
}

func (n *reverseStepNode) deps() []string { return n.node.reverseDeps() }
func (n *reverseStepNode) mark()          { n.node.reject() }

type reverseStepNodeSet struct{ set *stepNodeSet }

func (s *reverseStepNodeSet) depNode(id string) depNode {
	n, ok := s.set.byID(id)
	if !ok {
		return nil // For interface casting
	}
	return &reverseStepNode{node: n}
}

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

func (s *stepNodeSet) add(node *stepNode) {
	s.nodes[node.id] = node
}

func (s *stepNodeSet) addName(id, name string) error {
	if _, ok := s.nameMap[name]; ok {
		return fmt.Errorf("duplicate node key %q", name)
	}
	s.nameMap[name] = s.nodes[id]
	return nil
}

func (s *stepNodeSet) byName(name string) (*stepNode, bool) {
	node, ok := s.nameMap[name]
	return node, ok
}

func (s *stepNodeSet) byID(id string) (*stepNode, bool) {
	node, ok := s.nodes[id]
	return node, ok
}

func (s *stepNodeSet) depNode(id string) depNode {
	n, ok := s.byID(id)
	if !ok {
		return nil // For interface casting
	}
	return n
}

func (s *stepNodeSet) markDeps(starts map[string]struct{}) {
	markDeps(s, starts)
}

func (s *stepNodeSet) rejectDeps(starts map[string]struct{}) {
	markDeps(&reverseStepNodeSet{set: s}, starts)
}

func (s *stepNodeSet) addDep(from, to string) {
	fromNode, fromOK := s.nodes[from]
	toNode, toOK := s.nodes[to]

	if !fromOK || !toOK {
		return
	}

	fromNode.addDep(to)
	toNode.addRevDep(from)
}
