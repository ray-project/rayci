package raycicmd

import (
	"fmt"
	"sort"
)

func intersects(set1, set2 []string) bool {
	set := make(map[string]struct{})
	for _, s := range set1 {
		set[s] = struct{}{}
	}
	for _, s := range set2 {
		if _, hit := set[s]; hit {
			return true
		}
	}
	return false
}

// stepNode is a node for a generic step. The step can be a group, a wait,
// a block or a command step.
type stepNode struct {
	id string // Unique name of a step node.

	// User defined key. Optional.
	key string

	tags []string

	// Fields used for groups.
	srcGroup *pipelineGroup // set for group nodes
	subSteps []*stepNode

	// Fields used for steps.
	src map[string]any // Source definition of the step when it is not a group.

	depSet        map[string]struct{}
	reverseDepSet map[string]struct{}

	// Marked is set to true when the node will be included in a conversion.
	marked bool
}

func (n *stepNode) nodeKey() string { return n.key }

func (n *stepNode) hasTags() bool { return len(n.tags) > 0 }

func (n *stepNode) hasTagIn(tags []string) bool {
	return intersects(n.tags, tags)
}

func (n *stepNode) addDep(id string) {
	if n.depSet == nil {
		n.depSet = make(map[string]struct{})
	}
	n.depSet[id] = struct{}{}
}

func (n *stepNode) addReverseDep(id string) {
	if n.reverseDepSet == nil {
		n.reverseDepSet = make(map[string]struct{})
	}
	n.reverseDepSet[id] = struct{}{}
}

func setToStringList(set map[string]struct{}) []string {
	var list []string
	for k := range set {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}

func (n *stepNode) deps() []string {
	return setToStringList(n.depSet)
}

func (n *stepNode) reverseDeps() []string {
	return setToStringList(n.reverseDepSet)
}

func (n *stepNode) String() string {
	if n.key != "" {
		return fmt.Sprintf("node %q (key %q)", n.id, n.key)
	}
	return fmt.Sprintf("node %q", n.id)
}
