package raycicmd

import "testing"

func TestStepNodeSet(t *testing.T) {
	set := newStepNodeSet()

	set.add(&stepNode{id: "node1"})
	set.add(&stepNode{id: "node2", key: "test-foo"})
	set.add(&stepNode{id: "node3", key: "test-bar"})

	got, ok := set.byID("node1")
	if !ok || got.id != "node1" {
		t.Errorf("byID(node1), got %s, ok=%v", got, ok)
	}

	got, ok = set.byID("non-exist")
	if ok || got != nil {
		t.Errorf("byID(non-exist), got %s, ok=%v", got, ok)
	}

	if err := set.buildIndex(); err != nil {
		t.Fatalf("buildIndex, got %v", err)
	}

	got, ok = set.byKey("test-foo")
	if !ok || got.id != "node2" {
		t.Errorf("byKey(test-foo), got %s, ok=%v", got, ok)
	}

	got, ok = set.byKey("test-bar")
	if !ok || got.id != "node3" {
		t.Errorf("byKey(test-bar), got %s, ok=%v", got, ok)
	}
}

func TestStepNodeSet_keyConflict(t *testing.T) {
	set := newStepNodeSet()

	set.add(&stepNode{id: "node0", key: "best"})
	set.add(&stepNode{id: "node1", key: "best"})

	if err := set.buildIndex(); err == nil {
		t.Fatalf("buildIndex got no error, expected error")
	}
}
