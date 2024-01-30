package raycicmd

import (
	"testing"
)

func TestPipelineGroupLess(t *testing.T) {
	groups := []*pipelineGroup{
		{sortKey: "a", filename: "a.yaml"},
		{sortKey: "a", filename: "b.yaml"},
		{sortKey: "b", filename: "a.yaml"},
		{sortKey: "b", filename: "b.yaml"},
	}

	for i, g1 := range groups {
		if i == 0 {
			continue
		}
		for _, g2 := range groups[:i] {
			if g1.lessThan(g2) {
				t.Errorf("expected %v > %v", g1, g2)
			}

			if !g2.lessThan(g1) {
				t.Errorf("expected %v <= %v", g2, g1)
			}
		}
	}
}
