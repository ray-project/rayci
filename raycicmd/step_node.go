package raycicmd

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

	// Marked is set to true when the node will be included in a conversion.
	marked bool
}

func (n *stepNode) hasTags() bool { return len(n.tags) > 0 }

func (n *stepNode) hasTagIn(tags []string) bool {
	return intersects(n.tags, tags)
}
