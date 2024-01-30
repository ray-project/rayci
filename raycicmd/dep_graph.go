package raycicmd

type depNode interface {
	deps() []string
	mark() // mark if this node should be included
}

type depGraph interface {
	depNode(id string) depNode
}

func markDeps(g depGraph, set map[string]struct{}) {
	hit := make(map[string]struct{})
	for id := range set {
		hit[id] = struct{}{}
	}

	// For any included node, also include their dependencies.
	this := make(map[string]struct{})
	for id := range set {
		this[id] = struct{}{}
	}

	// BFS to include all dependencies.
	for len(this) > 0 {
		next := make(map[string]struct{})
		for id := range this {
			node := g.depNode(id)
			if node == nil { // sliently skip non-existing nodes
				continue
			}

			node.mark()
			for _, dep := range node.deps() {
				if _, ok := hit[dep]; !ok {
					next[dep] = struct{}{}
					hit[dep] = struct{}{}
				}
			}
		}
		this = next
	}
}
