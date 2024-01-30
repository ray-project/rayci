package raycicmd

import (
	"sort"
)

// stepNode is a node for a job node or a group in the pipeline.
type stepNode struct {
	id   string
	tags []string

	key string // User defined key.

	// Fields used for steps.
	src map[string]any

	// Fields used for groups.
	srcGroup *pipelineGroup // set for group nodes
	subSteps []*stepNode

	dependsOn map[string]struct{}

	// marked is set to true if this node will be included
	// in the converstion.
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
