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

	runAll bool
	tags   []string
}

func (f *stepFilter) reject(step *stepNode) bool {
	return step.hasTagIn(f.skipTags)
}

func (f *stepFilter) accept(step *stepNode) bool {
	if f.selects != nil && f.tags != nil {
		// in both key selection mode, hit when the step has both keys and tags
		return f.acceptSelectHit(step) && f.acceptTagHit(step)
	}

	if f.selects != nil {
		// in only key selection mode, hit when the step has keys
		return f.acceptSelectHit(step)
	}

	// in tags filtering mode
	return f.acceptTagHit(step)
}

func (f *stepFilter) acceptSelectHit(step *stepNode) bool {
	if f.selects != nil {
		return step.selectHit(f.selects)
	}

	return true
}

func (f *stepFilter) acceptTagHit(step *stepNode) bool {
	if f.runAll {
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

func newSelectAndTagsStepFilter(
	skipTags []string, selects []string, filterCmd []string,
) (*stepFilter, error) {
	select_filter := newSelectStepFilter(skipTags, selects)
	tag_filter, err := newTagsStepFilter(skipTags, filterCmd)

	return &stepFilter{
		skipTags: skipTags,
		selects:  select_filter.selects,
		runAll:   tag_filter.runAll,
		tags:     tag_filter.tags,
	}, err
}

func newSelectStepFilter(skipTags []string, selects []string) *stepFilter {
	filter := &stepFilter{skipTags: skipTags, selects: make(map[string]bool)}
	for _, k := range selects {
		filter.selects[k] = true
	}
	return filter
}

func newTagsStepFilter(skips []string, filterCmd []string) (
	*stepFilter, error,
) {
	filter := &stepFilter{skipTags: skips, runAll: true}

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
	filter.runAll = false
	filter.tags = tags

	return filter, nil
}
