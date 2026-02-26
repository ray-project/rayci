package raycilint

import (
	"fmt"
	"strconv"
	"strings"
)

type diffStats struct {
	linesAdded   int
	linesDeleted int
}

// parseDiffNumstat parses "git diff --numstat" output into aggregate
// line counts, skipping binary files and ignored prefixes.
func parseDiffNumstat(output []byte, ignore []string) (*diffStats, error) {
	stats := new(diffStats)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		if parts[0] == "-" || parts[1] == "-" {
			continue
		}

		filename := parts[2]
		skip := false
		for _, prefix := range ignore {
			if strings.HasPrefix(filename, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		added, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("parse additions for %s: %w", filename, err)
		}
		deleted, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parse deletions for %s: %w", filename, err)
		}

		stats.linesAdded += added
		stats.linesDeleted += deleted
	}
	return stats, nil
}

// checkSize returns warning messages for each threshold exceeded.
// A zero threshold means that dimension is not checked.
func checkSize(cfg *prsizeConfig, stats *diffStats) []string {
	var exceeded []string
	if cfg.MaxAdditions > 0 && stats.linesAdded > cfg.MaxAdditions {
		exceeded = append(exceeded, fmt.Sprintf(
			"WARNING: additions (%d) exceed threshold (%d)",
			stats.linesAdded, cfg.MaxAdditions,
		))
	}
	if cfg.MaxDeletions > 0 && stats.linesDeleted > cfg.MaxDeletions {
		exceeded = append(exceeded, fmt.Sprintf(
			"WARNING: deletions (%d) exceed threshold (%d)",
			stats.linesDeleted, cfg.MaxDeletions,
		))
	}
	return exceeded
}
