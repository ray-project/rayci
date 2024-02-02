package raycicmd

import (
	"testing"

	"reflect"
)

func TestIntersects(t *testing.T) {
	for _, test := range []struct {
		set1 []string
		set2 []string
		want bool
	}{{
		set1: []string{"foo", "bar"},
		set2: []string{"foo", "w00t"},
		want: true,
	}, {
		set1: []string{"foo", "bar"},
		set2: []string{"hi", "w00t"},
		want: false,
	}, {
		set1: []string{},
		set2: []string{},
		want: false,
	}} {
		if got := intersects(test.set1, test.set2); got != test.want {
			t.Errorf(
				"intersects %+v, %+v: got %+v, want %+v",
				test.set1, test.set2, got, test.want,
			)
		}
	}
}

func TestStepNodeHasTags(t *testing.T) {
	for _, test := range []struct {
		tags []string
		want bool
	}{{
		tags: []string{"foo", "bar"},
		want: true,
	}, {
		tags: []string{},
		want: false,
	}, {
		tags: nil,
		want: false,
	}} {
		n := &stepNode{tags: test.tags}
		if got := n.hasTags(); got != test.want {
			t.Errorf("hasTags %+v: got %+v, want %+v", test.tags, got, test.want)
		}
	}
}

func TestStepNodeHasTagIn(t *testing.T) {
	for _, test := range []struct {
		tags  []string
		check []string
		want  bool
	}{{
		tags:  []string{"foo", "bar"},
		check: []string{"foo"},
		want:  true,
	}, {
		tags:  []string{"foo", "bar"},
		check: []string{"foo", "woo"},
		want:  true,
	}, {
		tags:  []string{"foo", "bar"},
		check: []string{"woohoo"},
		want:  false,
	}, {
		tags:  []string{"foo", "bar"},
		check: []string{"FOO", "Bar"},
		want:  false,
	}, {
		tags:  []string{},
		check: []string{"foo"},
		want:  false,
	}, {
		tags:  nil,
		check: nil,
		want:  false,
	}} {
		n := &stepNode{tags: test.tags}
		if got := n.hasTagIn(test.check); got != test.want {
			t.Errorf(
				"hasTagIn(%+v, %+v): got %+v, want %+v",
				test.tags, test.check,
				got, test.want,
			)
		}
	}
}

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
