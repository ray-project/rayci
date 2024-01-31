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

func (s *stepNodeSet) depNode(id string) depNode {
	n := s.nodes[id]
	if n == nil {
		return nil // For interface casting
	}
	return n
}

func (s *stepNodeSet) markDeps(starts map[string]struct{}) {
	markDeps(s, starts)
}
