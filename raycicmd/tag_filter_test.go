package raycicmd

import (
	"reflect"
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
func TestTagFilter(t *testing.T) {
	for _, test := range []struct {
		cmd     []string
		want    *tagFilter
		wantErr bool
	}{{
		cmd:  []string{"echo", "RAYCI_COVERAGE"},
		want: &tagFilter{tags: []string{"RAYCI_COVERAGE"}},
	}, {
		cmd:  []string{"echo", "\t  \n  \t"},
		want: &tagFilter{},
	}, {
		cmd:  []string{},
		want: runAllTags,
	}, {
		cmd:     []string{"exit", "1"},
		wantErr: true,
	}, {
		cmd:  []string{"./local-not-exist.sh"},
		want: runAllTags,
	}} {
		got, err := runTagFilterCommand(test.cmd)
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
