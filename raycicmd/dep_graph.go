package raycicmd

// depNode is a node in a dependency graph.
type depNode interface {
	// deps lists all its dependencies.
	deps() []string

	// mark marks this node as being included in the dependency graph.
	mark()
}

// depGraph defines the interface for a dependency graph.
// where each node has a unique name in the form of a string.
type depGraph interface {
	depNode(id string) depNode
}

// markDeps marks all nodes in a dependency graph that are reachable from the
// given set of starting nodes of starts.
func markDeps(g depGraph, starts map[string]struct{}) {
	hit := make(map[string]bool)
	for id := range starts {
		hit[id] = true
	}

	// For any included node, also include their dependencies.
	this := make(map[string]bool)
	for id := range starts {
		this[id] = true
	}

	// BFS to include all dependencies.
	for len(this) > 0 {
		next := make(map[string]bool)
		for id := range this {
			node := g.depNode(id)
			if node == nil { // sliently skip non-existing nodes
				continue
			}

			node.mark()
			for _, dep := range node.deps() {
				if !hit[dep] {
					next[dep] = true
					hit[dep] = true
				}
			}
		}
		this = next
	}
}
