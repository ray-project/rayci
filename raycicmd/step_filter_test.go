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
		if !filter.hit(&stepNode{tags: tags}) {
			t.Errorf("miss %+v", tags)
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
