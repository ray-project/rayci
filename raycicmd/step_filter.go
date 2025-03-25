package raycicmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type stepFilter struct {
	// first pass: skip tags
	skipTags map[string]bool

	// second pass: tag selection filters
	runAll bool // Run all the steps; do not filter by tags.
	tags   map[string]bool

	// third pass: selecting steps
	selects    map[string]bool // based on ID or key
	tagSelects map[string]bool // or based on tags
}

func (f *stepFilter) reject(step *stepNode) bool {
	return step.hasTagInMap(f.skipTags)
}

func (f *stepFilter) accept(step *stepNode) bool {
	return f.acceptSelectHit(step) && f.acceptTagHit(step)
}

func (f *stepFilter) acceptSelectHit(step *stepNode) bool {
	if f.selects == nil && f.tagSelects == nil {
		// no select filters, accept everything.
		return true
	}

	return step.selectHit(f.selects) || step.hasTagInMap(f.tagSelects)
}

func (f *stepFilter) acceptTagHit(step *stepNode) bool {
	if f.runAll {
		return true
	}

	// if not in run-all mode, hit when the step has any of the tags.
	if !step.hasTags() {
		return true // step does not have any tags: a step that always runs
	}
	return step.hasTagInMap(f.tags)
}

func (f *stepFilter) hit(step *stepNode) bool {
	return !f.reject(step) && f.accept(step)
}

func newStepFilter(
	skipTags []string, selects []string, filterCmd []string,
) (*stepFilter, error) {
	filter, err := stepFilterFromCmd(skipTags, filterCmd)
	if err != nil {
		return filter, err
	}

	if selects != nil {
		filter.selects = make(map[string]bool)
		filter.tagSelects = make(map[string]bool)

		for _, k := range selects {
			if strings.HasPrefix(k, "tag:") {
				name := strings.TrimPrefix(k, "tag:")
				filter.tagSelects[name] = true
			} else {
				filter.selects[k] = true
			}
		}
	}

	return filter, err
}

func stepFilterFromCmd(skips []string, filterCmd []string) (
	*stepFilter, error,
) {
	filter := &stepFilter{skipTags: stringSet(skips...), runAll: true}

	if len(filterCmd) == 0 {
		return filter, nil
	}

	bin := filterCmd[0]
	if strings.HasPrefix(bin, "./") {
		// A local in repo launcher, and the file does not exist yet.
		// Run all tags in this case.
		if _, err := os.Lstat(bin); os.IsNotExist(err) {
			return filter, nil
		}
	}

	// TODO: put the execution in an unprivileged sandbox
	cmd := exec.Command(filterCmd[0], filterCmd[1:]...)
	cmd.Stderr = os.Stderr
	filters, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tag filter script: %w", err)
	}

	filtersStr := strings.TrimSpace(string(filters))
	if filtersStr == "*" {
		// '*" means run everything (except the skips).
		// It is equivalent to having no tag filters configured.
		return filter, nil
	}

	tags := strings.Fields(filtersStr)
	filter.runAll = false
	if len(tags) != 0 {
		filter.tags = stringSet(tags...)
	}

	return filter, nil
}
