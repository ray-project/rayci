package raycicmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

const labelRulesFileSuffix = ".labels.yaml"

// labelRule adjusts step selection when a pull request carries a label.
type labelRule struct {
	Label string `yaml:"label"`

	// KeepOnlyTags narrows selection when the label is present: a step is
	// selected only if it would be selected anyway and it also carries one
	// of these tags. Steps outside the set are not selected, but they are not
	// rejected either, so steps that depend on them are unaffected.
	KeepOnlyTags []string `yaml:"keep_only_tags"`
}

func parseLabelRulesFile(file string) ([]*labelRule, error) {
	bs, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read label rules file: %w", err)
	}

	var rules []*labelRule
	dec := yaml.NewDecoder(bytes.NewReader(bs))
	dec.KnownFields(true)
	if err := dec.Decode(&rules); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil // empty or comment-only file: no rules
		}
		return nil, fmt.Errorf("unmarshal label rules file %s: %w", file, err)
	}

	for i, r := range rules {
		if r == nil {
			return nil, fmt.Errorf("label rule %d in %s: rule is null", i, file)
		}
		if r.Label == "" {
			return nil, fmt.Errorf("label rule %d in %s: label is empty", i, file)
		}
		if len(r.KeepOnlyTags) == 0 {
			return nil, fmt.Errorf(
				"label rule %q in %s: no tags to keep", r.Label, file,
			)
		}
	}
	return rules, nil
}

func loadLabelRules(files []string) ([]*labelRule, error) {
	var rules []*labelRule
	for _, file := range files {
		rs, err := parseLabelRulesFile(file)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rs...)
	}
	return rules, nil
}

// prLabels returns the pull request's labels. Buildkite sets
// BUILDKITE_PULL_REQUEST_LABELS to a comma-separated list, or to an empty
// string for a build that is not a pull request.
func prLabels(envs Envs) map[string]bool {
	labels := make(map[string]bool)
	for _, l := range strings.Split(getEnv(envs, "BUILDKITE_PULL_REQUEST_LABELS"), ",") {
		if l = strings.TrimSpace(l); l != "" {
			labels[l] = true
		}
	}
	return labels
}

// applyLabelRules adjusts the filter by the rules whose label is present.
// When more than one keep-only rule applies, a step needs a tag from any of
// them.
func applyLabelRules(filter *stepFilter, rules []*labelRule, labels map[string]bool) {
	for _, r := range rules {
		if !labels[r.Label] {
			continue
		}
		if filter.keepOnlyTags == nil {
			filter.keepOnlyTags = make(map[string]bool)
		}
		for _, t := range r.KeepOnlyTags {
			filter.keepOnlyTags[t] = true
		}
		log.Printf(
			"label %q: keep only tags: %s", r.Label, strings.Join(r.KeepOnlyTags, " "),
		)
	}
}
