package raycicmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type jobFilter struct {
	skipTags []string

	runAll bool
	tags   []string

	selects map[string]bool
}

func intersects(set1, set2 []string) bool {
	set := make(map[string]struct{})
	for _, s := range set1 {
		set[s] = struct{}{}
	}
	for _, s := range set2 {
		if _, hit := set[s]; hit {
			return true
		}
	}
	return false
}

func (f *jobFilter) hit(selects, tags []string) bool {
	if f.hitSelects(selects) {
		return true
	}
	return f.hitTags(tags)
}

func (f *jobFilter) hitSelects(selects []string) bool {
	if f.selects != nil && len(selects) > 0 {
		for _, k := range selects {
			if f.selects[k] {
				return true
			}
		}
	}
	return false
}

func (f *jobFilter) hitTags(tags []string) bool {
	if len(tags) == 0 {
		return true
	}
	if intersects(f.skipTags, tags) {
		return false
	}
	if f.runAll {
		return true
	}
	return intersects(f.tags, tags)
}

func (f *jobFilter) addSelects(selects []string) {
	if f.selects == nil {
		f.selects = make(map[string]bool)
	}
	for _, s := range selects {
		f.selects[s] = true
	}
}

func newTagFilter(skips []string, filterCmd []string) (*jobFilter, error) {
	filter := &jobFilter{skipTags: skips, runAll: true}

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
