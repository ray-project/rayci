package raycicmd

import (
	"sort"
)

// stepNode is a node for a job node or a group in the pipeline.
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

	// rejected means that this step will not be included in the conversion.
	rejected bool

	// marked is set to true when the node will be included in a conversion.
	marked bool
}

func (n *stepNode) addDep(id string) {
	if n.depSet == nil {
		n.depSet = make(map[string]struct{})
	}
	n.depSet[id] = struct{}{}
}

func (n *stepNode) addRevDep(id string) {
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

func (n *stepNode) deps() []string { return setToStringList(n.depSet) }

func (n *stepNode) reverseDeps() []string {
	return setToStringList(n.reverseDepSet)
}

func (n *stepNode) mark() { n.marked = true }

func (n *stepNode) reject() { n.rejected = true }

func (n *stepNode) hit() bool { return !n.rejected && n.marked }

func (n *stepNode) keys() []string {
	var keys []string
	keys = append(keys, n.id)
	if n.key != "" {
		keys = append(keys, n.key)
	}
	return keys
}

func (n *stepNode) hasTags() bool { return len(n.tags) > 0 }

func (n *stepNode) hasAnyTag(tags []string) bool {
	return intersects(n.tags, tags)
}

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
