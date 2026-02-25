package raycilint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteGitHubStepSummary(t *testing.T) {
	summaryFile := filepath.Join(t.TempDir(), "summary.md")

	cfg := &prsizeConfig{MaxAdditions: 100, MaxDeletions: 200}
	stats := &diffStats{linesAdded: 150, linesDeleted: 250}
	writeGitHubStepSummary(summaryFile, cfg, stats)

	bs, err := os.ReadFile(summaryFile)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	got := string(bs)
	want := strings.Join([]string{
		"### PR Size Warning",
		"",
		"- additions: max 100, changed 150",
		"- deletions: max 200, changed 250",
		"",
	}, "\n")
	if got != want {
		t.Errorf("writeGitHubStepSummary() =\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteGitHubStepSummary_EmptyPath(t *testing.T) {
	cfg := &prsizeConfig{MaxAdditions: 100}
	stats := &diffStats{linesAdded: 150}
	writeGitHubStepSummary("", cfg, stats)
}

func TestWriteGitHubOutput(t *testing.T) {
	outputFile := filepath.Join(t.TempDir(), "output.txt")

	cfg := &prsizeConfig{MaxAdditions: 300, MaxDeletions: 500}
	stats := &diffStats{linesAdded: 450, linesDeleted: 120}
	writeGitHubOutput(outputFile, cfg, stats)

	bs, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(bs)
	want := "additions=450 (max allowed: 300)\n"
	if got != want {
		t.Errorf("writeGitHubOutput() = %q, want %q", got, want)
	}
}
