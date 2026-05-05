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

func TestStepNodeSelectHit(t *testing.T) {
	n := &stepNode{id: "step-id", key: "step-key"}

	set := func(selects ...string) map[string]bool {
		set := make(map[string]bool)
		for _, s := range selects {
			set[s] = true
		}
		return set
	}

	for _, test := range []struct {
		selects map[string]bool
		want    bool
	}{
		{selects: set("step-id"), want: true},
		{selects: set("step-key"), want: true},
		{selects: set("step-id", "step-key"), want: true},
		{selects: set("step-id", "step-key", "other"), want: true},
		{selects: set("other"), want: false},

		// selectHit is exact match only; prefixes do not hit.
		{selects: set("step-"), want: false},
		{selects: set("s"), want: false},
	} {
		if got := n.selectHit(test.selects); got != test.want {
			t.Errorf(
				"selectHit %+v: got %+v, want %+v",
				test.selects, got, test.want,
			)
		}
	}

	// a node with no key
	n = &stepNode{id: "step-id"}

	for _, test := range []struct {
		selects map[string]bool
		want    bool
	}{
		{selects: set("step-id"), want: true},
		{selects: set("step-key"), want: false},
		{selects: set("step-id", "step-key"), want: true},
		{selects: set("step-id", "step-key", "other"), want: true},
		{selects: set("other"), want: false},
	} {
		if got := n.selectHit(test.selects); got != test.want {
			t.Errorf(
				"selectHit %+v: got %+v, want %+v",
				test.selects, got, test.want,
			)
		}
	}
}

func TestStepNodePrefixHit(t *testing.T) {
	set := func(prefixes ...string) map[string]bool {
		m := make(map[string]bool)
		for _, p := range prefixes {
			m[p] = true
		}
		return m
	}

	n := &stepNode{id: "step-id", key: "step-key"}
	for _, test := range []struct {
		prefixes map[string]bool
		want     bool
	}{
		// exact strings still hit (a string is a prefix of itself).
		{prefixes: set("step-id"), want: true},
		{prefixes: set("step-key"), want: true},

		// proper prefixes of id or key hit.
		{prefixes: set("step-"), want: true},
		{prefixes: set("s"), want: true},

		// longer than both id and key: no hit.
		{prefixes: set("step-key-extra"), want: false},

		// not a prefix of id or key.
		{prefixes: set("tep"), want: false},

		// empty prefix is ignored: never hits.
		{prefixes: set(""), want: false},

		// at least one of multiple entries hits.
		{prefixes: set("nope", "step-"), want: true},
	} {
		if got := n.prefixHit(test.prefixes); got != test.want {
			t.Errorf(
				"prefixHit %+v: got %v, want %v",
				test.prefixes, got, test.want,
			)
		}
	}

	// id-only node: prefix on id hits.
	if !(&stepNode{id: "step-id"}).prefixHit(set("step-")) {
		t.Errorf("prefixHit on id-only node: want hit")
	}

	// key-only node: prefix on key hits.
	if !(&stepNode{key: "step-key"}).prefixHit(set("step-")) {
		t.Errorf("prefixHit on key-only node: want hit")
	}

	// empty node never hits.
	if (&stepNode{}).prefixHit(set("step-")) {
		t.Errorf("prefixHit on empty node: want miss")
	}
}
