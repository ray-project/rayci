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

	dependsOn map[string]struct{}

	// marked is set to true when the node will be included in a conversion.
	marked bool
}

func (n *stepNode) addDependsOn(id string) {
	if n.dependsOn == nil {
		n.dependsOn = make(map[string]struct{})
	}
	n.dependsOn[id] = struct{}{}
}

func (n *stepNode) deps() []string {
	var deps []string
	for id := range n.dependsOn {
		deps = append(deps, id)
	}
	sort.Strings(deps)
	return deps
}

func (n *stepNode) mark() { n.marked = true }

func (n *stepNode) selectKeys() []string {
	var keys []string
	keys = append(keys, n.id)
	if n.key != "" {
		keys = append(keys, n.key)
	}
	return keys
}
