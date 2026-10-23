package raycicmd

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func writeLabelRules(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "test"+labelRulesFileSuffix)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write label rules: %v", err)
	}
	return p
}

func TestParseLabelRulesFile(t *testing.T) {
	p := writeLabelRules(t, t.TempDir(), strings.Join([]string{
		"- label: docstring-only",
		"  keep_only_tags: [always, lint]",
	}, "\n"))

	got, err := parseLabelRulesFile(p)
	if err != nil {
		t.Fatalf("parseLabelRulesFile: %v", err)
	}
	want := []*labelRule{{Label: "docstring-only", KeepOnlyTags: []string{"always", "lint"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseLabelRulesFile() = %+v, want %+v", got, want)
	}
}

func TestParseLabelRulesFile_errors(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
	}{{
		name:    "empty label",
		content: "- keep_only_tags: [always]",
	}, {
		name:    "no tags",
		content: "- label: docstring-only",
	}, {
		name:    "unknown field",
		content: "- label: docstring-only\n  keep_tags: [always]",
	}, {
		name:    "null element",
		content: "- label: docstring-only\n  keep_only_tags: [always]\n- null",
	}} {
		t.Run(test.name, func(t *testing.T) {
			p := writeLabelRules(t, t.TempDir(), test.content)
			if _, err := parseLabelRulesFile(p); err == nil {
				t.Errorf("parseLabelRulesFile(%q) = nil error, want error", test.content)
			}
		})
	}
}

func TestParseLabelRulesFile_noRules(t *testing.T) {
	for _, content := range []string{"", "\n", "# no rules yet\n"} {
		p := writeLabelRules(t, t.TempDir(), content)
		got, err := parseLabelRulesFile(p)
		if err != nil {
			t.Fatalf("parseLabelRulesFile(%q): %v", content, err)
		}
		if got != nil {
			t.Errorf("parseLabelRulesFile(%q) = %v, want nil", content, got)
		}
	}
}

func TestPRLabels(t *testing.T) {
	for _, test := range []struct {
		value string
		want  map[string]bool
	}{
		{value: "", want: map[string]bool{}},
		{value: "go", want: map[string]bool{"go": true}},
		{value: "go,docstring-only", want: map[string]bool{"go": true, "docstring-only": true}},
		{value: " go , ,docs ", want: map[string]bool{"go": true, "docs": true}},
	} {
		envs := newEnvsMap(map[string]string{"BUILDKITE_PULL_REQUEST_LABELS": test.value})
		if got := prLabels(envs); !reflect.DeepEqual(got, test.want) {
			t.Errorf("prLabels(%q) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestApplyLabelRules(t *testing.T) {
	rules := []*labelRule{
		{Label: "docstring-only", KeepOnlyTags: []string{"always", "lint"}},
		{Label: "lint-only", KeepOnlyTags: []string{"lint", "pre_commit"}},
	}

	for _, test := range []struct {
		name   string
		labels map[string]bool
		want   map[string]bool
	}{{
		name:   "label absent",
		labels: map[string]bool{"go": true},
		want:   nil,
	}, {
		name:   "one label",
		labels: map[string]bool{"docstring-only": true},
		want:   stringSet("always", "lint"),
	}, {
		name:   "two labels keep the union",
		labels: map[string]bool{"docstring-only": true, "lint-only": true},
		want:   stringSet("always", "lint", "pre_commit"),
	}} {
		t.Run(test.name, func(t *testing.T) {
			filter := &stepFilter{}
			applyLabelRules(filter, rules, test.labels)
			if !reflect.DeepEqual(filter.keepOnlyTags, test.want) {
				t.Errorf("keepOnlyTags = %v, want %v", filter.keepOnlyTags, test.want)
			}
		})
	}
}

func TestStepFilter_keepOnly(t *testing.T) {
	for _, test := range []struct {
		name   string
		filter *stepFilter
		hits   [][]string
		misses [][]string
	}{{
		name: "with selected tags",
		filter: &stepFilter{
			skipTags:     stringSet("disabled"),
			tags:         stringSet("data", "always"),
			keepOnlyTags: stringSet("always", "docstring"),
		},
		hits:   [][]string{{"always"}, {"data", "docstring"}},
		misses: [][]string{{"data"}, {"core"}, {"core", "docstring"}, {"always", "disabled"}},
	}, {
		name: "with run all",
		filter: &stepFilter{
			runAll:       true,
			keepOnlyTags: stringSet("always"),
		},
		hits:   [][]string{{"always"}, {"data", "always"}},
		misses: [][]string{nil, {"data"}},
	}, {
		name: "untagged steps are not kept",
		filter: &stepFilter{
			tags:             stringSet("data"),
			keepOnlyTags:     stringSet("always"),
			noTagMeansAlways: true,
		},
		misses: [][]string{nil, {"data"}},
	}} {
		t.Run(test.name, func(t *testing.T) {
			for _, tags := range test.hits {
				if !test.filter.hit(&stepNode{tags: tags}) {
					t.Errorf("miss %+v", tags)
				}
			}
			for _, tags := range test.misses {
				if test.filter.hit(&stepNode{tags: tags}) {
					t.Errorf("hit %+v", tags)
				}
			}
		})
	}
}

func emittedKeys(groups []*bkPipelineGroup) []string {
	var keys []string
	for _, g := range groups {
		for _, s := range g.Steps {
			if k, ok := stringInMap(s.(map[string]any), "key"); ok {
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func TestConvertPipelineGroups_keepOnly(t *testing.T) {
	c := newConverter(&config{
		ArtifactsBucket: "artifacts_bucket",
		CITemp:          "s3://ci-temp/",
		RunnerQueues:    map[string]string{"default": "runner"},
	}, &buildInfo{buildID: "abc123"})

	groups := []*pipelineGroup{{
		Group: "images",
		Steps: []map[string]any{
			{"name": "shared-image", "wanda": "shared.yaml"},
			{"name": "data-image", "wanda": "data.yaml"},
			{"name": "dep-image", "wanda": "dep.yaml"},
		},
	}, {
		Group: "data",
		Tags:  []string{"data"},
		Steps: []map[string]any{{
			"key":        "data-tests",
			"commands":   []string{"test data"},
			"tags":       []any{"data"},
			"depends_on": []any{"shared-image", "data-image"},
		}},
	}, {
		Group: "docs",
		Steps: []map[string]any{{
			"key":        "helper",
			"commands":   []string{"helper"},
			"tags":       []any{"data"},
			"depends_on": "dep-image",
		}, {
			"key":        "examples",
			"commands":   []string{"run examples"},
			"tags":       []any{"data", "docstring"},
			"depends_on": []any{"shared-image", "helper"},
		}},
	}}

	filter := &stepFilter{
		tags:         stringSet("data"),
		keepOnlyTags: stringSet("docstring"),
	}
	bk, err := c.convertGroups(groups, filter)
	if err != nil {
		t.Fatalf("convertGroups: %v", err)
	}

	// The data group's own tag is selected but not kept, so it doesn't
	// accept its steps. data-image feeds only the dropped step, so it drops
	// too. helper isn't kept, but a kept step depends on it, so it runs
	// along with its image.
	want := []string{"dep-image", "examples", "helper", "shared-image"}
	if got := emittedKeys(bk); !reflect.DeepEqual(got, want) {
		t.Errorf("emitted keys = %v, want %v", got, want)
	}
}

func TestMakePipeline_labelRules(t *testing.T) {
	tmp := t.TempDir()
	bkDir := filepath.Join(tmp, ".buildkite")
	if err := os.MkdirAll(bkDir, 0o700); err != nil {
		t.Fatalf("mkdir .buildkite: %v", err)
	}

	pipeline := strings.Join([]string{
		"group: g",
		"steps:",
		"  - label: tests",
		"    key: tests",
		"    tags: data",
		"    commands: [ 'echo tests' ]",
		"  - label: examples",
		"    key: examples",
		"    tags: [ data, docstring ]",
		"    commands: [ 'echo examples' ]",
	}, "\n")
	if err := os.WriteFile(
		filepath.Join(bkDir, "test.rayci.yaml"), []byte(pipeline), 0o600,
	); err != nil {
		t.Fatalf("write pipeline: %v", err)
	}
	rules := strings.Join([]string{"! data", "src/", "@ data", ";"}, "\n")
	if err := os.WriteFile(filepath.Join(bkDir, "test.rules.txt"), []byte(rules), 0o600); err != nil {
		t.Fatalf("write rules: %v", err)
	}
	writeLabelRules(t, bkDir, strings.Join([]string{
		"- label: docstring-only",
		"  keep_only_tags: [ docstring ]",
	}, "\n"))

	lister, envs := setupGitRepoInDir(t, tmp, []string{"src/file.py"})

	for _, test := range []struct {
		labels string
		want   []string
	}{
		{labels: "", want: []string{"examples", "tests"}},
		{labels: "go,docstring-only", want: []string{"examples"}},
	} {
		envs.m["BUILDKITE_PULL_REQUEST_LABELS"] = test.labels
		got, err := makePipeline(&pipelineContext{
			repoDir:      tmp,
			changeLister: lister,
			config: &config{
				ArtifactsBucket: "artifacts",
				CITemp:          "s3://ci-temp",
				CIWorkRepo:      "fakeecr",
				RunnerQueues:    map[string]string{"default": "runner_x"},
			},
			info: &buildInfo{buildID: "test"},
			envs: envs,
		})
		if err != nil {
			t.Fatalf("makePipeline: %v", err)
		}
		if keys := emittedKeys(got.Steps); !reflect.DeepEqual(keys, test.want) {
			t.Errorf("labels %q: emitted keys = %v, want %v", test.labels, keys, test.want)
		}
	}
}

func TestParseLabelRulesFile_skipForms(t *testing.T) {
	p := writeLabelRules(t, t.TempDir(), strings.Join([]string{
		"- label: docs-example-test",
		"  skip_tags_unless_present: [doc_example_optin]",
		"- label: no-gpu",
		"  skip_tags_when_present: [gpu]",
	}, "\n"))

	got, err := parseLabelRulesFile(p)
	if err != nil {
		t.Fatalf("parseLabelRulesFile: %v", err)
	}
	want := []*labelRule{
		{Label: "docs-example-test", SkipTagsUnlessPresent: []string{"doc_example_optin"}},
		{Label: "no-gpu", SkipTagsWhenPresent: []string{"gpu"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseLabelRulesFile() = %+v, want %+v", got, want)
	}
}

func TestConvertPipelineGroups_skipForms(t *testing.T) {
	groups := func() []*pipelineGroup {
		return []*pipelineGroup{{
			Group: "images",
			Steps: []map[string]any{
				{"name": "shared-image", "wanda": "shared.yaml"},
				{"name": "optin-image", "wanda": "optin.yaml"},
			},
		}, {
			Group: "tests",
			Steps: []map[string]any{{
				"key":        "unit",
				"commands":   []string{"unit"},
				"tags":       []any{"doc"},
				"depends_on": "shared-image",
			}, {
				"key":        "optin",
				"commands":   []string{"optin"},
				"tags":       []any{"doc", "optin"},
				"depends_on": []any{"shared-image", "optin-image"},
			}, {
				"key":        "after-optin",
				"commands":   []string{"after"},
				"tags":       []any{"doc"},
				"depends_on": "optin",
			}},
		}}
	}
	all := []string{"after-optin", "optin", "optin-image", "shared-image", "unit"}
	// Skipping optin drops the image only it needs, keeps the shared image,
	// and rejects the step that depends on it, as a configured skip tag would.
	skipped := []string{"shared-image", "unit"}

	for _, test := range []struct {
		name   string
		rule   *labelRule
		labels map[string]bool
		want   []string
	}{{
		name:   "unless present, label absent",
		rule:   &labelRule{Label: "run-optin", SkipTagsUnlessPresent: []string{"optin"}},
		labels: map[string]bool{"go": true},
		want:   skipped,
	}, {
		name:   "unless present, label present",
		rule:   &labelRule{Label: "run-optin", SkipTagsUnlessPresent: []string{"optin"}},
		labels: map[string]bool{"run-optin": true},
		want:   all,
	}, {
		name:   "when present, label present",
		rule:   &labelRule{Label: "no-optin", SkipTagsWhenPresent: []string{"optin"}},
		labels: map[string]bool{"no-optin": true},
		want:   skipped,
	}, {
		name:   "when present, label absent",
		rule:   &labelRule{Label: "no-optin", SkipTagsWhenPresent: []string{"optin"}},
		labels: map[string]bool{},
		want:   all,
	}} {
		t.Run(test.name, func(t *testing.T) {
			c := newConverter(&config{
				ArtifactsBucket: "artifacts_bucket",
				CITemp:          "s3://ci-temp/",
				RunnerQueues:    map[string]string{"default": "runner"},
			}, &buildInfo{buildID: "abc123"})

			filter := &stepFilter{tags: stringSet("doc")}
			applyLabelRules(filter, []*labelRule{test.rule}, test.labels)

			bk, err := c.convertGroups(groups(), filter)
			if err != nil {
				t.Fatalf("convertGroups: %v", err)
			}
			if got := emittedKeys(bk); !reflect.DeepEqual(got, test.want) {
				t.Errorf("emitted keys = %v, want %v", got, test.want)
			}
		})
	}
}
