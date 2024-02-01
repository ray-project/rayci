package raycicmd

import (
	"testing"
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
