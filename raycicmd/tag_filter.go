package raycicmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type tagFilter struct {
	tags   []string
	runAll bool
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

func (f *tagFilter) hit(tags []string) bool {
	if f.runAll {
		return true
	}
	return intersects(f.tags, tags)
}

var runAllTags = &tagFilter{runAll: true}

func runTagFilterCommand(tagFilterCommand []string) (*tagFilter, error) {
	if len(tagFilterCommand) == 0 {
		return runAllTags, nil
	}

	bin := tagFilterCommand[0]
	if strings.HasPrefix(bin, "./") {
		// A local in repo launcher, and the file does not exist yet.
		// Run all tags in this case.
		if _, err := os.Lstat(bin); os.IsNotExist(err) {
			return runAllTags, nil
		}
	}

	// TODO: put the execution in an unprivileged sandbox
	cmd := exec.Command(tagFilterCommand[0], tagFilterCommand[1:]...)
	filters, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tag filter script: %w", err)
	}

	tags := strings.Fields(string(filters))
	if len(tags) == 0 {
		tags = nil
	}
	return &tagFilter{tags: tags}, nil
}
