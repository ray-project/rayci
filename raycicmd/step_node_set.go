package raycicmd

import (
	"fmt"
	"sort"
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
	nodes    map[string]*stepNode
	keyNodes map[string]*stepNode
}

func newStepNodeSet() *stepNodeSet {
	return &stepNodeSet{
		nodes: make(map[string]*stepNode),
	}
}

func (s *stepNodeSet) add(node *stepNode) {
	s.nodes[node.id] = node
}

func (s *stepNodeSet) buildIndex() error {
	var ids []string
	for id := range s.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	s.keyNodes = make(map[string]*stepNode, len(s.nodes))
	for _, id := range ids {
		key := s.nodes[id].nodeKey()
		if key == "" {
			continue
		}
		if _, ok := s.keyNodes[key]; ok {
			return fmt.Errorf("duplicate node key %q", key)
		}
		s.keyNodes[key] = s.nodes[id]
	}

	return nil
}

// byKey returns the node of the key. This only works after buildIndex
// is called and succeeds.
func (s *stepNodeSet) byKey(key string) (*stepNode, bool) {
	node, ok := s.keyNodes[key]
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
