package raycicmd

import (
	"fmt"
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
	skipTags, selects []string, filterConfig []string, envs Envs, lister FileLister,
) (*stepFilter, error) {
	filterConfigRes, err := runFilterConfig(filterConfig, envs, lister)
	if err != nil {
		return nil, fmt.Errorf("run filter config: %w", err)
	}

	filter := &stepFilter{
		skipTags: stringSet(skipTags...),
	}
	if len(filterConfig) == 0 || filterConfigRes.runAll {
		filter.runAll = true
	} else {
		filter.tags = filterConfigRes.tags
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

func runFilterConfig(filterConfig []string, envs Envs, lister FileLister) (*filterConfigResult, error) {
	res := &filterConfigResult{}

	fmt.Println("filterConfig", filterConfig)
	if len(filterConfig) == 0 {
		res.runAll = true
		return res, nil
	}

	tags, err := RunTagAnalysis(
		filterConfig,
		envs,
		lister,
	)
	if err != nil {
		return nil, err
	}

	if len(tags) == 1 && tags[0] == "*" {
		// '*" means run everything (except the skips).
		// It is often equivalent to having no tag filters configured.
		res.runAll = true
	} else {
		res.tags = stringSet(tags...)
	}

	return res, nil
}

type filterConfigResult struct {
	runAll bool
	tags   map[string]bool
}
