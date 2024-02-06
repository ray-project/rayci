package raycicmd

import (
	"fmt"
	"sort"
)

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
