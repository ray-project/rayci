package prcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_MissingFlags(t *testing.T) {
	code, err := Main([]string{"prcheck"})
	if err != nil {
		t.Fatalf("Main() error: %v", err)
	}
	if code != 1 {
		t.Errorf("Main() = %d, want 1", code)
	}
}

func TestMain_InvalidFlag(t *testing.T) {
	code, err := Main([]string{"prcheck", "-bogus"})
	if err == nil {
		t.Fatal("Main() error = nil, want error for invalid flag")
	}
	if code != 1 {
		t.Errorf("Main() = %d, want 1", code)
	}
}

func TestMain_NoConfig(t *testing.T) {
	code, err := Main([]string{
		"prcheck",
		"-config", filepath.Join(t.TempDir(), "nope.yaml"),
		"-base-ref", "main",
		"-head-ref", "feature",
	})
	if err == nil {
		t.Fatal("Main() error = nil, want error for missing config")
	}
	if code != 1 {
		t.Errorf("Main() = %d, want 1", code)
	}
}

func TestWriteJobSummary(t *testing.T) {
	summaryFile := filepath.Join(t.TempDir(), "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryFile)

	cfg := &sizeConfig{MaxAdditions: 100, MaxDeletions: 200}
	stats := &diffStats{linesAdded: 150, linesDeleted: 250}
	writeJobSummary(cfg, stats)

	bs, err := os.ReadFile(summaryFile)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	got := string(bs)
	if !strings.Contains(got, "max 100") {
		t.Errorf("summary missing additions threshold: %s", got)
	}
	if !strings.Contains(got, "changed 150") {
		t.Errorf("summary missing additions count: %s", got)
	}
	if !strings.Contains(got, "max 200") {
		t.Errorf("summary missing deletions threshold: %s", got)
	}
	if !strings.Contains(got, "changed 250") {
		t.Errorf("summary missing deletions count: %s", got)
	}
}

func TestWriteJobSummary_NoEnv(t *testing.T) {
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	cfg := &sizeConfig{MaxAdditions: 100}
	stats := &diffStats{linesAdded: 150}
	writeJobSummary(cfg, stats)
}

func TestWriteGitHubOutput(t *testing.T) {
	outputFile := filepath.Join(t.TempDir(), "output.txt")
	t.Setenv("GITHUB_OUTPUT", outputFile)

	cfg := &sizeConfig{MaxAdditions: 300, MaxDeletions: 500}
	stats := &diffStats{linesAdded: 450, linesDeleted: 120}
	writeGitHubOutput(cfg, stats)

	bs, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(bs)
	if !strings.Contains(got, "additions=450 (max allowed: 300)") {
		t.Errorf("output missing exceeded additions: %s", got)
	}
	if strings.Contains(got, "deletions=") {
		t.Errorf("output should omit deletions under threshold: %s", got)
	}
}

func TestRun_NoConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

	g := &gitClient{}
	code, err := run(configPath, "main", "feature", g)
	if err == nil {
		t.Fatal("run() error = nil, want error for missing config")
	}
	if code != 1 {
		t.Errorf("run() = %d, want 1", code)
	}
}

func TestRun_NoThresholds(t *testing.T) {
	content := strings.Join([]string{
		"size:",
		"  max_additions: 0",
		"  max_deletions: 0",
	}, "\n") + "\n"

	configPath := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	g := &gitClient{}
	code, err := run(configPath, "master", "feature", g)
	if err != nil {
		t.Fatalf("run() error: %v", err)
	}
	if code != 0 {
		t.Errorf("run() = %d, want 0", code)
	}
}

func TestRun_ExceedsThreshold(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial commit")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "// line")
	}
	h.writeFile("big.go", strings.Join(lines, "\n")+"\n")
	h.commitAll("add big file")
	h.git("push", "origin", "feature")

	configPath := h.writeConfig(100, 0, nil)

	g := &gitClient{workDir: h.WorkDir}
	code, err := run(configPath, "master", "feature", g)
	if err != nil {
		t.Fatalf("run() error: %v", err)
	}
	if code != 1 {
		t.Errorf("run() = %d, want 1", code)
	}
}

func TestRun_UnderThreshold(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial commit")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	h.writeFile("small.go", "package main\n")
	h.commitAll("add small file")
	h.git("push", "origin", "feature")

	configPath := h.writeConfig(10000, 10000, nil)

	g := &gitClient{workDir: h.WorkDir}
	code, err := run(configPath, "master", "feature", g)
	if err != nil {
		t.Fatalf("run() error: %v", err)
	}
	if code != 0 {
		t.Errorf("run() = %d, want 0", code)
	}
}

func TestRun_IgnoredFiles(t *testing.T) {
	h := newGitTestHelper(t)
	h.writeFile("README.md", "# README\n")
	h.commitAll("initial commit")
	h.git("push", "origin", "master")

	h.git("checkout", "-b", "feature")
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "// line")
	}
	h.writeFile("vendor/big.go", strings.Join(lines, "\n")+"\n")
	h.writeFile("src/small.go", "package main\n")
	h.commitAll("add files")
	h.git("push", "origin", "feature")

	configPath := h.writeConfig(100, 0, []string{"vendor/"})

	g := &gitClient{workDir: h.WorkDir}
	code, err := run(configPath, "master", "feature", g)
	if err != nil {
		t.Fatalf("run() error: %v", err)
	}
	if code != 0 {
		t.Errorf("run() = %d, want 0 (vendor/ should be ignored)", code)
	}
}
