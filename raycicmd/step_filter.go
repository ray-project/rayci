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

	noTagMeansAlways bool
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

	if f.noTagMeansAlways {
		// if noTagMeansAlways is set, hit when the step has any of the tags.
		if !step.hasTags() {
			return true // step does not have any tags: a step that always runs
		}
	}
	return step.hasTagInMap(f.tags)
}

func (f *stepFilter) hit(step *stepNode) bool {
	return !f.reject(step) && f.accept(step)
}

func newStepFilter(
	skipTags, selects []string, filterCmd []string,
) (*stepFilter, error) {
	filterCmdRes, err := runFilterCmd(filterCmd)
	if err != nil {
		return nil, err
	}

	filter := &stepFilter{
		skipTags: stringSet(skipTags...),
	}
	if !filterCmdRes.cmdExists || filterCmdRes.runAll {
		filter.runAll = true
	} else {
		filter.tags = filterCmdRes.tags
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

type filterCmdResult struct {
	cmdExists bool
	runAll    bool
	tags      map[string]bool
}

func runFilterCmd(cmd []string) (*filterCmdResult, error) {
	res := &filterCmdResult{}
	if len(cmd) == 0 {
		return res, nil
	}

	bin := cmd[0]
	if strings.HasPrefix(bin, "./") {
		// A local in repo launcher, and the file does not exist yet.
		// Run all tags in this case.
		if _, err := os.Lstat(bin); os.IsNotExist(err) {
			return res, nil
		}
	}

	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stderr = os.Stderr
	output, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("tag filter script: %w", err)
	}

	res.cmdExists = true

	tags := strings.Fields(string(output))
	if len(tags) == 1 && tags[0] == "*" {
		// '*" means run everything (except the skips).
		// It is often equivalent to having no tag filters configured.
		res.runAll = true
		return res, nil
	}

	res.tags = stringSet(tags...)
	return res, nil
}
