package raycicmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type stepFilter struct {
	skipTags []string

	runAll bool
	tags   []string
}

func (f *stepFilter) hit(node *stepNode) bool {
	if !node.hasTags() {
		return true
	}
	if node.hasTagIn(f.skipTags) {
		return false
	}
	if f.runAll {
		return true
	}
	return node.hasTagIn(f.tags)
}

func newStepFilter(skips []string, filterCmd []string) (*stepFilter, error) {
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
