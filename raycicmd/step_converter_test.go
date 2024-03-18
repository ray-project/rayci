package raycicmd

import (
	"testing"

	"reflect"
)

func TestWaitConverter(t *testing.T) {
	for _, test := range []struct {
		step map[string]any
		want map[string]any
	}{{
		step: map[string]any{"wait": nil},
		want: map[string]any{"wait": nil},
	}, {
		step: map[string]any{"wait": "true", "depends_on": "a"},
		want: map[string]any{"wait": "true", "depends_on": "a"},
	}, {
		step: map[string]any{
			"wait": "true", "if": "true", "depends_on": []string{"dep"},
			"continue_on_failure": "true",
			"tags":                []string{"tag"},
		},
		want: map[string]any{
			"wait": "true", "if": "true", "depends_on": []string{"dep"},
			"continue_on_failure": "true",
		},
	}} {
		match := waitConverter.match(test.step)
		if !match {
			t.Errorf("cannot match wait step %+v", test.step)
		}

		got, err := waitConverter.convert(test.step)
		if err != nil {
			t.Errorf("convert %+v got error %v", test.step, err)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"convert %+v, got %+v, want %+v",
				test.step, got, test.want,
			)
		}
	}
}

func TestBlockConverter(t *testing.T) {
	for _, test := range []struct {
		step map[string]any
		want map[string]any
	}{{
		step: map[string]any{"block": "click me"},
		want: map[string]any{"block": "click me"},
	}, {
		step: map[string]any{"block": "true", "depends_on": "a"},
		want: map[string]any{"block": "true", "depends_on": "a"},
	}, {
		step: map[string]any{
			"block": "me", "if": "false", "depends_on": []string{"dep"},
			"tags": []string{"tag"},
		},
		want: map[string]any{
			"block": "me", "if": "false", "depends_on": []string{"dep"},
		},
	}} {
		match := blockConverter.match(test.step)
		if !match {
			t.Errorf("cannot match wait step %+v", test.step)
		}

		got, err := blockConverter.convert(test.step)
		if err != nil {
			t.Errorf("convert %+v got error %v", test.step, err)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"convert %+v, got %+v, want %+v",
				test.step, got, test.want,
			)
		}
	}
}

func TestTriggerConverter(t *testing.T) {
	for _, test := range []struct {
		step map[string]any
		want map[string]any
	}{{
		step: map[string]any{"trigger": "me"},
		want: map[string]any{"trigger": "me"},
	}, {
		step: map[string]any{"trigger": "me", "depends_on": "a"},
		want: map[string]any{"trigger": "me", "depends_on": "a"},
	}, {
		step: map[string]any{
			"trigger": "me", "build": map[string]string{"branch": "master"},
			"tags": []string{"tag"},
		},
		want: map[string]any{
			"trigger": "me", "build": map[string]string{"branch": "master"},
		},
	}, {
		step: map[string]any{
			"trigger":                  "me",
			"key":                      "my_key",
			"depends_on":               "a",
			"if":                       1 < 2,
			"soft_fail":                "true",
			"allow_dependency_failure": "true",
			"tags":                     []string{"tag"},
		},
		want: map[string]any{
			"trigger":                  "me",
			"key":                      "my_key",
			"depends_on":               "a",
			"if":                       1 < 2,
			"soft_fail":                "true",
			"allow_dependency_failure": "true",
		},
	}} {
		got, err := triggerConverter.convert(test.step)
		if err != nil {
			t.Errorf("convert %+v got error %v", test.step, err)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf(
				"convert %+v, got %+v, want %+v",
				test.step, got, test.want,
			)
		}
	}
}

func TestIsBlockOrWait(t *testing.T) {
	for _, test := range []map[string]any{
		{"wait": "true"},
		{"block": "true"},
		{"block": "me", "if": "false", "depends_on": []string{"dep"}},
		{"wait": nil, "tags": []string{"tag"}},
	} {
		if !isBlockOrWait(test) {
			t.Errorf("%+v should be treated as a block or wait step", test)
		}
	}

	for _, test := range []map[string]any{
		{},
		{"command": "echo hello"},
		{"commands": []string{"echo hello"}},
	} {
		if isBlockOrWait(test) {
			t.Errorf("%+v should not be treated as a block or wait step", test)
		}
	}
}
