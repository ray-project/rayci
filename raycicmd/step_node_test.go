package raycicmd

import (
	"reflect"
	"testing"
)

func TestStepNodeDeps(t *testing.T) {
	n := &stepNode{id: "mine"}

	if got := n.deps(); len(got) != 0 {
		t.Errorf("got deps %v, want empty list", n.deps())
	}

	n.addDep("foo")
	want := []string{"foo"}
	if got := n.deps(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	n.addDep("bar")
	want = []string{"bar", "foo"}
	if got := n.deps(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	n.addDep("foo")
	if got := n.deps(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestStepNodeReverseDeps(t *testing.T) {
	n := &stepNode{id: "mine"}

	if got := n.reverseDeps(); len(got) != 0 {
		t.Errorf("got deps %v, want empty list", n.deps())
	}

	n.addReverseDep("foo")
	want := []string{"foo"}
	if got := n.reverseDeps(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	n.addReverseDep("bar")
	want = []string{"bar", "foo"}
	if got := n.reverseDeps(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	n.addReverseDep("foo")
	if got := n.reverseDeps(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
