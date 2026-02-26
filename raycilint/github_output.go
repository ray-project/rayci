package raycilint

import (
	"fmt"
	"os"
	"strings"
)

// writeGitHubOutput writes key=value pairs to the given path so
// subsequent workflow steps can reference them.
func writeGitHubOutput(path string, cfg *prsizeConfig, stats *diffStats) {
	var sb strings.Builder
	if cfg.MaxAdditions > 0 && stats.linesAdded > cfg.MaxAdditions {
		fmt.Fprintf(
			&sb, "additions=%d (max allowed: %d)\n",
			stats.linesAdded, cfg.MaxAdditions,
		)
	}
	if cfg.MaxDeletions > 0 && stats.linesDeleted > cfg.MaxDeletions {
		fmt.Fprintf(
			&sb, "deletions=%d (max allowed: %d)\n",
			stats.linesDeleted, cfg.MaxDeletions,
		)
	}
	appendGitHubFile(path, sb.String())
}

// writeGitHubStepSummary appends a markdown summary to the given path
// when running inside GitHub Actions.
func writeGitHubStepSummary(path string, cfg *prsizeConfig, stats *diffStats) {
	var sb strings.Builder
	sb.WriteString("### PR Size Warning\n\n")
	if cfg.MaxAdditions > 0 {
		fmt.Fprintf(
			&sb, "- additions: max %d, changed %d\n",
			cfg.MaxAdditions, stats.linesAdded,
		)
	}
	if cfg.MaxDeletions > 0 {
		fmt.Fprintf(
			&sb, "- deletions: max %d, changed %d\n",
			cfg.MaxDeletions, stats.linesDeleted,
		)
	}
	appendGitHubFile(path, sb.String())
}

func appendGitHubFile(path, content string) {
	if path == "" || content == "" {
		return
	}
	f, err := os.OpenFile(
		path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "raycilint: could not open %s: %v\n", path, err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "raycilint: could not close %s: %v\n", path, err)
		}
	}()
	if _, err := f.WriteString(content); err != nil {
		fmt.Fprintf(os.Stderr, "raycilint: could not write to %s: %v\n", path, err)
	}
}
