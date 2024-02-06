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
		want: &stepFilter{tags: []string{"RAYCI_COVERAGE"}},
	}, {
		cmd:  []string{"echo", "RAYCI_COVERAGE\n"},
		want: &stepFilter{tags: []string{"RAYCI_COVERAGE"}},
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
		want:     &stepFilter{skipTags: []string{"disabled"}, runAll: true},
	}, {
		cmd:     []string{"exit", "1"},
		wantErr: true,
	}, {
		cmd:  []string{"./local-not-exist.sh"},
		want: &stepFilter{runAll: true},
	}} {
		got, err := newTagsStepFilter(test.skipTags, test.cmd)
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
		skipTags: []string{"disabled"},
		tags:     []string{"tune"},
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
		skipTags: []string{"disabled"},
		tags:     []string{"tune"},
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
		skipTags: []string{"disabled"},
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
