package raycicmd

import (
	"testing"

	"reflect"
)

func TestNewTagsStepFilter(t *testing.T) {
	for _, test := range []struct {
		cmd      []string
		skipTags []string
		want     *stepFilter
		wantErr  bool
	}{{
		cmd:  []string{"echo", "RAYCI_COVERAGE"},
		want: &stepFilter{tags: stringSet("RAYCI_COVERAGE")},
	}, {
		cmd:  []string{"echo", "RAYCI_COVERAGE\n"},
		want: &stepFilter{tags: stringSet("RAYCI_COVERAGE")},
	}, {
		cmd:  []string{"echo", "\t  \n  \t"},
		want: &stepFilter{},
	}, {
		cmd:  []string{},
		want: &stepFilter{runAll: true},
	}, {
		cmd:  nil,
		want: &stepFilter{runAll: true},
	}, {
		cmd:  []string{"echo", "*"},
		want: &stepFilter{runAll: true},
	}, {
		skipTags: []string{"disabled"},
		want:     &stepFilter{skipTags: stringSet("disabled"), runAll: true},
	}, {
		cmd:     []string{"exit", "1"},
		wantErr: true,
	}, {
		cmd:  []string{"./local-not-exist.sh"},
		want: &stepFilter{runAll: true},
	}} {
		got, err := newStepFilter(test.skipTags, nil, test.cmd)
		if test.wantErr {
			if err == nil {
				t.Errorf("run %q: want error, got nil", test.cmd)
			}
			continue
		}
		if err != nil {
			t.Fatalf("run %q: %s", test.cmd, err)
		}

		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"run %q: got %+v, want %+v",
				test.cmd, got, test.want,
			)
		}
	}
}

func TestStepFilter_tags(t *testing.T) {
	filter := &stepFilter{
		skipTags: stringSet("disabled"),
		tags:     stringSet("tune"),
	}

	for _, tags := range [][]string{
		{},
		{"tune"},
		{"tune", "foo"},
		{"bar", "tune"},
	} {
		node := &stepNode{tags: tags}

		if !filter.hit(node) {
			t.Errorf("miss %+v", tags)
		}

		if !filter.accept(node) {
			t.Errorf("not accepting %+v", tags)
		}
	}

	for _, tags := range [][]string{
		// Even with "disabled" in the tags list, accept will return true, as it
		// only checks for tags matching.
		{"tune", "data", "disabled"},
	} {
		node := &stepNode{tags: tags}
		if !filter.accept(node) {
			t.Errorf("not accepting %+v", tags)
		}
	}

	for _, tags := range [][]string{
		{"data"},
	} {
		if filter.accept(&stepNode{tags: tags}) {
			t.Errorf("accept %+v, should not", tags)
		}
	}

	for _, tags := range [][]string{
		{"disabled"},
		{"data"},
		{"tune", "disabled"},
		{"disabled", "tune"},
	} {
		if filter.hit(&stepNode{tags: tags}) {
			t.Errorf("hit %+v", tags)
		}
	}
}

func TestStepFilter_tagsReject(t *testing.T) {
	filter := &stepFilter{
		skipTags: stringSet("disabled"),
		tags:     stringSet("tune"),
	}

	for _, tags := range [][]string{
		{},
		{"tune"},
		{"tune", "foo"},
		{"bar", "tune"},
		{"data"},
	} {
		if filter.reject(&stepNode{tags: tags}) {
			t.Errorf("rejects %+v", tags)
		}
	}

	for _, tags := range [][]string{
		{"disabled"},
		{"tune", "disabled"},
		{"disabled", "tune"},
	} {
		if !filter.reject(&stepNode{tags: tags}) {
			t.Errorf("does not reject %+v", tags)
		}
	}
}

func TestStepFilter_runAll(t *testing.T) {
	filter := &stepFilter{
		skipTags: stringSet("disabled"),
		runAll:   true,
	}

	for _, tags := range [][]string{
		nil,
		{},
		{"data"},
		{"tune"},
		{"tune", "foo"},
		{"bar", "tune"},
	} {
		if !filter.hit(&stepNode{tags: tags}) {
			t.Errorf("miss %+v", tags)
		}
	}

	for _, tags := range [][]string{
		{"tune", "disabled"},
		{"disabled", "tune"},
	} {
		if filter.hit(&stepNode{tags: tags}) {
			t.Errorf("hit %+v", tags)
		}
	}
}

func TestStepFilter_selects(t *testing.T) {
	filter, _ := newStepFilter([]string{"disabled"}, []string{"foo", "bar"}, nil)
	for _, node := range []*stepNode{
		{key: "foo"},
		{id: "foo"},
		{id: "bar"},
		{id: "foo", key: "k"},
		{id: "id", key: "foo"},
		{id: "disabled", key: "bar"},
		{id: "foo", tags: []string{"bar"}},

		// even disabled nodes can be selected
		{id: "foo", tags: []string{"disabled"}},
		{key: "bar", tags: []string{"disabled"}},
	} {
		if !filter.accept(node) {
			t.Errorf("miss %+v", node)
		}
	}

	filter, _ = newStepFilter([]string{"disabled"}, []string{"foo", "bar"}, nil)
	for _, node := range []*stepNode{
		{key: "f"},
		{id: "f"},
		{id: "f", tags: []string{"disabled"}},
		{key: "b", tags: []string{"disabled"}},
	} {
		if filter.accept(node) {
			t.Errorf("hit %+v", node)
		}
	}
}

func TestStepFilter_tagSelects(t *testing.T) {
	filter, _ := newStepFilter(nil, []string{"tag:foo", "bar"}, nil)
	for _, node := range []*stepNode{
		{key: "bar"},
		{id: "id", tags: []string{"foo"}},
	} {
		if !filter.accept(node) {
			t.Errorf("tag select miss %+v", node)
		}
	}
}

func TestStepFilter_selectsAndTags(t *testing.T) {
	filter, _ := newStepFilter(
		[]string{"disabled"},
		[]string{"foo", "bar", "tag:pick"},
		[]string{"echo", "tune"},
	)

	for _, node := range []*stepNode{
		{key: "foo"},
		{id: "foo", tags: []string{"tune"}},
		{id: "other", tags: []string{"pick", "tune"}},
		{id: "bar"},
	} {
		if !filter.accept(node) {
			t.Errorf("miss %+v", node)
		}
	}

	for _, node := range []*stepNode{
		{id: "foo", tags: []string{"not_tune"}},
		{id: "bar", tags: []string{"tune_not"}},
		{key: "w00t"},
	} {
		if filter.accept(node) {
			t.Errorf("hit %+v", node)
		}
	}
}
