package raycicmd

import (
	"testing"

	"reflect"
)

func TestBoolInMap(t *testing.T) {
	for _, test := range []struct {
		m    map[string]any
		key  string
		want bool
		ok   bool
	}{{
		m:    map[string]any{"a": true},
		key:  "a",
		want: true,
		ok:   true,
	}, {
		m:   map[string]any{"b": "a"},
		key: "a",
	}, {
		m:   map[string]any{"a": 1},
		key: "a",
	}, {
		m:   nil,
		key: "a",
	}} {
		got, ok := boolInMap(test.m, test.key)
		if got != test.want {
			t.Errorf(
				"boolInMap(%v, %v): got %v, want %v",
				test.m, test.key, got, test.want,
			)
		}
		if ok != test.ok {
			t.Errorf(
				"boolInMap(%v, %v): got ok %v, want %v",
				test.m, test.key, ok, test.ok,
			)
		}
	}
}

func TestStringHasPrefix(t *testing.T) {
	for _, test := range []struct {
		str      string
		prefixes []string
		want     bool
	}{{
		str:      "abc",
		prefixes: []string{"a"},
		want:     true,
	}, {
		str:      "",
		prefixes: []string{},
		want:     false,
	}, {
		str:      "abc",
		prefixes: []string{""},
		want:     true,
	}, {
		str:      "def",
		prefixes: []string{"a"},
		want:     false,
	}} {
		got := stringHasPrefix(test.str, test.prefixes)
		if got != test.want {
			t.Errorf(
				"stringHasPrefix(%q, %v): got %v, want %v",
				test.str, test.prefixes, got, test.want,
			)
		}
	}
}

func TestStringInMap(t *testing.T) {
	for _, test := range []struct {
		m    map[string]any
		key  string
		want string
		ok   bool
	}{{
		m:    map[string]any{"a": "b"},
		key:  "a",
		want: "b",
		ok:   true,
	}, {
		m:   map[string]any{"b": "a"},
		key: "a",
	}, {
		m:   map[string]any{"a": 1},
		key: "a",
	}, {
		m:   nil,
		key: "a",
	}} {
		got, ok := stringInMap(test.m, test.key)
		if got != test.want {
			t.Errorf(
				"stringInMap(%v, %q): got %q, want %q",
				test.m, test.key, got, test.want,
			)
		}
		if ok != test.ok {
			t.Errorf(
				"stringInMap(%v, %q): got ok %v, want %v",
				test.m, test.key, ok, test.ok,
			)
		}
	}
}

func TestIntInMap(t *testing.T) {
	for _, test := range []struct {
		m    map[string]any
		key  string
		want int
		ok   bool
	}{{
		m:    map[string]any{"a": 10},
		key:  "a",
		want: 10,
		ok:   true,
	}, {
		m:   map[string]any{"b": "a"},
		key: "a",
	}, {
		m:   map[string]any{"a": "1"},
		key: "a",
	}, {
		m:   nil,
		key: "a",
	}} {
		got, ok := intInMap(test.m, test.key)
		if got != test.want {
			t.Errorf("intInMap(%v, %q): got %v, want %v", test.m, test.key, got, test.want)
		}
		if ok != test.ok {
			t.Errorf("intInMap(%v, %q): got ok %v, want %v", test.m, test.key, ok, test.ok)
		}
	}
}

func TestStringInMapAnyKey(t *testing.T) {
	for _, test := range []struct {
		m    map[string]any
		keys []string
		want string
		ok   bool
	}{{
		m:    map[string]any{"a": "b"},
		keys: []string{"a"},
		want: "b",
		ok:   true,
	}, {
		m:    map[string]any{"b": "a"},
		keys: []string{"a"},
	}, {
		m:    map[string]any{"a": 1},
		keys: []string{"a"},
	}, {
		m:    nil,
		keys: []string{"a"},
	}, {
		m:    map[string]any{"a": "v"},
		keys: []string{"a", "b"},
		want: "v",
		ok:   true,
	}, {
		m:    map[string]any{"a": 1, "b": "v"},
		keys: []string{"a", "b"},
		want: "v",
		ok:   true,
	}, {
		m:    map[string]any{"c": "v"},
		keys: []string{"a", "b"},
	}} {
		got, found := stringInMapAnyKey(test.m, test.keys...)
		if got != test.want {
			t.Errorf(
				"stringInMapAnyKey(%v, %q): got %q, want %q",
				test.m, test.keys, got, test.want,
			)
		}
		if found != test.ok {
			t.Errorf(
				"stringInMapAnyKey(%v, %q): got found %v, want %v",
				test.m, test.keys, found, test.ok,
			)
		}
	}
}

