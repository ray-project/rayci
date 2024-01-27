package raycicmd

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

	// mark if this node should be included
	include bool
}
