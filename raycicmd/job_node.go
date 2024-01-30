package raycicmd

import (
	"sort"
)

// jobNode is a node for a job node or a group in the pipeline.
type jobNode struct {
	id   string
	tags []string

	userKey string // user defined key

	// fields used for groups
	srcGroup *pipelineGroup // set for group nodes
	steps    []*jobNode

	// fields used for steps
	srcStep map[string]any // set for steps

	dependsOn map[string]struct{}

	// mark if this node should be included
	include bool
}

func (n *jobNode) addDependsOn(id string) {
	if n.dependsOn == nil {
		n.dependsOn = make(map[string]struct{})
	}
	n.dependsOn[id] = struct{}{}
}

func (n *jobNode) deps() []string {
	var deps []string
	for id := range n.dependsOn {
		deps = append(deps, id)
	}
	sort.Strings(deps)
	return deps
}

func (n *jobNode) mark() { n.include = true }

func (n *jobNode) selectKeys() []string {
	var keys []string
	keys = append(keys, n.id)
	if n.userKey != "" {
		keys = append(keys, n.userKey)
	}
	return keys
}