func TestCloneMap(t *testing.T) {
	for _, m := range []map[string]any{
		nil,
		{},
		{"a": "b"},
		{"a": 1, "c": "d"},
	} {
		got := cloneMap(m)
		if !reflect.DeepEqual(got, m) {
			t.Errorf("cloneMap(%v): got %v", m, got)
		}
	}

	// Test that it is actually a cloned copy.
	m := map[string]any{"a": "b"}
	got := cloneMap(m)
	got["a"] = "c"
	if v := m["a"]; v != "b" {
		t.Errorf(
			"cloneMap(%v): value changed to %v after mutation in clone",
			m, got,
		)
	}
}

func TestCloneMapExcept(t *testing.T) {
	for _, test := range []struct {
		m      map[string]any
		except []string
		want   map[string]any
	}{{
		m: nil, except: nil, want: nil,
	}, {
		m: map[string]any{"a": "b"}, except: nil,
		want: map[string]any{"a": "b"},
	}, {
		m: map[string]any{"a": "b"}, except: []string{"a"},
		want: nil,
	}, {
		m: map[string]any{"a": "b", "c": "d"}, except: []string{"a"},
		want: map[string]any{"c": "d"},
	}, {
		m: map[string]any{"a": "b", "c": "d"}, except: []string{"a", "c"},
		want: nil,
	}, {
		m: map[string]any{"a": "b", "c": "d"}, except: []string{"b"},
		want: map[string]any{"a": "b", "c": "d"},
	}} {
		got := cloneMapExcept(test.m, test.except)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("cloneMapExcept(%v, %v): got %v, want %v",
				test.m, test.except, got, test.want,
			)
		}
	}

	// Test that it is actually a cloned copy.
	m := map[string]any{"a": "b"}
	got := cloneMapExcept(m, nil)
	got["a"] = "c"
	if v := m["a"]; v != "b" {
		t.Errorf(
			"cloneMap(%v): value changed to %v after mutation in clone",
			m, got,
		)
	}
}

func TestCheckStepKeys(t *testing.T) {
	for _, test := range []struct {
		step    map[string]any
		allowed []string
		ok      bool
	}{{
		step: map[string]any{"a": "b"}, allowed: []string{"a"}, ok: true,
	}, {
		step: map[string]any{"a": "b"}, allowed: []string{"b"}, ok: false,
	}, {
		step: map[string]any{"a": "b"}, allowed: []string{"a", "b"}, ok: true,
	}, {
		step: map[string]any{"a": "b"}, allowed: []string{"a", "c"}, ok: true,
	}, {
		step: map[string]any{"a": "b", "d": "c"}, allowed: []string{"a", "c"},
		ok: false,
	}, {
		step: map[string]any{}, allowed: []string{"a"}, ok: true,
	}, {
		step: map[string]any{"a": "b"}, allowed: nil, ok: false,
	}} {
		got := checkStepKeys(test.step, test.allowed)
		if test.ok != (got == nil) {
			t.Errorf("checkStepKeys(%v, %v): got %v, want %v",
				test.step, test.allowed, got, test.ok,
			)
		}
	}
}

func TestToStringList(t *testing.T) {
	for _, test := range []struct {
		in   any
		want []string
	}{
		{nil, nil},
		{"hello", []string{"hello"}},
		{[]string{"hello", "world"}, []string{"hello", "world"}},
		{[]any{"hello", "world"}, []string{"hello", "world"}},
		{[]any{"hello", 42}, []string{"hello"}},
	} {
		got := toStringList(test.in)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("fieldToStringList(%v): got %v, want %v",
				test.in, got, test.want,
			)
		}
	}
}

func TestCopyEnvMap(t *testing.T) {
	m := map[string]string{
		"A": "b",
		"C": "d",
	}

	cp := copyEnvMap(m)
	if !reflect.DeepEqual(cp, m) {
		t.Errorf("copyEnvMap(%v): got %v", m, cp)
	}
}

func TestStringSet(t *testing.T) {
	for _, test := range []struct {
		slice []string
		want  map[string]bool
	}{
		{slice: nil, want: nil},
		{slice: []string{"a"}, want: map[string]bool{"a": true}},
		{slice: []string{"a", "b"}, want: map[string]bool{"a": true, "b": true}},
	} {
		got := stringSet(test.slice...)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("stringSet(%v): got %v, want %v",
				test.slice, got, test.want,
			)
		}
	}
}
