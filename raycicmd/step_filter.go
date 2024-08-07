package raycicmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type stepFilter struct {
	skipTags []string

	// selecting steps based on ID or key
	selects map[string]bool

	runAllTags bool
	tags       []string
}

func (f *stepFilter) reject(step *stepNode) bool {
	return step.hasTagIn(f.skipTags)
}

func (f *stepFilter) accept(step *stepNode) bool {
	return f.acceptSelectHit(step) && f.acceptTagHit(step)
}

func (f *stepFilter) acceptSelectHit(step *stepNode) bool {
	if f.selects != nil {
		return step.selectHit(f.selects)
	}

	return true
}

func (f *stepFilter) acceptTagHit(step *stepNode) bool {
	if f.runAllTags {
		return true
	}

	// if not in run-all mode, hit when the step has any of the tags.
	if !step.hasTags() {
		return true // step does not have any tags: a step that always runs
	}
	return step.hasTagIn(f.tags)
}

func (f *stepFilter) hit(step *stepNode) bool {
	return !f.reject(step) && f.accept(step)
}

func newStepFilter(
	skipTags []string, selects []string, filterCmd []string,
) (*stepFilter, error) {
	filter, err := stepFilterFromCmd(skipTags, filterCmd)
	if selects != nil && err == nil {
		filter.selects = make(map[string]bool)
		for _, k := range selects {
			filter.selects[k] = true
		}
	}

	return filter, err
}

func stepFilterFromCmd(skips []string, filterCmd []string) (
	*stepFilter, error,
) {
	filter := &stepFilter{skipTags: skips, runAllTags: true}

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
	if len(tags) == 0 {
		tags = nil
	}
	filter.runAllTags = false
	filter.tags = tags

	return filter, nil
}
